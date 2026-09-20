package nats

type NatsConfig struct {
	BrokerURL string `default:"nats://localhost:4222" split_words:"true" desc:"The NATS broker URL"`
}
