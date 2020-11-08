package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Environment is environment specific configuration
type Environment struct {
	ClusterName          string            `yaml:"cluster_name"`
	EnvironmentVariables map[string]string `yaml:"environment_variables"`
	LogGroupName         string            `yaml:"log_group_name"`
	Pipeline             []Step            `yaml:"pipeline"`
	Region               string            `yaml:"region"`
	Secrets              []string          `yaml:"secrets"`
	SecurityGroupNames   []string          `yaml:"security_group_names"`
	SubnetNames          []string          `yaml:"subnet_names"`
}

// Config represents all options that can be configured by a flecs config file
type Config struct {
	ClusterName          string                 `yaml:"cluster_name"`
	Definitions          map[string]Definition  `yaml:"definitions"`
	EnvironmentVariables map[string]string      `yaml:"environment_variables"`
	Environments         map[string]Environment `yaml:"environments"`
	LogGroupName         string                 `yaml:"log_group_name"`
	Pipeline             []Step                 `yaml:"pipeline"`
	ProjectName          string                 `yaml:"project_name"`
	Region               string                 `yaml:"region"`
	Secrets              []string               `yaml:"secrets"`
	SecurityGroupNames   []string               `yaml:"security_group_names"`
	Services             map[string]Service     `yaml:"services"`
	SubnetNames          []string               `yaml:"subnet_names"`

	EnvironmentName string
	Tag             string
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
	if viper.GetString("environment") != "" {
		config.EnvironmentName = viper.GetString("environment")

		if _, ok := config.Environments[config.EnvironmentName]; !ok {
			return config, fmt.Errorf("no environment configuration found")
		}

		config.ClusterName = config.Environments[config.EnvironmentName].ClusterName
	}

	if config.Tag == "" {
		r, err := git.PlainOpen(".")
		if err != nil {
			return config, err
		}

		ref, err := r.Head()
		if err != nil {
			return config, err
		}

		config.Tag = ref.Hash().String()
		Log.Infof("Using tag %s", config.Tag)
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

	// Merge environment variables
	if len(envConfig.EnvironmentVariables) > 0 {
		for key, value := range envConfig.EnvironmentVariables {
			config.EnvironmentVariables[key] = value
		}
	}

	// Merge secrets
	var mergedSecrets []string
	if len(envConfig.Secrets) > 0 && len(config.Secrets) == 0 {
		mergedSecrets = envConfig.Secrets
	}

	if len(config.Secrets) > 0 && len(envConfig.Secrets) == 0 {
		mergedSecrets = config.Secrets
	}

	if len(config.Secrets) > 0 && len(envConfig.Secrets) > 1 {
		// Turn the secrets into a map so it's easier to merge
		mappedSecrets := make(map[string]string)
		for _, secret := range config.Secrets {
			mappedSecrets[secret] = ""
		}

		// Replace any global secrets with environment specific ones
		for _, secret := range envConfig.Secrets {
			mappedSecrets[secret] = ""
		}

		// Convert back to slice
		for key, _ := range mappedSecrets {
			mergedSecrets = append(mergedSecrets, key)
		}
	}

	config.Secrets = mergedSecrets

	// Check and set LogGroupName
	if config.LogGroupName == "" && envConfig.LogGroupName != "" {
		config.LogGroupName = envConfig.LogGroupName
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
