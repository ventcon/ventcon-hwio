package main

import (
	"github.com/neumantm/logtrace"
	"github.com/sirupsen/logrus"
)

// setupLogging initialized the logging for this project.
// It should be called as early as possible.
func setupLogging() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.AddHook(logtrace.DefaultLogtraceHook())
	logrus.SetReportCaller(true)
}

// configureLogging configures the logging for this project using the given config.
// It should be called as early as possible but after loading the configuration.
// It should be called after setupLogging.
func configureLogging(config Config) {
	logrus.SetLevel(config.LogLevel)
}
