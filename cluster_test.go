package main

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

var (
	clients Clients
	err     error
)

var config = Config{
	Options: ConfigOptions{
		ClusterName: "test",
	},
}

func TestClusterExists(t *testing.T) {
	var resp bool

	clients = Clients{
		ECS: mockedECSClient{},
	}

	resp, err = clients.ClusterExists(Config{})
	assert.Nil(t, err)

	assert.Equal(t, false, resp, "Cluster does not exist")

	clients = Clients{
		ECS: mockedECSClient{
			DescribeClustersResp: ecs.DescribeClustersOutput{
				Clusters: []*ecs.Cluster{
					&ecs.Cluster{
						ClusterName: aws.String("test"),
						Status:      aws.String("ACTIVE"),
					},
				},
			},
		},
	}

	resp, err = clients.ClusterExists(config)
	assert.Nil(t, err)

	t.Log(resp)

	assert.Equal(t, true, resp, "Cluster exists")
}

func TestCreateCluster(t *testing.T) {
	clients = Clients{
		ECS: mockedECSClient{
			DescribeClustersResp: ecs.DescribeClustersOutput{
				Clusters: []*ecs.Cluster{
					&ecs.Cluster{
						ClusterName: aws.String("test"),
						Status:      aws.String("ACTIVE"),
					},
				},
			},
			CreateClusterResp: ecs.CreateClusterOutput{},
		},
	}

	err = clients.CreateCluster(config, 0)
	assert.Nil(t, err)
}

func TestDeleteCluster(t *testing.T) {
	clients = Clients{
		ECS: mockedECSClient{
			DescribeClustersResp: ecs.DescribeClustersOutput{
				Clusters: []*ecs.Cluster{
					&ecs.Cluster{
						ClusterName: aws.String("test"),
						Status:      aws.String("ACTIVE"),
					},
				},
			},
			DeleteClusterResp: ecs.DeleteClusterOutput{},
		},
	}

	err = clients.DeleteCluster(config)
	assert.Nil(t, err)
}
