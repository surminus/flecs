package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
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

// ClientECS uses iface to allow us to mock responses in tests
type ClientECS struct {
	Client ecsiface.ECSAPI
}

// ClientEC2 uses iface to allow us to mock responses in tests
type ClientEC2 struct {
	Client ec2iface.EC2API
}

// ClientSTS uses iface to allow us to mock responses in tests
type ClientSTS struct {
	Client stsiface.STSAPI
}

// ClientIAM uses iface to allow us to mock responses in teiam
type ClientIAM struct {
	Client iamiface.IAMAPI
}

// ECS creates an ECS client
func (c Client) ECS() (client ClientECS, err error) {
	session, err := c.session()
	if err != nil {
		return client, err
	}

	client = ClientECS{
		Client: ecs.New(session),
	}

	return client, err
}

// EC2 creates an EC2 client
func (c Client) EC2() (client ClientEC2, err error) {
	session, err := c.session()
	if err != nil {
		return client, err
	}

	client = ClientEC2{
		Client: ec2.New(session),
	}

	return client, err
}

// STS creates an STS client
func (c Client) STS() (client ClientSTS, err error) {
	session, err := c.session()
	if err != nil {
		return client, err
	}

	client = ClientSTS{
		Client: sts.New(session),
	}

	return client, err
}

// IAM creates an IAM client
func (c Client) IAM() (client ClientIAM, err error) {
	session, err := c.session()
	if err != nil {
		return client, err
	}

	client = ClientIAM{
		Client: iam.New(session),
	}

	return client, err
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
