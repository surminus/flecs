package main

import (
	"strings"

	"github.com/spf13/viper"
)

// Config represents all options that can be configured by a flecs config file
type Config struct {
	Environment string
	ClusterName string
	Pipeline    []Step
}

// Step describes a step in the pipeline
type Step struct {
	Name    string
	Script  Script
	Service Service
	Task    Task
}

// LoadConfig will load all configuration options if they exist, allowing
// environment specific options to override top level options
func LoadConfig() (config Config, err error) {
	clusterName := viper.GetString("cluster_name")
	pipeline := viper.Get("pipeline").([]interface{})

	envConfig := ""
	if cmdFlagEnvironment != "" {
		envConfig = envConfigOption("")
	}

	if envConfig != "" {
		if viper.IsSet(envConfig) {
			if viper.IsSet(envConfigOption("cluster_name")) {
				clusterName = viper.GetString(envConfigOption("cluster_name"))
			}

			if viper.IsSet(envConfigOption("pipeline")) {
				pipeline = viper.Get(envConfigOption("pipeline")).([]interface{})
			}
		}
	}

	steps := []Step{}

	for _, step := range pipeline {
		for key, value := range step.(map[interface{}]interface{}) {
			class := key.(string)
			details := value.(map[interface{}]interface{})

			name := details["name"].(string)

			s := Step{Name: name}

			switch class {
			case "task":
				command := findString(details, "command")
				taskDefinition := findString(details, "task_definition")

				s.Task = Task{
					Command:        command,
					TaskDefinition: taskDefinition,
				}
			case "service":
				s.Service = Service{}
			case "script":
				s.Script = Script{}
			default:
				Abort("Configuration validation failed! Pipeline entry \"" + class + "\" not recognised!")
			}

			steps = append(steps, s)
		}
	}

	config = Config{
		ClusterName: clusterName,
		Environment: cmdFlagEnvironment,
		Pipeline:    steps,
	}

	return config, err
}

func findString(values map[interface{}]interface{}, keyword string) (output string) {
	if values[keyword] != nil {
		return values[keyword].(string)

	}

	return output
}

// envConfigOption resolves the name of the config option in the environment
// specific part of the configuration file, using the concept viper uses for
// searching for subkeys, ie "foo.bar.option".
//
// If argument is passed as an empty string, then it returns the plain name of
// the environment subkey, ie environments.[environment]
func envConfigOption(option string) (result string) {
	envConfig := strings.Join([]string{"environments", cmdFlagEnvironment}, ".")
	if option == "" {
		return envConfig
	}

	result = strings.Join([]string{envConfig, option}, ".")
	return result
}
