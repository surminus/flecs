package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	execute()
}

func execute() {
	viper.SetEnvPrefix("flecs")
	viper.AutomaticEnv()

	cmd.PersistentFlags().StringP("environment", "e", "", "An environment (or stage) to deploy to")
	viper.BindPFlag("environment", cmd.PersistentFlags().Lookup("environment"))

	cmd.PersistentFlags().StringP("tag", "t", "", "The tag is used by images")
	viper.BindPFlag("tag", cmd.PersistentFlags().Lookup("tag"))

	viper.SetConfigName("flecs")
	viper.AddConfigPath(".")
	CheckError(viper.ReadInConfig())

	cmd.AddCommand(deploy, rm)

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

		err = config.Deploy()
		CheckError(err)

		Log.Info("END")
	},
}

// rm is used for deleting resources
var rm = &cobra.Command{
	Use:  "rm",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Each other function should accept the config type
		config, err := LoadConfig()
		CheckError(err)

		switch args[0] {
		case "service":
			err = config.Remove("service", args[1])
			CheckError(err)
		default:
			Log.Fatalf("Unrecognised resource %s", args[0])
		}
	},
}
