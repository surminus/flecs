package main

import (
	"fmt"
)

// Run runs through the pipeline and performs each task
func (config Config) Run() (err error) {
	for _, step := range config.Pipeline {
		Log.Info("Step: ", step.Type)

		switch step.Type {
		case "task":
			Log.Info("Name: ", step.Task.Name)
		case "service":
			Log.Info("Name: ", step.Service.Name)

			service, ok := config.Services[step.Service.Service]
			if !ok {
				return fmt.Errorf("cannot find service configured called %s", step.Service.Service)
			}

			service.Name = step.Service.Service

			client := Client{Region: config.Region}
			serviceName, err := service.Create(client, config)
			if err != nil {
				return err
			}

			Log.Infof("Configured service %s", serviceName)

		case "script":
			Log.Info("Name: ", step.Script.Name)
			_, err = step.Script.Run()
			if err != nil {
				return err
			}
		default:
			Log.Fatal("Invalid configuration")
		}
	}

	return err
}
