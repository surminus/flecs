package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
)

// Execute executes the root command
func Execute() error {
	return cmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().StringVarP(&cfgFile, "file", "f", "", "Path to config file")

	cmd.PersistentFlags().StringP("environment", "e", "", "An environment (or stage) to deploy to")
	viper.BindPFlag("environment", cmd.PersistentFlags().Lookup("environment"))

	cmd.PersistentFlags().StringP("tag", "t", "", "The tag is used by images")
	viper.BindPFlag("tag", cmd.PersistentFlags().Lookup("tag"))

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
	Args:  cobra.ExactArgs(2),
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
