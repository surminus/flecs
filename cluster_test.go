package main

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

func TestClusterExists(t *testing.T) {
	var (
		clients Clients
		resp    bool
		err     error
	)

	clients = Clients{
		ECS: mockedDescribeClusters{Resp: ecs.DescribeClustersOutput{}},
	}

	resp, err = clients.ClusterExists(Config{})
	assert.Nil(t, err)

	assert.Equal(t, false, resp, "Cluster does not exist")

	clients = Clients{
		ECS: mockedDescribeClusters{
			Resp: ecs.DescribeClustersOutput{
				Clusters: []*ecs.Cluster{
					&ecs.Cluster{
						ClusterName: aws.String("test"),
						Status:      aws.String("ACTIVE"),
					},
				},
			},
		},
	}

	resp, err = clients.ClusterExists(
		Config{
			Options: ConfigOptions{
				ClusterName: "test",
			},
		},
	)
	assert.Nil(t, err)

	t.Log(resp)

	assert.Equal(t, true, resp, "Cluster exists")
}
