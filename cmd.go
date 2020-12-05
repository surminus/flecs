package main

import (
	"io/ioutil"

	"github.com/go-git/go-git/v5"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile, flecsFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Path to configuration file")
	cmd.PersistentFlags().StringVarP(&flecsFile, "file", "f", "flecs.yaml", "Path to Flecsfile")

	cmd.PersistentFlags().StringP("environment", "e", "", "An environment (or stage) to deploy to")
	CheckError(viper.BindPFlag("environment", cmd.PersistentFlags().Lookup("environment")))

	cmd.PersistentFlags().StringP("tag", "t", "", "The tag is used by images")
	CheckError(viper.BindPFlag("tag", cmd.PersistentFlags().Lookup("tag")))

	deploy.PersistentFlags().Bool("recreate-services", false, "Force a recreation of all services")
	CheckError(viper.BindPFlag("deploy.recreate_services", deploy.PersistentFlags().Lookup("recreate-services")))

	cmd.AddCommand(deploy, rm)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		CheckError(err)

		// Search config in home directory with name ".flecs_cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".flecs_cli")
	}

	viper.ReadInConfig() //nolint,errcheck

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

		file, err := ioutil.ReadFile(flecsFile)
		CheckError(err)

		// Declare tag
		tag, err := getTag()
		CheckError(err)

		// Each other function should accept the config type
		config, err := LoadConfig(
			file,
			viper.GetString("environment"),
			tag,
			viper.GetBool("deploy.recreate_services"),
		)
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
		file, err := ioutil.ReadFile(flecsFile)
		CheckError(err)

		// Declare tag
		tag, err := getTag()
		CheckError(err)

		// Each other function should accept the config type
		config, err := LoadConfig(
			file,
			viper.GetString("environment"),
			tag,
			viper.GetBool("deploy.recreate_services"),
		)
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

func getTag() (tag string, err error) {
	if viper.GetString("tag") != "" {
		tag = viper.GetString("tag")
	}

	if viper.GetString("tag") == "" {
		r, err := git.PlainOpen(".")
		if err != nil {
			return tag, err
		}

		ref, err := r.Head()
		if err != nil {
			return tag, err
		}

		tag = ref.Hash().String()
	}

	return tag, err
}
