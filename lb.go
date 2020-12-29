package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

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
	arn             string
	securityGroupID string
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
	createSecurityGroupInput := ec2.CreateSecurityGroupInput{
		Description: aws.String("Managed by Flecs"),
		GroupName:   aws.String(l.Name),
	}

	securityGroup, err := c.EC2.CreateSecurityGroup(&createSecurityGroupInput)
	if err != nil {
		return alb, err
	}

	alb.securityGroupID = aws.StringValue(securityGroup.GroupId)

	ingressInput := ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: securityGroup.GroupId,
		IpPermissions: []*ec2.IpPermission{
			{
				FromPort:   aws.Int64(l.Port),
				IpProtocol: aws.String("tcp"),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Public access to load balancer"),
					},
				},
				ToPort: aws.Int64(l.Port),
			},
		},
	}

	_, err = c.EC2.AuthorizeSecurityGroupIngress(&ingressInput)
	if err != nil {
		return alb, err
	}

	egressInput := ec2.AuthorizeSecurityGroupEgressInput{
		GroupId: securityGroup.GroupId,
		IpPermissions: []*ec2.IpPermission{
			{
				FromPort:   aws.Int64(-1),
				IpProtocol: aws.String("-1"),
				IpRanges: []*ec2.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Allow all traffic outbound"),
					},
				},
				ToPort: aws.Int64(-1),
			},
		},
	}

	_, err = c.EC2.AuthorizeSecurityGroupEgress(&egressInput)
	if err != nil {
		return alb, err
	}

	// Create load balancer if required
	if alb.arn == "" {
		network, err := c.NetworkConfiguration(cfg)
		if err != nil {
			return alb, err
		}

		subnets := network.AwsvpcConfiguration.Subnets
		sgs := []*string{securityGroup.GroupId}

		input := elbv2.CreateLoadBalancerInput{
			Name:           aws.String(l.Name),
			SecurityGroups: sgs,
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

	// Check that the listener exists
	describeListenersInput := elbv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(alb.arn),
	}

	listeners, err := c.ELB.DescribeListeners(&describeListenersInput)
	if err != nil {
		return alb, err
	}

	var listenerExists bool
	for _, listener := range listeners.Listeners {
		if aws.Int64Value(listener.Port) == l.Port {
			listenerExists = true
		}
	}

	// Create listener, attached to load balancer, using target group as
	// default rule
	if !listenerExists {
		if l.Port == 0 {
			// Default to HTTP
			l.Port = 80
		}

		if l.Protocol == "" {
			// Default to HTTP
			l.Protocol = "HTTP"
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

		_, err = c.ELB.CreateListener(&listenerInput)
		if err != nil {
			return alb, err
		}
	}

	return alb, err
}
