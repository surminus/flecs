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
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Service     string `yaml:"service"`
}

// Service contains the parameters for creating a service
type Service struct {
	Definition   string `yaml:"definition"`
	LaunchType   string `yaml:"launch_type"`
	Name         string
	LoadBalancer LoadBalancer `yaml:"load_balancer"`
}

// LoadBalancer configures a load balancer that has been created elsewhere
type LoadBalancer struct {
	TargetGroupArn string `yaml:"target_group_arn"`
	ContainerName  string `yaml:"container_name"`
	ContainerPort  int64  `yaml:"container_port"`
}

// Create creates a service if it doesn't exist, and returns the name of the
// service
func (s Service) Create(c Client, cfg Config) (serviceName string, err error) {
	// Set up ECS client
	clients, err := c.InitClients()
	if err != nil {
		return serviceName, err
	}

	clientECS := clients.ECS

	// Check if the cluster exists
	clusterExists, err := clients.ClusterExists(cfg)
	if err != nil {
		return serviceName, err
	}

	// Create a default cluster if it doesn't
	if !clusterExists {
		err = clients.CreateCluster(cfg)
		if err != nil {
			return serviceName, err
		}
	}

	serviceNamePrefix := s.serviceNamePrefix(cfg)

	// Check if the service already exists
	serviceName, err = s.checkServiceExists(clients, cfg, serviceNamePrefix)
	if err != nil || serviceName != "" {
		return serviceName, err
	}

	networkConfiguration, err := clients.NetworkConfiguration(cfg)
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
		Cluster:              aws.String(cfg.ClusterName),
		DesiredCount:         aws.Int64(1),
		LaunchType:           aws.String(s.LaunchType),
		NetworkConfiguration: &networkConfiguration,
		ServiceName:          aws.String(serviceName),
		TaskDefinition:       aws.String(taskDefinitionArn),
	}

	if s.LoadBalancer != (LoadBalancer{}) {
		loadBalancers := []*ecs.LoadBalancer{
			&ecs.LoadBalancer{
				ContainerName:  aws.String(s.LoadBalancer.ContainerName),
				ContainerPort:  aws.Int64(s.LoadBalancer.ContainerPort),
				TargetGroupArn: aws.String(s.LoadBalancer.TargetGroupArn),
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
		Cluster:  aws.String(cfg.ClusterName),
		Services: aws.StringSlice([]string{aws.StringValue(output.Service.ServiceName)}),
	})
	if err != nil {
		return serviceName, err
	}

	return serviceName, err
}

// Update updates a service that already exists
func (s Service) Update() (serviceName string, err error) {
	return serviceName, err
}

// Delete a service (but not created log groups, clusters or roles)
func (s Service) Destroy(c Client, cfg Config) (serviceName string, err error) {
	// Set up ECS client
	clients, err := c.InitClients()
	if err != nil {
		return serviceName, err
	}

	clientECS := clients.ECS

	// Check if the service already exists
	serviceName, err = s.checkServiceExists(clients, cfg, s.serviceNamePrefix(cfg))
	if err != nil || serviceName == "" {
		return serviceName, err
	}

	deleteServiceInput := ecs.DeleteServiceInput{
		Cluster: aws.String(cfg.ClusterName),
		Force:   aws.Bool(true),
		Service: aws.String(serviceName),
	}

	Log.Info("Deleting service")
	_, err = clientECS.DeleteService(&deleteServiceInput)
	if err != nil {
		return serviceName, err
	}

	for count := 0; count < 30; count++ {
		serviceName, err = s.checkServiceExists(clients, cfg, s.serviceNamePrefix(cfg))
		if err != nil {
			return serviceName, err
		}

		if serviceName == "" {
			break
		}

		Log.Info("Waiting for service to terminate")
		time.Sleep(10 * time.Second)
	}

	return serviceName, err
}

func (s Service) checkServiceExists(c Clients, cfg Config, serviceNamePrefix string) (serviceName string, err error) {
	client := c.ECS

	// Check if service already exists, if so, return early with the name
	listServiceInput := ecs.ListServicesInput{
		Cluster:    aws.String(cfg.ClusterName),
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
	serviceNamePrefix = strings.Join([]string{"flecs", cfg.ProjectName}, "-")
	if cfg.ProjectName != s.Name {
		serviceNamePrefix = strings.Join([]string{serviceNamePrefix, s.Name}, "-")
	}

	return serviceNamePrefix
}
