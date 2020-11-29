package main

import (
	"fmt"
)

// Deploy runs through the pipeline and performs each task
func (config Config) Deploy() (err error) {
	for i, step := range config.Pipeline {
		switch step.Type {
		case "task":
			Log.Infof("[step %d] ==> task", i+1)
			if step.Task.Name != "" {
				Log.Info("Name: ", step.Task.Name)
			}

			client := Client{Region: config.Options.Region}
			_, err = step.Task.Run(client, config)
			if err != nil {
				return err
			}

		case "service":
			Log.Infof("[step %d] ==> service", i+1)
			if step.Service.Name != "" {
				Log.Info("Name: ", step.Service.Name)
			}

			client := Client{Region: config.Options.Region}
			_, err = step.Service.Run(client, config)
			if err != nil {
				return err
			}

		case "script":
			Log.Infof("[step %d] ==> script", i+1)
			if step.Script.Name != "" {
				Log.Info("Name: ", step.Script.Name)
			}

			_, err = step.Script.Run()
			if err != nil {
				return err
			}

		case "docker":
			Log.Infof("[step %d] ==> docker", i+1)
			if step.Docker.Name != "" {
				Log.Infof("Name: %s", step.Docker.Name)
			}

			client := Client{Region: config.Options.Region}
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
	c := Client{Region: config.Options.Region}
	clients, err := c.InitClients()
	if err != nil {
		return err
	}

	switch resource {
	case "service":
		service, ok := config.Services[name]
		if !ok {
			return fmt.Errorf("cannot find service configured called %s", name)
		}

		service.Name = name

		serviceName, err := service.Destroy(clients, config)
		if err != nil {
			return err
		}

		Log.Infof("Deleted service %s", serviceName)

	case "cluster":
		clients.DeleteCluster(config)
	}

	return err
}
