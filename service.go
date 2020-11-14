package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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

	// Check if the cluster exists
	describeClusterInput := ecs.DescribeClustersInput{
		Clusters: aws.StringSlice([]string{cfg.ClusterName}),
	}
	describeCluster, err := clientECS.Client.DescribeClusters(&describeClusterInput)
	if err != nil {
		return serviceName, err
	}
	clusterMissing := false

	if len(describeCluster.Clusters) > 0 {
		if aws.StringValue(describeCluster.Clusters[0].Status) != "ACTIVE" {
			clusterMissing = true
		}
	} else {
		clusterMissing = true
	}

	if !clusterMissing {
		Log.Infof("Using cluster %s", aws.StringValue(describeCluster.Clusters[0].ClusterName))
	}

	// Create a default cluster if it doesn't
	if clusterMissing {
		Log.Infof("Creating cluster %s", cfg.ClusterName)

		createClusterInput := ecs.CreateClusterInput{
			ClusterName: aws.String(cfg.ClusterName),
		}
		_, err := clientECS.Client.CreateCluster(&createClusterInput)
		if err != nil {
			return serviceName, err
		}

		clusterCreated := false
		for count := 0; count < 30; count++ {
			Log.Infof("Waiting for cluster to provision")
			time.Sleep(10 * time.Second)

			describeClusterInput := ecs.DescribeClustersInput{
				Clusters: aws.StringSlice([]string{cfg.ClusterName}),
			}
			describeCluster, err := clientECS.Client.DescribeClusters(&describeClusterInput)
			if err != nil {
				return serviceName, err
			}

			if len(describeCluster.Clusters) < 1 {
				continue
			}

			cluster := describeCluster.Clusters[0]
			if aws.StringValue(cluster.Status) == "ACTIVE" {
				clusterCreated = true
				break
			}
		}

		if !clusterCreated {
			Log.Fatal("Failed to create cluster!")
		}

	}

	serviceNamePrefix := s.serviceNamePrefix(cfg)

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

	assignPublicIP := cfg.AssignPublicIP

	// Get subnet IDs
	subnetIDs, err := clientEC2.getSubnetIDs(cfg.SubnetNames)
	if err != nil {
		return serviceName, err
	}

	// Use default VPC subnets if no subnets configured
	if len(subnetIDs) == 0 {
		subnetIDs, err = clientEC2.getDefaultSubnetIDs()
		if err != nil {
			return serviceName, err
		}

		// Always assign a public IP for default subnets
		assignPublicIP = true
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

	if assignPublicIP {
		createServiceInput.NetworkConfiguration.AwsvpcConfiguration.SetAssignPublicIp("ENABLED")
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

	output, err := client.CreateService(&createServiceInput)
	if err != nil {
		return serviceName, err
	}

	Log.Infof("Created service %s", aws.StringValue(output.Service.ServiceArn))

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

// Delete a service (but not created log groups, clusters or roles)
func (s Service) Destroy(c Client, cfg Config) (serviceName string, err error) {
	// Set up ECS client
	clientECS, err := c.ECS()
	if err != nil {
		return serviceName, err
	}

	// Check if the service already exists
	serviceName, err = s.checkServiceExists(clientECS, cfg, s.serviceNamePrefix(cfg))
	if err != nil || serviceName == "" {
		return serviceName, err
	}

	deleteServiceInput := ecs.DeleteServiceInput{
		Cluster: aws.String(cfg.ClusterName),
		Force:   aws.Bool(true),
		Service: aws.String(serviceName),
	}

	Log.Info("Deleting service")
	_, err = clientECS.Client.DeleteService(&deleteServiceInput)
	if err != nil {
		return serviceName, err
	}

	for count := 0; count < 30; count++ {
		serviceName, err = s.checkServiceExists(clientECS, cfg, s.serviceNamePrefix(cfg))
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

func (c ClientEC2) getDefaultSubnetIDs() (ids []string, err error) {
	result, err := c.Client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("default-for-az"),
				Values: aws.StringSlice([]string{"true"}),
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, s := range result.Subnets {
		ids = append(ids, aws.StringValue(s.SubnetId))
	}

	return ids, err
}

func (s Service) serviceNamePrefix(cfg Config) (serviceNamePrefix string) {
	// Configure service name
	serviceNamePrefix = strings.Join([]string{"flecs", cfg.ProjectName}, "-")
	if cfg.ProjectName != s.Name {
		serviceNamePrefix = strings.Join([]string{serviceNamePrefix, s.Name}, "-")
	}

	return serviceNamePrefix
}
