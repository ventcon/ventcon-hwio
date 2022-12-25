package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/shoenig/test"
	log "github.com/sirupsen/logrus"
)

func TestSanitizeEnvVarName(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{"FOOBAR1", "FOOBAR1"},
		{"fooBAR1", "FOOBAR1"},
		{"foo_bar1", "FOO_BAR1"},
		{"foo-bar1", "FOO_BAR1"},
		{"FOOBAR?", "FOOBAR_"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(`sanitizeEnvVarName(%q)`, tc.input), func(t *testing.T) {
			test.EqOp(t, tc.expectedOutput, sanitizeEnvVarName(tc.input))
		})
	}
}

func setEnvVar(configOption string, value string) {
	var envVar = sanitizeEnvVarName(PREFIX + "_" + configOption)
	err := os.Setenv(envVar, value)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

type TestConfig struct {
	ConfigOptionFoo int           `required:"true" desc:"Some foo"`
	ConfigOptionBar string        `default:"stringA" split_words:"true" desc:"Some bar"`
	LogLevel        log.Level     `default:"Info" split_words:"true" desc:"The log level (panic, fatal, error, warn, info, debug, trace)"`
	SubConfig       TestSubConfig `split_words:"true"`
}

type TestSubConfig struct {
	ConfigOptionFooBar bool `split_words:"true" desc:"Whether to foobar"`
}

func TestLoadConfigSetAll(t *testing.T) {
	os.Clearenv()
	setEnvVar("ConfigOptionFoo", "5")
	setEnvVar("Config_Option_Bar", "foooooo")
	setEnvVar("Log_Level", "warn")
	setEnvVar("Sub_Config_Config_Option_foo_bar", "true")

	var config TestConfig
	err := loadConfig(&config)

	test.NoError(t, err)

	test.EqOp(t, 5, config.ConfigOptionFoo)
	test.EqOp(t, "foooooo", config.ConfigOptionBar)
	test.EqOp(t, log.WarnLevel, config.LogLevel)
	test.True(t, config.SubConfig.ConfigOptionFooBar)
}

func TestLoadConfigDefaults(t *testing.T) {
	os.Clearenv()
	setEnvVar("ConfigOptionFoo", "-5")

	var config TestConfig
	err := loadConfig(&config)

	test.NoError(t, err)

	test.EqOp(t, -5, config.ConfigOptionFoo)
	test.EqOp(t, "stringA", config.ConfigOptionBar)
	test.EqOp(t, log.InfoLevel, config.LogLevel)
	test.False(t, config.SubConfig.ConfigOptionFooBar)
}

func TestLoadConfigMissing(t *testing.T) {
	os.Clearenv()

	var config TestConfig
	err := loadConfig(&config)

	test.Error(t, err)
	test.EqOp(t, "required key VENTCON_HWIO_CONFIGOPTIONFOO missing value", err.Error())
}

func TestLoadConfigAdditional(t *testing.T) {
	os.Clearenv()
	setEnvVar("ConfigOptionFoo", "-5")
	setEnvVar("SomeOtherOption", "-5")

	var config TestConfig
	err := loadConfig(&config)

	test.Error(t, err)
	test.EqOp(t, "unknown environment variable VENTCON_HWIO_SOMEOTHEROPTION", err.Error())
}

func TestGetUsage(t *testing.T) {
	var config TestConfig
	actualVars, err := getUsage(&config)

	test.NoError(t, err)

	expectedVars := []Variable{
		{
			Name:        "VENTCON_HWIO_CONFIGOPTIONFOO",
			Type:        "Integer",
			Default:     "",
			Required:    true,
			Description: "Some foo",
		},
		{
			Name:        "VENTCON_HWIO_CONFIG_OPTION_BAR",
			Type:        "String",
			Default:     "stringA",
			Required:    false,
			Description: "Some bar",
		},
		{
			Name:        "VENTCON_HWIO_LOG_LEVEL",
			Type:        "Level",
			Default:     "Info",
			Required:    false,
			Description: "The log level (panic, fatal, error, warn, info, debug, trace)",
		},
		{
			Name:        "VENTCON_HWIO_SUB_CONFIG_CONFIG_OPTION_FOO_BAR",
			Type:        "True or False",
			Default:     "",
			Required:    false,
			Description: "Whether to foobar",
		},
	}

	for _, actualVar := range actualVars {
		test.SliceContains(t, expectedVars, actualVar)
	}

	for _, expectedVar := range expectedVars {
		test.SliceContains(t, actualVars, expectedVar)
	}
}
