package nats

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	msgspec "github.com/ventcon/ventcon-msgspec"
)

const durableName = "ventcon-hwio"

type NatsClient interface {
	markAsValidNatsClient()
	SendVentilatorState(state *msgspec.VentilatorState) error
	SubscribeToVentilatorCommands(ventialtorNumber int) (jetstream.MessagesContext, error)
	Connect() error
	Close() error
}

type natsClient struct {
	config         NatsConfig
	nc             *nats.Conn
	js             jetstream.JetStream
	subSetupCancel context.CancelFunc
}

func NewNatsClient(config NatsConfig) NatsClient {
	return &natsClient{
		config: config,
	}
}

func (nc *natsClient) Connect() error {
	client, err := nats.Connect(nc.config.BrokerURL)
	if err != nil {
		return merry.Prependf(err, "Failed to connect to NATS broker %s", nc.config.BrokerURL)
	}
	nc.nc = client

	js, err := jetstream.New(client)
	if err != nil {
		return merry.Prepend(err, "Failed to create NATS JetStream after connecting to broker")
	}
	nc.js = js
	return nil
}

func (nc *natsClient) Close() error {
	if nc.subSetupCancel != nil {
		nc.subSetupCancel()
	}
	err := nc.nc.Drain()
	if err != nil {
		return merry.Prepend(err, "Failed to close NATS connection")
	}
	nc.nc = nil
	return nil
}

func (nc *natsClient) SendVentilatorState(state *msgspec.VentilatorState) error {
	if nc.nc == nil {
		return merry.New("NATS client is not connected")
	}

	data, err := json.Marshal(state)
	if err != nil {
		return merry.Prepend(err, "Failed to marshal VentilatorState")
	}

	err = nc.nc.Publish(msgspec.HeartbeatTopic, data)
	if err != nil {
		return merry.Prepend(err, "Failed to publish VentilatorState")
	}
	return nil
}

func (nc *natsClient) SubscribeToVentilatorCommands(ventialtorNumber int) (jetstream.MessagesContext, error) {
	ctx, cancel := context.WithDeadlineCause(context.Background(), time.Now().Add((time.Second * 10)), merry.Errorf("Failed to setup nats subscriber in 10s"))
	nc.subSetupCancel = cancel

	subject := msgspec.CommandTopicPrefix + "." + strconv.Itoa(ventialtorNumber)

	stream, err := nc.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     strings.ReplaceAll(subject, ".", "_"),
		Subjects: []string{subject},
	})
	if err != nil {
		return nil, merry.Prependf(err, "Failed to create or update stream for subject %s", subject)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:    durableName,
		MaxDeliver: 10,
	})
	if err != nil {
		return nil, merry.Prependf(err, "Failed to create or update consumer for subject %s", subject)
	}

	msgContext, err := consumer.Messages()
	if err != nil {
		return nil, merry.Prependf(err, "Failed to get messages for consumer %s", durableName)
	}

	return msgContext, nil
}

func (nc *natsClient) markAsValidNatsClient() {
	// intentionally blank
}
