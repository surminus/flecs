package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
)

// Client sets up a client with configurable options
type Client struct {
	Region string
}

// ClientECS uses iface to allow us to mock responses in tests
type ClientECS struct {
	Client ecsiface.ECSAPI
}

// ECS is used by Cobra upstream to initiate a client
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
