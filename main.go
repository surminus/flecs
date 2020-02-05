package main

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cmdFlagEnvironment string
)

// Config represents all options that can be configured by a flecs config file
type Config struct {
	Environment string
	ClusterName string
	Pipeline    []Task
}

// Task describes a task in a pipeline
type Task struct {
	Command []string
}

// loadConfig will load all configuration options if they exist, allowing
// environment specific options to override top level options
func loadConfig() (config Config) {
	if cmdFlagEnvironment == "" {
		Abort("--environment or -e flag is a required value")
	}

	clusterName := viper.GetString("cluster_name")
	pipeline := viper.GetStringSlice("pipeline")

	envConfig := envConfigOption("")

	if viper.IsSet(envConfig) {
		clusterName = viper.GetString(envConfigOption("cluster_name"))
		pipeline = viper.GetStringSlice(envConfigOption("pipeline"))
	}

	tasks := []Task{}

	for _, task := range pipeline {
		command := strings.Split(task, " ")
		tasks = append(tasks, Task{Command: command})
	}

	config = Config{
		ClusterName: clusterName,
		Environment: cmdFlagEnvironment,
		Pipeline:    tasks,
	}

	return config
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

func main() {
	execute()
}

func execute() {
	cmd.PersistentFlags().StringVarP(&cmdFlagEnvironment, "environment", "e", "", "An environment (or stage) to deploy to")

	viper.SetConfigName("flecs")
	viper.AddConfigPath(".")
	CheckError(viper.ReadInConfig())

	// Run the thing
	if err := cmd.Execute(); err != nil {
		CheckError(err)
	}
}

// Log allows us to output in a consistent way everywhere
var Log = logrus.New()

// cmd is the base command
var cmd = &cobra.Command{
	Use: "flecs",
	Run: func(cmd *cobra.Command, args []string) {
		Log.Info("This is Flecs!")

		// Each other function should accept the config type
		config := loadConfig()

		Log.Info(config)
	},
}

// CheckError will display any errors and quit if found
func CheckError(err error) {
	if err != nil {
		Log.Error(err)
		os.Exit(1)
	}
}

// Abort will log the message and quit
func Abort(message string) {
	Log.Error(message)
	os.Exit(1)
}
