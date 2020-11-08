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

	cmd.AddCommand(deploy)

	// Run the thing
	if err := cmd.Execute(); err != nil {
		CheckError(err)
	}
}

// cmd is the base command
var cmd = &cobra.Command{
	Use: "flecs",
}

// deploy is used for running through the pipeline from start to finish
var deploy = &cobra.Command{
	Use: "deploy",
	Run: func(cmd *cobra.Command, args []string) {
		Log.Info("START")

		// Each other function should accept the config type
		config, err := LoadConfig()
		CheckError(err)

		err = config.Run()
		CheckError(err)

		Log.Info("END")
	},
}
