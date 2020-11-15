package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// Client sets up a client with configurable options
type Client struct {
	Region string
}

// Clients contains all AWS clients we're using
type Clients struct {
	CloudWatchLogs cloudwatchlogsiface.CloudWatchLogsAPI
	EC2            ec2iface.EC2API
	ECR            ecriface.ECRAPI
	ECS            ecsiface.ECSAPI
	IAM            iamiface.IAMAPI
	STS            stsiface.STSAPI
}

// InitClients sets up all clients that we use
func (c Client) InitClients() (clients Clients, err error) {
	session, err := c.session()
	if err != nil {
		return clients, err
	}

	clients = Clients{
		CloudWatchLogs: cloudwatchlogs.New(session),
		EC2:            ec2.New(session),
		ECR:            ecr.New(session),
		ECS:            ecs.New(session),
		IAM:            iam.New(session),
		STS:            sts.New(session),
	}

	return clients, err
}

func (c Client) session() (sess *session.Session, err error) {
	config := aws.Config{
		Region: aws.String(c.Region),
	}

	sess, err = session.NewSession(&config)
	if err != nil {
		return sess, err
	}

	return sess, err
}
