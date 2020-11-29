package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().StringVarP(&cfgFile, "file", "f", "", "Path to config file")

	cmd.PersistentFlags().StringP("environment", "e", "", "An environment (or stage) to deploy to")
	CheckError(viper.BindPFlag("environment", cmd.PersistentFlags().Lookup("environment")))

	cmd.PersistentFlags().StringP("tag", "t", "", "The tag is used by images")
	CheckError(viper.BindPFlag("tag", cmd.PersistentFlags().Lookup("tag")))

	cmd.AddCommand(deploy, rm)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigName("flecs")
	}

	CheckError(viper.ReadInConfig())

	viper.SetEnvPrefix("flecs")
	viper.AutomaticEnv()
}

// cmd is the base command
var cmd = &cobra.Command{
	Use: "flecs",
}

// deploy is used for running through the pipeline from start to finish
var deploy = &cobra.Command{
	Use:   "deploy",
	Short: "Run through the configured pipeline",
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
	Use:   "rm [resource] [name]",
	Short: "Run through the configured pipeline",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Each other function should accept the config type
		config, err := LoadConfig()
		CheckError(err)

		switch args[0] {
		case "service":
			if len(args) < 2 {
				Log.Fatal("Must specify service name")
			}

			err = config.Remove("service", args[1])
			CheckError(err)
		case "cluster":
			if len(args) > 1 {
				Log.Fatal("Deleting cluster does not take any arguments. Use --environment to choose different clusters.")
			}

			err = config.Remove("cluster", "")
			CheckError(err)
		default:
			Log.Fatalf("Unrecognised resource %s", args[0])
		}
	},
}
