package main

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
)

// This file contains all the interfaces we want to stub using the AWS
// interfaces. Any API call that we make to AWS should always get included
// in this file to allow easy testing against all interfaces

// ECS
type mockedDescribeClusters struct {
	ecsiface.ECSAPI
	Resp ecs.DescribeClustersOutput
}

func (m mockedDescribeClusters) DescribeClusters(in *ecs.DescribeClustersInput) (*ecs.DescribeClustersOutput, error) {
	return &m.Resp, nil
}
