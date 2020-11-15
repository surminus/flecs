package main

import (
	"fmt"
)

// Deploy runs through the pipeline and performs each task
func (config Config) Deploy() (err error) {
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
		case "docker":
			Log.Infof("Name: %s", step.Docker.Name)
			client := Client{Region: config.Region}
			err = step.Docker.Run(client, config)
			if err != nil {
				return err
			}
		default:
			Log.Fatal("Invalid configuration")
		}
	}

	return err
}

// Remove deletes a resource
func (config Config) Remove(resource, name string) (err error) {
	switch resource {
	case "service":
		service, ok := config.Services[name]
		if !ok {
			return fmt.Errorf("cannot find service configured called %s", name)
		}

		service.Name = name

		client := Client{Region: config.Region}
		serviceName, err := service.Destroy(client, config)
		if err != nil {
			return err
		}

		Log.Infof("Deleted service %s", serviceName)
	}

	return err
}
