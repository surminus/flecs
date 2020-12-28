package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"
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

// TargetGroup specifies details for a target group. If you wish to use
// an existing target group, specify it in the main service configuration
type TargetGroup struct {
	ContainerName  string `yaml:"container_name"`
	ContainerPort  int64  `yaml:"container_port"`
	TargetGroupArn string `yaml:"target_group_arn"`
}

// LoadBalancer configures a load balancer, and creates it if it does not
// already exist.
type LoadBalancer struct {
	CertificateArn string      `yaml:"certificate_arn"`
	Port           int64       `yaml:"port"`
	Protocol       string      `yaml:"protocol"`
	SSLPolicy      string      `yaml:"ssl_policy"`
	TargetGroup    TargetGroup `yaml:"target_group"`

	// Name is automatically assigned
	Name string

	// belows values are used internally
	arn string
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

// Create creates a load balancer and all required components
func (l LoadBalancer) Create(c Clients, cfg Config) (alb LoadBalancer, err error) {
	alb = l

	// Check load balancer exists
	describeInput := elbv2.DescribeLoadBalancersInput{
		Names: aws.StringSlice([]string{l.Name}),
	}

	lbs, err := c.ELB.DescribeLoadBalancers(&describeInput)
	if err != nil {
		return alb, err
	}

	// if it already exists then skip creation of load balancer
	if len(lbs.LoadBalancers) > 0 {
		alb.arn = aws.StringValue(lbs.LoadBalancers[0].LoadBalancerArn)
	}

	// Create security group to allow access from load balancer to service

	// Create security group to allow access from public to load balancer

	// Create load balancer if required
	if alb.arn == "" {
		network, err := c.NetworkConfiguration(cfg)
		if err != nil {
			return alb, err
		}

		subnets := network.AwsvpcConfiguration.Subnets

		input := elbv2.CreateLoadBalancerInput{
			Name:           aws.String(l.Name),
			SecurityGroups: []*string{}, // attach previously created sgs
			Subnets:        subnets,
		}

		lb, err := c.ELB.CreateLoadBalancer(&input)
		if err != nil {
			return alb, err
		}

		alb.arn = aws.StringValue(lb.LoadBalancers[0].LoadBalancerArn)

		// Wait for it to provision
		err = c.ELB.WaitUntilLoadBalancerAvailable(&describeInput)
		if err != nil {
			return alb, err
		}
	}

	// Check if the target group exists
	describeTargetGroupsInput := elbv2.DescribeTargetGroupsInput{
		Names: aws.StringSlice([]string{l.Name}),
	}

	describeTargetGroupsOutput, err := c.ELB.DescribeTargetGroups(&describeTargetGroupsInput)
	if err != nil {
		return alb, err
	}

	if len(describeTargetGroupsOutput.TargetGroups) > 0 {
		tg := describeTargetGroupsOutput.TargetGroups[0]
		alb.TargetGroup = TargetGroup{
			TargetGroupArn: aws.StringValue(tg.TargetGroupArn),
			ContainerPort:  aws.Int64Value(tg.Port),
		}
	}

	// Create target group if it does not exist
	if alb.TargetGroup.TargetGroupArn == "" {
		targetGroupInput := elbv2.CreateTargetGroupInput{
			Name:       aws.String(l.Name),
			TargetType: aws.String("ip"),
		}

		targetGroup, err := c.ELB.CreateTargetGroup(&targetGroupInput)
		if err != nil {
			return alb, err
		}

		tg := targetGroup.TargetGroups[0]

		alb.TargetGroup = TargetGroup{
			TargetGroupArn: aws.StringValue(tg.TargetGroupArn),
			ContainerPort:  aws.Int64Value(tg.Port),
		}
	}

	// Create listener, attached to load balancer, using target group as
	// default rule
	if l.Port == 0 {
		// Default to HTTPS
		l.Port = 443
	}

	if l.Protocol == "" {
		// Default to HTTPS
		l.Protocol = "HTTPS"
	}

	if l.Protocol == "HTTPS" && l.CertificateArn == "" {
		return alb, fmt.Errorf("must set certificate arn when using HTTPS")
	}

	defaultActions := []*elbv2.Action{
		&elbv2.Action{
			Order:          aws.Int64(10),
			TargetGroupArn: aws.String(alb.TargetGroup.TargetGroupArn),
			Type:           aws.String("forward"),
		},
	}

	listenerInput := elbv2.CreateListenerInput{
		DefaultActions: defaultActions,
		Port:           aws.Int64(l.Port),
		Protocol:       aws.String(l.Protocol),
	}

	if l.SSLPolicy != "" {
		listenerInput.SslPolicy = aws.String(l.SSLPolicy)
	}

	if l.CertificateArn != "" {
		listenerInput.Certificates = []*elbv2.Certificate{
			&elbv2.Certificate{
				CertificateArn: aws.String(l.CertificateArn),
				IsDefault:      aws.Bool(true),
			},
		}
	}

	return alb, err
}
