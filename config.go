package main

import (
	"fmt"
	"regexp"

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
	Options ConfigOptions `yaml:",inline"`

	Definitions  map[string]Definition    `yaml:"definitions"`
	Environments map[string]ConfigOptions `yaml:"environments"`
	ProjectName  string                   `yaml:"project_name"`
	Services     map[string]Service       `yaml:"services"`

	EnvironmentName string
	Tag             string

	RecreateServices bool
}

// Step describes a step in the pipeline
type Step struct {
	Docker  DockerStep  `yaml:",inline"`
	Script  ScriptStep  `yaml:",inline"`
	Service ServiceStep `yaml:",inline"`
	Task    TaskStep    `yaml:",inline"`

	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
}

// LoadConfig will load all configuration options if they exist, allowing
// environment specific options to override top level options
func LoadConfig(yamlConfig, environment, tag, projectName string, recreate bool) (config Config, err error) {
	// These are special key words that can be used the configuration to allow
	// dynamic naming of resources
	conf := regexp.MustCompile(`{{\s*environment\s*}}`).ReplaceAllString(yamlConfig, environment)
	conf = regexp.MustCompile(`{{\s*tag\s*}}`).ReplaceAllString(conf, tag)
	conf = regexp.MustCompile(`{{\s*project_name\s*}}`).ReplaceAllString(conf, projectName)

	err = yaml.Unmarshal([]byte(conf), &config)
	if err != nil {
		return config, err
	}

	// Configure default options
	if environment != "" {
		config.EnvironmentName = environment
	}

	// Set tag
	config.Tag = tag

	// Load environment config
	envConfig, err := config.getEnvConfig(environment)
	if err != nil {
		return config, err
	}

	// Set default project name
	if config.ProjectName == "" && projectName == "" {
		config.ProjectName = "default"
	}

	if config.ProjectName == "" && projectName != "" {
		config.ProjectName = projectName
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

	config.RecreateServices = recreate

	// Check and set Pipeline
	if len(config.Options.Pipeline) < 1 && len(envConfig.Pipeline) < 1 {
		return config, fmt.Errorf("pipeline configuration not found")
	}

	if len(envConfig.Pipeline) > 0 {
		config.Options.Pipeline = envConfig.Pipeline
	}

	// Check Pipeline for syntax errors
	for index, step := range config.Options.Pipeline {
		if step.Type == "" {
			return config, fmt.Errorf("must specify step \"type\"")
		}

		validSteps := []string{
			"docker",
			"script",
			"service",
			"task",
		}

		valid := false
		for _, t := range validSteps {
			if t == step.Type {
				valid = true
				break
			}
		}

		if valid {
			continue
		}

		return config, fmt.Errorf("invalid step config on step %d", index)
	}

	return config, err
}

func (c Config) getEnvConfig(environment string) (env ConfigOptions, err error) {
	if environment != "" {
		e := environment

		var ok bool
		env, ok = c.Environments[e]
		if !ok {
			return env, err
		}
	}

	return env, err
}
