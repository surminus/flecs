package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cmdFlagEnvironment string
)

// Task runs a one-off task in the cluster
type Task struct{}

// Script runs an arbitary command
type Script struct{}

// Service will update one or many services and wait for completion
type Service struct{}

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

// cmd is the base command
var cmd = &cobra.Command{
	Use: "flecs",
	Run: func(cmd *cobra.Command, args []string) {
		Log.Info("Starting pipeline")

		// Each other function should accept the config type
		config, err := LoadConfig()
		CheckError(err)

		for i, step := range config.Pipeline {
			Log.Info(i)
			Log.Info("Step: ", step.Name)
			Log.Info("Class: ", step.Class)
		}

		Log.Info("Finished pipeline")
	},
}
