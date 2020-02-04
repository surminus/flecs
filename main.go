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
}

// loadConfig will load all configuration options if they exist, allowing
// environment specific options to override top level options
func loadConfig() (config Config) {
	if cmdFlagEnvironment == "" {
		Abort("--environment or -e flag is a required value")
	}

	clusterName := viper.GetString("cluster_name")

	envConfig := strings.Join([]string{"environments", cmdFlagEnvironment}, ".")

	if viper.IsSet(envConfig) {
		clusterName = viper.GetString(strings.Join([]string{envConfig, "cluster_name"}, "."))
	}

	config = Config{
		ClusterName: clusterName,
	}

	return config
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
