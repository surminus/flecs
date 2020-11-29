package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// ClusterExists returns true if the cluster exists
func (c Clients) ClusterExists(cfg Config) (result bool, err error) {
	describeClusterInput := ecs.DescribeClustersInput{
		Clusters: aws.StringSlice([]string{cfg.Options.ClusterName}),
	}
	describeCluster, err := c.ECS.DescribeClusters(&describeClusterInput)
	if err != nil {
		return result, err
	}

	if len(describeCluster.Clusters) > 0 {
		if aws.StringValue(describeCluster.Clusters[0].Status) == "ACTIVE" {
			return true, err
		}
	}

	return result, err
}

// CreateCluster creates a new cluster
func (c Clients) CreateCluster(cfg Config, timeout time.Duration) (err error) {
	Log.Infof("Creating cluster %s", cfg.Options.ClusterName)

	createClusterInput := ecs.CreateClusterInput{
		ClusterName: aws.String(cfg.Options.ClusterName),
	}
	_, err = c.ECS.CreateCluster(&createClusterInput)
	if err != nil {
		return err
	}

	clusterCreated := false
	for count := 0; count < 30; count++ {
		Log.Infof("Waiting for cluster to provision")
		time.Sleep(timeout * time.Second)

		describeClusterInput := ecs.DescribeClustersInput{
			Clusters: aws.StringSlice([]string{cfg.Options.ClusterName}),
		}
		describeCluster, err := c.ECS.DescribeClusters(&describeClusterInput)
		if err != nil {
			return err
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
		return fmt.Errorf("failed to create cluster")
	}

	// Wait 5 seconds for luck
	time.Sleep(timeout * time.Second)

	return err
}

// DeleteCluster deletes a cluster
func (c Clients) DeleteCluster(cfg Config) (err error) {
	Log.Infof("Deleting cluster %s", cfg.Options.ClusterName)

	deleteClusterInput := ecs.DeleteClusterInput{
		Cluster: aws.String(cfg.Options.ClusterName),
	}
	_, err = c.ECS.DeleteCluster(&deleteClusterInput)
	if err != nil {
		return err
	}

	Log.Infof("Cluster %s deleted", cfg.Options.ClusterName)

	return err
}
