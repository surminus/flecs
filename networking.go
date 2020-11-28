package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// GetSecurityGroupIDs returns the IDs of security groups filtered by the
// security group name
func (c Clients) GetSecurityGroupIDs(names []string) (ids []string, err error) {
	input := ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-name"),
				Values: aws.StringSlice(names),
			},
		},
	}

	result, err := c.EC2.DescribeSecurityGroups(&input)
	if err != nil {
		return ids, err
	}

	for _, g := range result.SecurityGroups {
		ids = append(ids, aws.StringValue(g.GroupId))
	}

	return ids, err
}

// GetSubnetIDs returns the IDs of subnets given by their name
func (c Clients) GetSubnetIDs(names []string) (ids []string, err error) {
	input := ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice(names),
			},
		},
	}

	result, err := c.EC2.DescribeSubnets(&input)
	if err != nil {
		return ids, err
	}

	for _, s := range result.Subnets {
		ids = append(ids, aws.StringValue(s.SubnetId))
	}

	return ids, err
}

func (c Clients) GetDefaultSubnetIDs() (ids []string, err error) {
	result, err := c.EC2.DescribeSubnets(&ec2.DescribeSubnetsInput{
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

func (c Clients) NetworkConfiguration(cfg Config) (out ecs.NetworkConfiguration, err error) {
	// Get security group IDs
	securityGroupIDs, err := c.GetSecurityGroupIDs(cfg.Options.SecurityGroupNames)
	if err != nil {
		return out, err
	}

	assignPublicIP := cfg.Options.AssignPublicIP

	// Get subnet IDs
	subnetIDs, err := c.GetSubnetIDs(cfg.Options.SubnetNames)
	if err != nil {
		return out, err
	}

	// Use default VPC subnets if no subnets configured
	if len(subnetIDs) == 0 {
		subnetIDs, err = c.GetDefaultSubnetIDs()
		if err != nil {
			return out, err
		}

		// Always assign a public IP for default subnets
		assignPublicIP = true
	}

	out = ecs.NetworkConfiguration{
		AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
			SecurityGroups: aws.StringSlice(securityGroupIDs),
			Subnets:        aws.StringSlice(subnetIDs),
		},
	}

	if assignPublicIP {
		out.AwsvpcConfiguration.SetAssignPublicIp("ENABLED")
	}

	return out, err
}
