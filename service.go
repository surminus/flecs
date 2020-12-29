package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/dchest/uniuri"
)

// ServiceStep will update a Service
type ServiceStep struct {
	Service string `yaml:"service"`
}

// Service contains the parameters for creating a service
type Service struct {
	Definition   string      `yaml:"definition"`
	LaunchType   string      `yaml:"launch_type"`
	LoadBalancer string      `yaml:"load_balancer"`
	TargetGroup  TargetGroup `yaml:"target_group"`

	// Name is automatically assigned
	Name string
}

// Run runs the service step
func (s ServiceStep) Run(c Client, cfg Config) (serviceName string, err error) {
	// Set up ECS client
	clients, err := c.InitClients()
	if err != nil {
		return serviceName, err
	}

	service, ok := cfg.Services[s.Service]
	if !ok {
		return serviceName, fmt.Errorf("cannot find service configured called %s", s.Service)
	}

	service.Name = s.Service

	// Check if the cluster exists
	clusterExists, err := clients.ClusterExists(cfg)
	if err != nil {
		return serviceName, err
	}

	// Create a default cluster if it doesn't
	if !clusterExists {
		Log.Infof("Creating cluster %s", cfg.Options.ClusterName)
		err = clients.CreateCluster(cfg, 5)
		if err != nil {
			return serviceName, err
		}
	}

	// If the cluster exists, check if the service exists
	if clusterExists {
		serviceNamePrefix := service.serviceNamePrefix(cfg)
		serviceName, err = service.checkServicePrefixExists(clients, cfg, serviceNamePrefix)
		if err != nil {
			return serviceName, err
		}
	}

	// If the service exists, and we want to recreate, then we have to create
	// a new service, then delete the old service
	if serviceName != "" && cfg.RecreateServices {
		Log.Infof("Re-creating service %s", serviceName)
		newServiceName, err := service.Create(clients, cfg)
		if err != nil {
			return newServiceName, err
		}

		Log.Infof("Created replacement service %s", newServiceName)

		oldServiceName := serviceName
		Log.Infof("Deleting old service %s", oldServiceName)
		err = service.Delete(clients, cfg, oldServiceName)
		if err != nil {
			return newServiceName, err
		}
		Log.Infof("Deleted old service %s", oldServiceName)

		return newServiceName, err
	}

	// Update the service if it already exists
	if serviceName != "" {
		Log.Infof("Updating service %s", serviceName)
		serviceName, err = service.Update(clients, cfg, serviceName)
		if err != nil {
			return serviceName, err
		}

		Log.Infof("Updated service %s", serviceName)
		return serviceName, err
	}

	// Otherwise create the service
	Log.Infof("Creating service %s", serviceName)

	// If a load balancer is configured, then create the load balancer
	if service.LoadBalancer != "" {
		lb, ok := cfg.LoadBalancers[service.LoadBalancer]
		if !ok {
			return serviceName, fmt.Errorf("cannot find load balancer configured called %s", service.LoadBalancer)
		}

		lb.Name = strings.Join([]string{cfg.ProjectName, service.LoadBalancer}, "-")

		_, err := lb.Create(clients, cfg)
		if err != nil {
			return serviceName, err
		}
	}

	serviceName, err = service.Create(clients, cfg)
	if err != nil {
		return serviceName, err
	}

	Log.Infof("Created service %s", serviceName)

	return serviceName, err
}

// Update updates a running service
func (s Service) Update(c Clients, cfg Config, service string) (serviceName string, err error) {
	networkConfiguration, err := c.NetworkConfiguration(cfg)
	if err != nil {
		return serviceName, err
	}

	// Register task definition
	definition, ok := cfg.Definitions[s.Definition]
	if !ok {
		return serviceName, fmt.Errorf("cannot find task definition called %s", s.Definition)
	}

	serviceNamePrefix := s.serviceNamePrefix(cfg)

	taskDefinitionArn, err := definition.Create(c, cfg, serviceNamePrefix)
	if err != nil {
		return serviceName, err
	}
	Log.Infof("Registered task definition %s", taskDefinitionArn)

	input := ecs.UpdateServiceInput{
		Cluster:              aws.String(cfg.Options.ClusterName),
		NetworkConfiguration: &networkConfiguration,
		Service:              aws.String(service),
		TaskDefinition:       aws.String(taskDefinitionArn),
	}

	resp, err := c.ECS.UpdateService(&input)
	if err != nil {
		return serviceName, err
	}

	serviceName = aws.StringValue(resp.Service.ServiceName)

	waitUntilInput := ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Options.ClusterName),
		Services: aws.StringSlice([]string{serviceName}),
	}

	err = c.ECS.WaitUntilServicesStable(&waitUntilInput)
	if err != nil {
		return serviceName, err
	}

	return serviceName, err
}

// Create creates a service if it doesn't exist, and returns the name of the
// service
func (s Service) Create(c Clients, cfg Config) (serviceName string, err error) {
	clientECS := c.ECS

	// serviceNamePrefix ensures that services have unique IDs, which will
	// eventually be used when we have to safely recreate a service
	serviceNamePrefix := s.serviceNamePrefix(cfg)

	networkConfiguration, err := c.NetworkConfiguration(cfg)
	if err != nil {
		return serviceName, err
	}

	// Register task definition
	definition, ok := cfg.Definitions[s.Definition]
	if !ok {
		return serviceName, fmt.Errorf("cannot find task definition called %s", s.Definition)
	}

	taskDefinitionArn, err := definition.Create(c, cfg, serviceNamePrefix)
	if err != nil {
		return serviceName, err
	}
	Log.Infof("Registered task definition %s", taskDefinitionArn)

	// Generate new service name with uuid
	serviceName = strings.Join([]string{serviceNamePrefix, uniuri.NewLen(8)}, "-")

	// Create service
	createServiceInput := ecs.CreateServiceInput{
		Cluster:              aws.String(cfg.Options.ClusterName),
		DesiredCount:         aws.Int64(1),
		LaunchType:           aws.String(s.LaunchType),
		NetworkConfiguration: &networkConfiguration,
		ServiceName:          aws.String(serviceName),
		TaskDefinition:       aws.String(taskDefinitionArn),
	}

	if s.TargetGroup != (TargetGroup{}) {
		loadBalancers := []*ecs.LoadBalancer{
			&ecs.LoadBalancer{
				ContainerName:  aws.String(s.TargetGroup.ContainerName),
				ContainerPort:  aws.Int64(s.TargetGroup.ContainerPort),
				TargetGroupArn: aws.String(s.TargetGroup.TargetGroupArn),
			},
		}

		createServiceInput.SetLoadBalancers(loadBalancers)
	}

	output, err := clientECS.CreateService(&createServiceInput)
	if err != nil {
		return serviceName, err
	}

	Log.Infof("Waiting for service to be ready: %s", aws.StringValue(output.Service.ServiceArn))

	// Wait for service to become stable
	err = clientECS.WaitUntilServicesStable(&ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Options.ClusterName),
		Services: aws.StringSlice([]string{aws.StringValue(output.Service.ServiceName)}),
	})
	if err != nil {
		return serviceName, err
	}

	return serviceName, err
}

// Delete deletes a service (but not created log groups, clusters or roles)
func (s Service) Delete(c Clients, cfg Config, service string) (err error) {
	clientECS := c.ECS

	deleteServiceInput := ecs.DeleteServiceInput{
		Cluster: aws.String(cfg.Options.ClusterName),
		Force:   aws.Bool(true),
		Service: aws.String(service),
	}

	_, err = clientECS.DeleteService(&deleteServiceInput)
	if err != nil {
		return err
	}

	for count := 0; count < 30; count++ {
		serviceExists, err := s.checkServiceExists(c, cfg, service)
		if err != nil {
			return err
		}

		if serviceExists {
			break
		}

		Log.Infof("Waiting for service %s to terminate", service)
		time.Sleep(10 * time.Second)
	}

	return err
}

func (s Service) checkServiceExists(c Clients, cfg Config, service string) (result bool, err error) {
	input := ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Options.ClusterName),
		Services: []*string{aws.String(service)},
	}

	resp, err := c.ECS.DescribeServices(&input)
	if err != nil {
		return result, err
	}

	result = len(resp.Services) > 0
	return result, err
}

func (s Service) checkServicePrefixExists(c Clients, cfg Config, serviceNamePrefix string) (serviceName string, err error) {
	client := c.ECS

	// Check if service already exists, if so, return early with the name
	listServiceInput := ecs.ListServicesInput{
		Cluster:    aws.String(cfg.Options.ClusterName),
		LaunchType: aws.String(s.LaunchType),
		MaxResults: aws.Int64(100),
	}
	listServiceOutput, err := client.ListServices(&listServiceInput)
	if err != nil {
		return serviceName, err
	}

	serviceNameRe, err := regexp.Compile(fmt.Sprintf(`^%s-\w+$`, serviceNamePrefix))
	if err != nil {
		return serviceName, err
	}

	for _, s := range listServiceOutput.ServiceArns {
		arn, err := arn.Parse(aws.StringValue(s))
		if err != nil {
			return serviceName, err
		}

		var name string

		nsl := strings.Split(arn.Resource, "/")
		if len(nsl) < 1 {
			name = arn.Resource
		} else {
			name = nsl[len(nsl)-1]
		}

		if serviceNameRe.MatchString(name) {
			serviceName = name
			break
		}
	}

	return serviceName, err
}

func (s Service) serviceNamePrefix(cfg Config) (serviceNamePrefix string) {
	// Configure service name
	if cfg.ProjectName == "flecs" {
		serviceNamePrefix = "flecs"
	} else {
		serviceNamePrefix = strings.Join([]string{"flecs", cfg.ProjectName}, "-")
	}

	if cfg.ProjectName != s.Name {
		serviceNamePrefix = strings.Join([]string{serviceNamePrefix, s.Name}, "-")
	}

	return serviceNamePrefix
}
