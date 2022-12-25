package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

// PREFIX is prepended the the configuration options of this project
// to obtain the names for the environment variables.
const PREFIX = "VENTCON_HWIO"

// Config represents the configuration options of this project.
//
// See github.com/kelseyhightower/envconfig for the format
// It can also include subconfig of specific components.
type Config struct {
	LogLevel log.Level `default:"Info" split_words:"true" desc:"The log level (panic, fatal, error, warn, info, debug, trace)"`
}

// LogLevel is a type alias used for the LogLevel config decoded
type LogLevel log.Level

// Decode is used to Decode LogLevel configurations
func (level *LogLevel) Decode(value string) error {
	logLevel, err := log.ParseLevel(value)
	*level = LogLevel(logLevel)
	return err
}

func sanitizeEnvVarName(envVarName string) string {
	var newEnvVarName string
	for _, char := range strings.ToUpper(envVarName) {
		var new_char rune
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			new_char = char
		} else {
			new_char = '_'
		}
		newEnvVarName += string(new_char)
	}
	return newEnvVarName
}

// loadConfig checks and loads the configuration for the given struct from the environment.
// Returns the loaded configuration and some information on possible configurations.
func loadConfig(config interface{}) error {
	if err := envconfig.Process(sanitizeEnvVarName(PREFIX), config); err != nil {
		return err
	}

	return envconfig.CheckDisallowed(sanitizeEnvVarName(PREFIX), config)
}

// loadMainConfig checks and loads the configuration as specified in Config.
// Returns the loaded configuration and some information on possible configurations.
func loadMainConfig() (Config, []Variable, error) {
	var config Config

	vars, err := getUsage(&config)
	if err != nil {
		return config, vars, err
	}

	if err := loadConfig(&config); err != nil {
		return config, vars, err
	}

	return config, vars, nil
}

/* usage related stuff */

// Variable describes one possible environment variable
type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// usageFormat is used by getUsage to print the usage info from envconfig as a json
const usageFormat = `{{ define "dec" }}{{ len (slice (printf "%*s" . "") 1) }}{{ end -}}
{{$length := len . -}}
[
{{range $idx, $val := .}}  {
    "name": "{{usage_key $val}}",
    "type": "{{usage_type $val}}",
    "default": "{{usage_default $val}}",
    "required": {{if usage_required $val -}} true {{- else -}} false {{- end}},
    "description": "{{usage_description $val}}"
  }{{if not (eq $idx (len (slice (printf "%*s" $length "") 1)))}},{{end}}{{/* If not last element print comma */}}
{{end}}]
`

// getUsage gets the usage information from envconfig, parses it and returns it as a array of Variables
func getUsage(config interface{}) ([]Variable, error) {
	var buff bytes.Buffer
	var vars []Variable

	if err := envconfig.Usagef(PREFIX, config, io.Writer(&buff), usageFormat); err != nil {
		return vars, err
	}

	if err := json.Unmarshal(buff.Bytes(), &vars); err != nil {
		return vars, err
	}

	return vars, nil
}
