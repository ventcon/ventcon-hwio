// main is the main package of this project
package main

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/ventcon/ventcon-hwio/nats"
	"github.com/ventcon/ventcon-hwio/scheduling"
	"github.com/ventcon/ventcon-hwio/serial"
	"github.com/ventcon/ventcon-hwio/ventilator"
	msgspec "github.com/ventcon/ventcon-msgspec"
)

// main is the main entrypoint of this project
func main() {
	setupLogging()

	config, vars, err := loadMainConfig()

	if err != nil {
		log.WithError(err).WithField("variables", vars).Fatal("Failed to initialize config.")
	}

	configureLogging(config)

	log.WithField("variables", vars).Info("This software is configured using environment variables.")
	log.WithField("config", config).Info("Sucesfully loaded the configuration.")

	natsClient := nats.NewNatsClient(config.Nats)
	if err := natsClient.Connect(); err != nil {
		log.WithError(err).Fatal("Failed to connect to NATS broker")
	}
	defer func() {
		if err := natsClient.Close(); err != nil {
			log.WithError(err).Fatal("Failed to close NATS connection")
		}
	}()

	serialManager, requestChannel, err := serial.NewSerialManager(config.PortName)
	if err != nil {
		log.WithError(err).Fatal("Failed to create serial manager")
	}
	err = serialManager.Start()
	defer func() {
		if err := serialManager.Stop(); err != nil {
			log.WithError(err).Fatal("Failed to stop serial manager")
		}
	}()
	if err != nil {
		log.WithError(err).Fatal("Failed to start serial manager")
	}

	scheduler := scheduling.NewFairScheduler(requestChannel)

	vent := ventilator.Ventilator{
		Address:            config.Address,
		HasFeatureHumidity: true,
		HasFeatureVOC:      false,
	}

	ventilatorManager, ventilatorRequestChannel := ventilator.NewVentilatorManager(
		vent,
		config.Period,
		func(state *msgspec.VentilatorState) {
			if err := natsClient.SendVentilatorState(state); err != nil {
				log.WithError(err).Error("Failed to send ventilator state")
			}
		},
	)

	err = scheduler.AddSource(ventilatorRequestChannel)
	if err != nil {
		log.WithError(err).Fatal("Failed to add source to the scheduler")
	}

	cmdlistener := nats.NewCommandListener(natsClient, config.Address, ventilatorManager.RunCommand)

	scheduler.Start()
	defer scheduler.Stop()

	ventilatorManager.Start()
	defer ventilatorManager.Stop()

	err = cmdlistener.Start()
	if err != nil {
		log.WithError(err).Fatal("Failed to start command listener")
	}
	defer cmdlistener.Stop()

	log.WithField("portName", config.PortName).Info("Opened Serial")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c // wait for signal
	log.Info("Received signal, shutting down")
	// Shutdown of parts handled by defer keyword
}
