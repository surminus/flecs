package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Environment is environment specific configuration
type Environment struct {
	ClusterName        string   `yaml:"cluster_name"`
	Pipeline           []Step   `yaml:"pipeline"`
	Region             string   `yaml:"region"`
	SecurityGroupNames []string `yaml:"security_group_names"`
	SubnetNames        []string `yaml:"subnet_names"`
}

// Config represents all options that can be configured by a flecs config file
type Config struct {
	ClusterName        string                 `yaml:"cluster_name"`
	Definitions        map[string]Definition  `yaml:"definitions"`
	Environments       map[string]Environment `yaml:"environments"`
	Pipeline           []Step                 `yaml:"pipeline"`
	ProjectName        string                 `yaml:"project_name"`
	Region             string                 `yaml:"region"`
	Services           map[string]Service     `yaml:"services"`
	SecurityGroupNames []string               `yaml:"security_group_names"`
	SubnetNames        []string               `yaml:"subnet_names"`

	// Set automatically
	EnvironmentName string
}

// Step describes a step in the pipeline
type Step struct {
	Script  ScriptStep  `yaml:"script"`
	Service ServiceStep `yaml:"service"`
	Task    TaskStep    `yaml:"task"`
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

	// Set default project name
	if config.ProjectName == "" {
		wd, err := os.Getwd()
		if err != nil {
			return config, err
		}

		config.ProjectName = path.Base(wd)
	}

	// Set ClusterName
	if config.ClusterName == "" && envConfig.ClusterName == "" {
		config.ClusterName = "default"
	}

	if envConfig.ClusterName != "" {
		config.ClusterName = envConfig.ClusterName
	}

	// Check and set region
	if config.Region == "" && envConfig.Region == "" {
		config.Region = "eu-west-1"
	}

	if envConfig.Region != "" {
		config.Region = envConfig.Region
	}

	// Check and set security group names
	if len(envConfig.SecurityGroupNames) > 0 {
		config.SecurityGroupNames = envConfig.SecurityGroupNames
	}

	// Check and set subnet names
	if len(envConfig.SubnetNames) > 0 {
		config.SubnetNames = envConfig.SubnetNames
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
		serviceSet := step.Service != (ServiceStep{})
		taskSet := step.Task != (TaskStep{})
		scriptSet := step.Script != (ScriptStep{})

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
