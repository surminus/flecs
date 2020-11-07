package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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

		for _, step := range config.Pipeline {
			Log.Info("Step: ", step.Type)

			switch step.Type {
			case "task":
				Log.Info("Name: ", step.Task.Name)
			case "service":
				Log.Info("Name: ", step.Service.Name)
			case "script":
				Log.Info("Name: ", step.Script.Name)
			default:
				Log.Fatal("Invalid configuration")
			}
		}

		Log.Info("Finished pipeline")
	},
}
