package nats

import (
	"encoding/json"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/nats-io/nats.go/jetstream"
	log "github.com/sirupsen/logrus"
	"github.com/ventcon/ventcon-hwio/ventilator"
	msgspec "github.com/ventcon/ventcon-msgspec"
)

type CommandListener interface {
	Start() error
	Stop()
	markAsValidCommandListener()
}

type commandListener struct {
	ventilatorNumber int
	nc               NatsClient
	msgCtx           jetstream.MessagesContext
	callback         func(msgspec.Command) error
}

func NewCommandListener(nc NatsClient, ventilatorNumber int, callback func(msgspec.Command) error) CommandListener {
	return &commandListener{
		ventilatorNumber: ventilatorNumber,
		nc:               nc,
		callback:         callback,
	}
}

func (cl *commandListener) Start() error {
	msgCtx, err := cl.nc.SubscribeToVentilatorCommands(cl.ventilatorNumber)
	if err != nil {
		return merry.Prependf(err, "Failed to subscribe to ventilator commands for ventilator %d", cl.ventilatorNumber)
	}

	cl.msgCtx = msgCtx

	go cl.Run()

	return nil
}

func Nak(msg jetstream.Msg, duration time.Duration) {
	err := msg.NakWithDelay(duration)
	if err != nil {
		log.WithError(err).Error("Failed to NAK message")
	}
}

func Terminate(msg jetstream.Msg, reason string) {
	err := msg.TermWithReason(reason)
	if err != nil {
		log.WithError(err).Error("Failed to terminate message")
	}
}

func (cl *commandListener) Run() {
	for {
		logWVN := log.WithField("ventilator", cl.ventilatorNumber)

		msg, err := cl.msgCtx.Next()
		if err == jetstream.ErrMsgIteratorClosed {
			break
		}
		if err != nil {
			logWVN.WithError(err).Error("Failed to receive message from NATS")
			continue
		}

		cmdMsg := msgspec.EmptyCommandMessage()
		err = json.Unmarshal(msg.Data(), cmdMsg)
		if err != nil {
			logWVN.WithError(merry.Wrap(err)).Error("Failed to unmarshal command message")
			Terminate(msg, "Failed to unmarshal command message")
			continue
		}

		if cmdMsg.Address() != cl.ventilatorNumber {
			logWVN.WithField("receivedVentilatorNumber", cmdMsg.Address()).Error("Received command for wrong ventilator. Terminating message")
			Terminate(msg, "Wrong ventilator number")
			continue
		}

		err = cl.callback(cmdMsg.Command())
		if err == ventilator.VentilatorIsOffline {
			logWVN.Infof("Ventilator %d is offline, delaying message", cl.ventilatorNumber)
			Nak(msg, 5*time.Second)
			continue
		}
		if err != nil {
			logWVN.WithField("command", cmdMsg.Command()).WithError(err).Error("Failed to process command message")
			Nak(msg, 1*time.Second)
			continue
		}
		err = msg.Ack()
		if err != nil {
			logWVN.WithField("command", cmdMsg.Command()).WithError(err).Warn("Failed to acknowledge command message")
		}
	}
}

func (cl *commandListener) Stop() {
	cl.msgCtx.Drain()
}

func (cl *commandListener) markAsValidCommandListener() {
	// intentionally left blank
}
