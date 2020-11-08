package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// ServiceStep will update a Service
type ServiceStep struct {
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Service     string `yaml:"service"`
}

type Service struct {
	Definition     string `yaml:"definition"`
	LaunchType     string `yaml:"launch_type"`
	Name           string
	TargetGroupArn string `yaml:"target_group_arn"`
}

func (s Service) Create(c ClientECS, cfg Config) (serviceName string, err error) {
	client := c.Client

	// Generate name
	serviceNamePrefix := strings.Join([]string{"flecs", cfg.ProjectName}, "-")
	if cfg.ProjectName != s.Name {
		serviceNamePrefix = strings.Join([]string{serviceNamePrefix, s.Name}, "-")
	}

	// Check if the service already exists
	serviceName, err = s.checkServiceExists(c, cfg, serviceNamePrefix)
	if err != nil || serviceName != "" {
		return serviceName, err
	}

	// Get security group IDs

	// Get subnet IDs

	// Register task definition

	// Create service
	createServiceInput := ecs.CreateServiceInput{
		Cluster:      aws.String(cfg.ClusterName),
		DesiredCount: aws.Int64(1),
		LaunchType:   aws.String(s.LaunchType),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				SecurityGroups: []*string{},
				Subnets:        []*string{},
			},
		},
		ServiceName:    aws.String(serviceName),
		TaskDefinition: aws.String(""),
	}

	output, err := client.CreateService(&createServiceInput)

	// Wait for service to become stable

	return serviceName, err
}

func (s Service) Update() (serviceName string, err error) {
	return serviceName, err
}

func (s Service) checkServiceExists(c ClientECS, cfg Config, serviceNamePrefix string) (serviceName string, err error) {
	client := c.Client

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
