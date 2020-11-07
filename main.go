package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Script runs an arbitary command
type Script struct{}

// Service will update one or many services and wait for completion
type Service struct{}

func main() {
	execute()
}

func execute() {
	cmd.PersistentFlags().StringP("environment", "e", "", "An environment (or stage) to deploy to")
	viper.BindPFlag("environment", cmd.PersistentFlags().Lookup("environment"))

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
		}

		Log.Info("Finished pipeline")
	},
}
