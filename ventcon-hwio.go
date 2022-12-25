// main is the main package of this project
package main

import (
	log "github.com/sirupsen/logrus"
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
}
