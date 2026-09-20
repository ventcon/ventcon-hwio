package ventilator

import (
	"errors"
	"time"

	"github.com/ansel1/merry/v2"
	log "github.com/sirupsen/logrus"
	"github.com/ventcon/ventcon-hwio/encoding"
	"github.com/ventcon/ventcon-hwio/serial"
	msgSpec "github.com/ventcon/ventcon-msgspec"
)

const (
	MAX_PERIOD = 10 * time.Minute
)

var VentilatorIsOffline = merry.Sentinel("Ventilator is offline")

type VentilatorManager interface {
	Start()
	Stop()
	RunCommand(command msgSpec.Command) error
	markAsValidVentilatorManager()
}

type ventilatorManager struct {
	ventilator           Ventilator
	statusUpdateCallback func(state *msgSpec.VentilatorState)
	state                *msgSpec.VentilatorState
	configuredPeriod     time.Duration
	currentPeriod        time.Duration
	ticker               *time.Ticker
	hasIntiallyRead      bool
	requestChannel       chan<- serial.Request
	stop                 chan (bool)
}

func NewVentilatorManager(
	ventilator Ventilator,
	period time.Duration,
	statusUpdateCallback func(state *msgSpec.VentilatorState),
) (VentilatorManager, <-chan serial.Request) {
	requestChannel := make(chan serial.Request)
	return &ventilatorManager{
		ventilator:           ventilator,
		statusUpdateCallback: statusUpdateCallback,
		state: &msgSpec.VentilatorState{
			Address: ventilator.Address,
			Online:  false,
			Data:    msgSpec.VentilatorData{},
		},
		configuredPeriod: period,
		currentPeriod:    period,
		ticker:           time.NewTicker(period),
		hasIntiallyRead:  false,
		requestChannel:   requestChannel,
		stop:             make(chan (bool)),
	}, requestChannel
}

func (ventilatorManager *ventilatorManager) logger() *log.Entry {
	return log.WithField("ventilator", ventilatorManager.ventilator.Address)
}

func (ventilatorManager *ventilatorManager) Start() {
	ventilatorManager.logger().Debug("Starting ventilator manager for ", ventilatorManager.ventilator.Address)

	go func() {
		ventilatorManager.ticker.Reset(ventilatorManager.currentPeriod)
		defer ventilatorManager.ticker.Stop()
		defer close(ventilatorManager.requestChannel)

		for {
			select {
			case <-ventilatorManager.ticker.C:
				ventilatorManager.ReadDataAndUpdateState(true)
			case <-ventilatorManager.stop:
				return
			}
		}
	}()
}

func (ventilatorManager *ventilatorManager) ReadDataAndUpdateState(scheduled bool) {
	var functions []Function
	if ventilatorManager.hasIntiallyRead {
		functions = AllFunctions[2:]
	} else {
		functions = AllFunctions[:]
	}
	err := ventilatorManager.ReadData(functions)

	if err != nil {
		if errors.Is(err, serial.NoDataOnSerialError) {
			ventilatorManager.logger().Debug("No data on serial, asuming ventilator is offline and reducing polling frequency")
			ventilatorManager.MarkAsOffline(scheduled)
		} else {
			ventilatorManager.logger().WithError(err).Error("Error while reading data")
		}
	} else {
		ventilatorManager.hasIntiallyRead = true
	}

	ventilatorManager.sendStatusUpdate()
}

func (ventilatorManager *ventilatorManager) MarkAsOffline(scheduled bool) {
	ventilatorManager.state.Online = false
	// If this is triggered due to a command to not extend the exponential backoff unless this is the first notice of offline
	if scheduled || ventilatorManager.currentPeriod == ventilatorManager.configuredPeriod {
		if ventilatorManager.currentPeriod*2 < MAX_PERIOD {
			ventilatorManager.currentPeriod *= 2
		}
		ventilatorManager.ticker.Reset(ventilatorManager.currentPeriod)
	}
}

func (ventilatorManager *ventilatorManager) MarkAsOnline() {
	if ventilatorManager.state.Online {
		return
	}

	ventilatorManager.hasIntiallyRead = false // Re-Read all data

	ventilatorManager.state.Online = true

	if ventilatorManager.currentPeriod != ventilatorManager.configuredPeriod {
		ventilatorManager.currentPeriod = ventilatorManager.configuredPeriod
		ventilatorManager.ticker.Reset(ventilatorManager.currentPeriod)
	}
}

func (ventilatorManager *ventilatorManager) ReadData(functions []Function) error {
	for _, fun := range functions {
		if (fun == ExhaustAirHumidity || fun == InletAirHumidity) && !ventilatorManager.ventilator.HasFeatureHumidity {
			continue
		}
		if (fun == VocConcentration) && !ventilatorManager.ventilator.HasFeatureVOC {
			continue
		}

		if err := ventilatorManager.sendReadRequest(fun); err != nil {
			return err
		}
	}

	return nil
}

func (ventilatorManager *ventilatorManager) sendReadRequest(fun Function) error {
	respChan := make(chan serial.Response)
	frame, err := encoding.NewReadRequest(ventilatorManager.ventilator.Address, fun)
	if err != nil {
		return merry.Prepend(err, "Failed to create read request")
	}
	req := serial.Request{
		ResponseChannel: respChan,
		Data:            frame,
	}
	ventilatorManager.requestChannel <- req
	resp := <-respChan
	if resp.Err != nil {
		return merry.Prepend(resp.Err, "Read request failed")
	}
	if resp.Response.Address() != ventilatorManager.ventilator.Address {
		return merry.New("Response address does not match ventilator address")
	}
	if resp.Response.FrameType() != encoding.ReadResponse {
		return merry.New("Response frame type is not a read response")
	}
	ventilatorManager.logger().Trace("Received read response: ", resp.Response)
	ventilatorManager.MarkAsOnline()
	err = updateVentilatorData(&ventilatorManager.state.Data, resp.Response.Function(), resp.Response.Value())
	return merry.Prepend(err, "Failed to update ventilator data")
}

func (ventilatorManager *ventilatorManager) sendWriteRequest(fun Function, value int) error {
	respChan := make(chan serial.Response)
	frame, err := encoding.NewWriteRequest(ventilatorManager.ventilator.Address, fun, value)
	if err != nil {
		return merry.Prepend(err, "Failed to create write request")
	}
	req := serial.Request{
		ResponseChannel: respChan,
		Data:            frame,
	}
	ventilatorManager.requestChannel <- req
	resp := <-respChan
	if resp.Err != nil {
		return merry.Prepend(resp.Err, "Write request failed")
	}
	if resp.Response.Address() != ventilatorManager.ventilator.Address {
		return merry.New("Response address does not match ventilator address")
	}
	if resp.Response.FrameType() != encoding.WriteResponse {
		return merry.New("Response frame type is not a write response")
	}
	ventilatorManager.logger().Trace("Received write response: ", resp.Response)
	ventilatorManager.MarkAsOnline()
	err = updateVentilatorData(&ventilatorManager.state.Data, resp.Response.Function(), resp.Response.Value())
	return merry.Prepend(err, "Failed to update ventilator data")
}

func (ventilatorManager *ventilatorManager) SendWriteRequestAndUpdateState(fun Function, value int) error {
	if !ventilatorManager.state.Online {
		return VentilatorIsOffline
	}

	err := ventilatorManager.sendWriteRequest(fun, value)

	if err != nil {
		if errors.Is(err, serial.NoDataOnSerialError) {
			ventilatorManager.logger().Debug("No data on serial (while running command), asuming ventilator is offline.")
			ventilatorManager.MarkAsOffline(false)
			err = merry.Apply(err, merry.WithCause(VentilatorIsOffline))
		}
	} else {
		ventilatorManager.sendStatusUpdate()
	}
	return err
}

func (ventilatorManager *ventilatorManager) sendStatusUpdate() {
	ventilatorManager.logger().WithField("state", ventilatorManager.state).Debug("Sending status update")
	ventilatorManager.statusUpdateCallback(ventilatorManager.state)
}

func (ventilatorManager *ventilatorManager) Stop() {
	ventilatorManager.logger().Debug("Stopping ventilator manager for ", ventilatorManager.ventilator.Address)
	close(ventilatorManager.stop)
}

func (ventilatorManager *ventilatorManager) RunCommand(command msgSpec.Command) error {
	ventilatorManager.logger().
		WithField("command", command).
		Debug("Running command")

	switch t := command.(type) {
	case *msgSpec.PollVentilatorNowCommand:
		ventilatorManager.ReadDataAndUpdateState(false)
	case *msgSpec.SetRemoteCommanderCommand:
		value := calculateNewCommanderAndVentilationMode(t.RemoteCommander, ventilatorManager.state.Data.VentilationMode)
		return ventilatorManager.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, value)
	case *msgSpec.SetVentilationModeCommand:
		value := calculateNewCommanderAndVentilationMode(ventilatorManager.state.Data.RemoteCommander, t.VentilationMode)
		return ventilatorManager.SendWriteRequestAndUpdateState(CommanderAndVentilationMode, value)
	case *msgSpec.SetIntakeAirLevelCommand:
		value := calculateNewIntakeAirlevelAndExhaustAirlevel(t.IntakeAirLevel, ventilatorManager.state.Data.RequestedExhaustAirLevel)
		return ventilatorManager.SendWriteRequestAndUpdateState(RequestedIntakeAndExhaustAirlevels, value)
	case *msgSpec.SetExhaustAirLevelCommand:
		value := calculateNewIntakeAirlevelAndExhaustAirlevel(ventilatorManager.state.Data.RequestedIntakeAirLevel, t.ExhaustAirLevel)
		return ventilatorManager.SendWriteRequestAndUpdateState(RequestedIntakeAndExhaustAirlevels, value)
	case *msgSpec.SetBothAirLevelCommand:
		value := calculateNewIntakeAirlevelAndExhaustAirlevel(t.IntakeAirLevel, t.ExhaustAirLevel)
		return ventilatorManager.SendWriteRequestAndUpdateState(RequestedIntakeAndExhaustAirlevels, value)
	default:
		return merry.Errorf("Unknown command: %T", command)
	}
	return nil
}

func (ventilatorManager *ventilatorManager) markAsValidVentilatorManager() {}
