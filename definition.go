package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
)

// Definition will be used to configure task definitions
type Definition struct {
	ExecutionRoleName    string                `yaml:"execution_role_name"`
	Tag                  string                `yaml:"tag"`
	TaskRoleName         string                `yaml:"task_role_name"`
	VolumeName           string                `yaml:"volume_name"`
	Containers           []Container           `yaml:"containers"`
	CPU                  int                   `yaml:"cpu"`
	Memory               int                   `yaml:"memory"`
	PlacementConstraints []PlacementConstraint `yaml:"placement_constraints"`
}

// Container sets up a container definition
type Container struct {
	Image string `yaml:"image"`
	Name  string `yaml:"name"`
}

// PlacementConstraint sets up a placement constraint
type PlacementConstraint struct{}

// Create registers a new task definition, and creates an execution role if one is not
// supplied
func (d Definition) Create(c Client, cfg Config, name string) (arn string, err error) {
	// Set up ECS client
	clientECS, err := c.ECS()
	if err != nil {
		return arn, err
	}
	client := clientECS.Client

	// Set up STS client
	clientSTS, err := c.STS()
	if err != nil {
		return arn, err
	}

	// Fetch current account ID
	getCallerIdentityOutput, err := clientSTS.Client.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return arn, err
	}

	accountID := aws.StringValue(getCallerIdentityOutput.Account)

	// Generate execution role arn
	var executionRoleArn string
	if d.ExecutionRoleName != "" {
		executionRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, d.ExecutionRoleName)
	}

	if d.ExecutionRoleName == "" {
		executionRoleArn, err = d.createDefaultExecutionRole(c)
		if err != nil {
			return arn, err
		}
	}

	// Generate task role arn
	var taskRoleArn string
	if d.ExecutionRoleName != "" {
		taskRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, d.TaskRoleName)
	}

	// Generate family name
	family := strings.Join([]string{"flecs", name}, "-")
	if cfg.EnvironmentName != "" {
		family = strings.Join([]string{family, cfg.EnvironmentName}, "-")
	}

	// Configure container definitions

	cpu := strconv.Itoa(d.CPU)
	memory := strconv.Itoa(d.Memory)

	registerTaskDefinitionInput := ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: []*ecs.ContainerDefinition{},
		Cpu:                  aws.String(cpu),
		Memory:               aws.String(memory),
		ExecutionRoleArn:     aws.String(executionRoleArn),
		Family:               aws.String(family),
		NetworkMode:          aws.String("awsvpc"),
		PlacementConstraints: []*ecs.TaskDefinitionPlacementConstraint{},
		TaskRoleArn:          aws.String(taskRoleArn),
		Volumes: []*ecs.Volume{
			&ecs.Volume{Name: aws.String(d.VolumeName)},
		},
	}

	output, err := client.RegisterTaskDefinition(&registerTaskDefinitionInput)
	if err != nil {
		return arn, err
	}

	arn = aws.StringValue(output.TaskDefinition.TaskDefinitionArn)

	return arn, err
}

func (d Definition) createDefaultExecutionRole(c Client) (roleArn string, err error) {
	clientIAM, err := c.IAM()
	if err != nil {
		return roleArn, err
	}

	defaultExecutionRoleName := "FlecsDefaultExecutionRole"

	getRoleOutput, err := clientIAM.Client.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(defaultExecutionRoleName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case iam.ErrCodeNoSuchEntityException:
				Log.Info("Creating default execution role")
			default:
				return roleArn, aerr
			}
		} else {
			return roleArn, aerr
		}
	} else {
		return aws.StringValue(getRoleOutput.Role.Arn), err
	}

	assumeRolePolicy := `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

	createRoleOutput, err := clientIAM.Client.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String("FlecsDefaultExecutionRole"),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
	})
	if err != nil {
		return roleArn, err
	}

	_, err = clientIAM.Client.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String("roleArn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  createRoleOutput.Role.RoleName,
	})
	if err != nil {
		return roleArn, err
	}

	Log.Infof("Created role %s", defaultExecutionRoleName)

	return aws.StringValue(createRoleOutput.Role.Arn), err
}
