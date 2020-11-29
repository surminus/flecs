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

// ConfigOptions contains all the configuration options that are configurable
// either at a per environment level or at the plain top level.
type ConfigOptions struct {
	AssignPublicIP       bool              `yaml:"public_ip"`
	ClusterName          string            `yaml:"cluster_name"`
	ECRRegion            string            `yaml:"ecr_region"`
	EnvironmentVariables map[string]string `yaml:"environment_variables"`
	LogGroupName         string            `yaml:"log_group_name"`
	Pipeline             []Step            `yaml:"pipeline"`
	Region               string            `yaml:"region"`
	Secrets              map[string]string `yaml:"secrets"`
	SecurityGroupNames   []string          `yaml:"security_group_names"`
	SubnetNames          []string          `yaml:"subnet_names"`
}

// Config represents all options that can be configured by a flecs config file
type Config struct {
	Options ConfigOptions

	Definitions  map[string]Definition    `yaml:"definitions"`
	Environments map[string]ConfigOptions `yaml:"environments"`
	Pipeline     []Step                   `yaml:"pipeline"`
	ProjectName  string                   `yaml:"project_name"`
	Services     map[string]Service       `yaml:"services"`

	EnvironmentName string
	Tag             string
}

// Step describes a step in the pipeline
type Step struct {
	Docker  DockerStep  `yaml:"docker"`
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

		config.Options.ClusterName = config.Environments[config.EnvironmentName].ClusterName
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
	if config.Options.ClusterName == "" && envConfig.ClusterName == "" {
		config.Options.ClusterName = "flecs"
	}

	if envConfig.ClusterName != "" {
		config.Options.ClusterName = envConfig.ClusterName
	}

	// Check and set region
	if config.Options.Region == "" && envConfig.Region == "" {
		config.Options.Region = "eu-west-1"
	}

	if envConfig.Region != "" {
		config.Options.Region = envConfig.Region
	}

	// Check and set security group names
	if len(envConfig.SecurityGroupNames) > 0 {
		config.Options.SecurityGroupNames = envConfig.SecurityGroupNames
	}

	// Check and set subnet names
	if len(envConfig.SubnetNames) > 0 {
		config.Options.SubnetNames = envConfig.SubnetNames
	}

	// Merge environment variables
	if len(envConfig.EnvironmentVariables) > 0 {
		for key, value := range envConfig.EnvironmentVariables {
			config.Options.EnvironmentVariables[key] = value
		}
	}

	// Merge secrets
	if len(envConfig.Secrets) > 0 {
		for key, value := range envConfig.Secrets {
			config.Options.Secrets[key] = value
		}
	}

	// Check and set LogGroupName
	if config.Options.LogGroupName == "" && envConfig.LogGroupName != "" {
		config.Options.LogGroupName = envConfig.LogGroupName
	}

	if config.Options.LogGroupName == "" {
		// Set a default log group name if not specified
		config.Options.LogGroupName = fmt.Sprintf("/flecs/%s", config.ProjectName)
	}

	// Check and set ECR region
	if envConfig.ECRRegion != "" {
		config.Options.ECRRegion = envConfig.ECRRegion
	}

	if config.Options.ECRRegion == "" && envConfig.ECRRegion == "" {
		config.Options.ECRRegion = config.Options.Region
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
		dockerSet := step.Docker != (DockerStep{})

		if !serviceSet && !taskSet && !scriptSet && !dockerSet {
			return config, fmt.Errorf("invalid step config on step %d", index)
		}

		// Here we set as a string what kind of step it is
		if serviceSet {
			config.Pipeline[index].Type = "service"
		}

		if taskSet {
			config.Pipeline[index].Type = "task"
		}

		if scriptSet {
			config.Pipeline[index].Type = "script"
		}

		if dockerSet {
			config.Pipeline[index].Type = "docker"
		}
	}

	return config, err
}

func (c Config) getEnvConfig() (env ConfigOptions, err error) {
	if viper.GetString("environment") != "" {
		e := viper.GetString("environment")

		env, ok := c.Environments[e]
		if !ok {
			return env, fmt.Errorf("Cannot find environment config for %s", e)
		}
	}

	return env, err
}
