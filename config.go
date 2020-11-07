package main

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Environment is environment specific configuration
type Environment struct {
	ClusterName string `yaml:"cluster_name"`
	Pipeline    []Step `yaml:"pipeline"`
}

// Config represents all options that can be configured by a flecs config file
type Config struct {
	ClusterName  string                 `yaml:"cluster_name"`
	Definitions  []Definition           `yaml:"definitions"`
	Environments map[string]Environment `yaml:"environments"`
	Pipeline     []Step                 `yaml:"pipeline"`
}

// Definition will be used to configure task definitions
type Definition struct{}

// Step describes a step in the pipeline
type Step struct {
	Script  Script  `yaml:"script"`
	Service Service `yaml:"service"`
	Task    Task    `yaml:"task"`
	Type    string
}

// LoadConfig will load all configuration options if they exist, allowing
// environment specific options to override top level options
func LoadConfig() (config Config, err error) {
	configPath := viper.GetViper().ConfigFileUsed()
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	yaml.Unmarshal(file, &config)

	// Configure default options
	if viper.IsSet("environment") {
		config.ClusterName = config.Environments[viper.GetString("environment")].ClusterName
	}

	envConfig, err := config.getEnvConfig()
	if err != nil {
		return config, err
	}

	// Set ClusterName
	if config.ClusterName == "" && envConfig.ClusterName == "" {
		config.ClusterName = "default"
	}

	if envConfig.ClusterName != "" {
		config.ClusterName = envConfig.ClusterName
	}

	// Check and set Pipeline
	if len(config.Pipeline) < 1 && len(envConfig.Pipeline) < 1 {
		return config, fmt.Errorf("pipeline configuration not found")
	}

	if len(envConfig.Pipeline) > 0 {
		config.Pipeline = envConfig.Pipeline
	}

	// Check Pipeline for syntax errors
	for index, step := range config.Pipeline {
		serviceSet := step.Service != (Service{})
		taskSet := step.Task != (Task{})
		scriptSet := step.Script != (Script{})

		if !serviceSet && !taskSet && !scriptSet {
			return config, fmt.Errorf("invalid step config on step %d", index)
		}

		if serviceSet && taskSet || taskSet && scriptSet || serviceSet && scriptSet {
			return config, fmt.Errorf("must configure only one of: service, task, script")
		}

		// Here we set as a string what kind of step it is
		if serviceSet && !taskSet && !scriptSet {
			config.Pipeline[index].Type = "service"
		}

		if taskSet && !scriptSet && !serviceSet {
			config.Pipeline[index].Type = "task"
		}

		if scriptSet && !serviceSet && !taskSet {
			config.Pipeline[index].Type = "script"
		}
	}

	return config, err
}

func (c Config) getEnvConfig() (env Environment, err error) {
	if viper.GetString("environment") != "" {
		e := viper.GetString("environment")

		env, ok := c.Environments[e]
		if !ok {
			return env, fmt.Errorf("Cannot find environment config for %s", e)
		}
	}

	return env, err
}
