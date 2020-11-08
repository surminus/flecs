package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	Definition     string `yaml:"definition"`
	LaunchType     string `yaml:"launch_type"`
	Name           string
	TargetGroupArn string `yaml:"target_group_arn"`
}

// Create creates a service if it doesn't exist, and returns the name of the
// service
func (s Service) Create(c Client, cfg Config) (serviceName string, err error) {
	// Set up ECS client
	clientECS, err := c.ECS()
	if err != nil {
		return serviceName, err
	}
	client := clientECS.Client

	// Set up EC2 client
	clientEC2, err := c.EC2()
	if err != nil {
		return serviceName, err
	}

	// Configure service name
	serviceNamePrefix := strings.Join([]string{"flecs", cfg.ProjectName}, "-")
	if cfg.ProjectName != s.Name {
		serviceNamePrefix = strings.Join([]string{serviceNamePrefix, s.Name}, "-")
	}

	// Check if the service already exists
	serviceName, err = s.checkServiceExists(clientECS, cfg, serviceNamePrefix)
	if err != nil || serviceName != "" {
		return serviceName, err
	}

	// Get security group IDs
	securityGroupIDs, err := clientEC2.getSecurityGroupIDs(cfg.SecurityGroupNames)
	if err != nil {
		return serviceName, err
	}

	// Get subnet IDs
	subnetIDs, err := clientEC2.getSubnetIDs(cfg.SubnetNames)
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
		Cluster:      aws.String(cfg.ClusterName),
		DesiredCount: aws.Int64(1),
		LaunchType:   aws.String(s.LaunchType),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				SecurityGroups: aws.StringSlice(securityGroupIDs),
				Subnets:        aws.StringSlice(subnetIDs),
			},
		},
		ServiceName:    aws.String(serviceName),
		TaskDefinition: aws.String(taskDefinitionArn),
	}

	output, err := client.CreateService(&createServiceInput)

	// Wait for service to become stable
	err = client.WaitUntilServicesStable(&ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.ClusterName),
		Services: aws.StringSlice([]string{aws.StringValue(output.Service.ServiceArn)}),
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

func (c ClientEC2) getSecurityGroupIDs(names []string) (ids []string, err error) {
	input := ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-name"),
				Values: aws.StringSlice(names),
			},
		},
	}

	result, err := c.Client.DescribeSecurityGroups(&input)
	if err != nil {
		return ids, err
	}

	for _, g := range result.SecurityGroups {
		ids = append(ids, aws.StringValue(g.GroupId))
	}

	return ids, err
}

func (c ClientEC2) getSubnetIDs(names []string) (ids []string, err error) {
	input := ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice(names),
			},
		},
	}

	result, err := c.Client.DescribeSubnets(&input)
	if err != nil {
		return ids, err
	}

	for _, s := range result.Subnets {
		ids = append(ids, aws.StringValue(s.SubnetId))
	}

	return ids, err
}
