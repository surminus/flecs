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
	ExecutionRoleName    string              `yaml:"execution_role_name"`
	Tag                  string              `yaml:"tag"`
	TaskRoleName         string              `yaml:"task_role_name"`
	VolumeName           string              `yaml:"volume_name"`
	Containers           []Container         `yaml:"containers"`
	CPU                  int                 `yaml:"cpu"`
	Memory               int                 `yaml:"memory"`
	PlacementConstraints []map[string]string `yaml:"placement_constraints"`
}

// Container sets up a container definition
type Container struct {
	Command     string       `yaml:"command"`
	DynamicTag  bool         `yaml:"dynamic_tag"`
	Essential   bool         `yaml:"essential"`
	HealthCheck HealthCheck  `yaml:"healthcheck"`
	Image       string       `yaml:"image"`
	MountPoints []MountPoint `yaml:"mount_points"`
	Name        string       `yaml:"name"`
	VolumesFrom []VolumeFrom `yaml:"volumes_from"`
}

// HealthCheck is used to check the health of a container
type HealthCheck struct {
	Command     string `yaml:"command"`
	Interval    int64  `yaml:"interval"`
	Retries     int64  `yaml:"retries"`
	StartPeriod int64  `yaml:"start_period"`
	Timeout     int64  `yaml:"timeout"`
}

// MountPoint allows setting up a mount point on a container
type MountPoint struct {
	ContainerPath string `yaml:"container_path"`
	ReadOnly      bool   `yaml:"read_only"`
	SourceVolume  string `yaml:"source_volume"`
}

// VolumeFrom allows sharing a volume with another container
type VolumeFrom struct {
	ReadOnly        bool   `yaml:"read_only"`
	SourceContainer string `yaml:"source_container"`
}

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
	containerDefinitions, err := d.generateContainerDefinitions(cfg, name)
	if err != nil {
		return arn, err
	}

	// Placement constraints
	var placementConstraints []*ecs.TaskDefinitionPlacementConstraint
	for _, constraint := range d.PlacementConstraints {
		for key, value := range constraint {
			placementConstraints = append(placementConstraints, &ecs.TaskDefinitionPlacementConstraint{
				Expression: aws.String(key),
				Type:       aws.String(value),
			})
		}
	}

	// Volumes
	volumes := []*ecs.Volume{
		&ecs.Volume{Name: aws.String(d.VolumeName)},
	}

	cpu := strconv.Itoa(d.CPU)
	memory := strconv.Itoa(d.Memory)

	registerTaskDefinitionInput := ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: containerDefinitions,
		Cpu:                  aws.String(cpu),
		Memory:               aws.String(memory),
		ExecutionRoleArn:     aws.String(executionRoleArn),
		Family:               aws.String(family),
		NetworkMode:          aws.String("awsvpc"),
		PlacementConstraints: placementConstraints,
		TaskRoleArn:          aws.String(taskRoleArn),
	}

	if d.VolumeName != "" {
		registerTaskDefinitionInput.SetVolumes(volumes)
	}

	output, err := client.RegisterTaskDefinition(&registerTaskDefinitionInput)
	if err != nil {
		return arn, err
	}

	arn = aws.StringValue(output.TaskDefinition.TaskDefinitionArn)

	return arn, err
}

func (d Definition) generateContainerDefinitions(cfg Config, logStreamPrefix string) (def []*ecs.ContainerDefinition, err error) {
	// Secrets
	var secrets []*ecs.Secret
	for name, valueFrom := range cfg.Secrets {
		secrets = append(secrets, &ecs.Secret{
			Name:      aws.String(name),
			ValueFrom: aws.String(valueFrom),
		})
	}

	// Environment variables
	var environmentVariables []*ecs.KeyValuePair
	for name, value := range cfg.EnvironmentVariables {
		environmentVariables = append(environmentVariables, &ecs.KeyValuePair{
			Name:  aws.String(name),
			Value: aws.String(value),
		})
	}

	// Log configuration
	logConfiguration := ecs.LogConfiguration{
		LogDriver: aws.String("awslogs"),
		Options: aws.StringMap(map[string]string{
			"awslogs-region":        cfg.Region,
			"awslogs-stream-prefix": logStreamPrefix,
			"awslogs-group":         cfg.LogGroupName,
		}),
	}

	for _, container := range d.Containers {
		command := strings.Split(container.Command, " ")

		healthcheck := ecs.HealthCheck{
			Command:     aws.StringSlice(strings.Split(container.HealthCheck.Command, " ")),
			Interval:    aws.Int64(container.HealthCheck.Interval),
			Retries:     aws.Int64(container.HealthCheck.Retries),
			StartPeriod: aws.Int64(container.HealthCheck.StartPeriod),
			Timeout:     aws.Int64(container.HealthCheck.Timeout),
		}

		essential := false
		if len(d.Containers) == 1 {
			essential = true
		} else {
			essential = container.Essential
		}

		var mountPoints []*ecs.MountPoint
		for _, mount := range container.MountPoints {
			mountPoints = append(mountPoints, &ecs.MountPoint{
				ContainerPath: aws.String(mount.ContainerPath),
				ReadOnly:      aws.Bool(mount.ReadOnly),
				SourceVolume:  aws.String(mount.SourceVolume),
			})
		}

		var volumesFrom []*ecs.VolumeFrom
		for _, volume := range container.VolumesFrom {
			volumesFrom = append(volumesFrom, &ecs.VolumeFrom{
				ReadOnly:        aws.Bool(volume.ReadOnly),
				SourceContainer: aws.String(volume.SourceContainer),
			})
		}

		var image string
		if container.DynamicTag {
			imgSplit := strings.Split(image, ":")
			if len(imgSplit) > 1 {
				return def, fmt.Errorf("do not specify a tag using \":\" when dynamic tag is specifed")
			}

			image = strings.Join([]string{image, cfg.Tag}, ":")
		}

		containerDefinition := ecs.ContainerDefinition{
			Command:          aws.StringSlice(command),
			Environment:      environmentVariables,
			Essential:        aws.Bool(essential),
			Image:            aws.String(container.Image),
			LogConfiguration: &logConfiguration,
			Name:             aws.String(container.Name),
			Secrets:          secrets,
			HealthCheck:      &healthcheck,
			MountPoints:      mountPoints,
			VolumesFrom:      volumesFrom,
		}

		def = append(def, &containerDefinition)
	}

	return def, err
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
		PolicyArn: aws.String("aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  createRoleOutput.Role.RoleName,
	})
	if err != nil {
		return roleArn, err
	}

	Log.Infof("Created role %s", defaultExecutionRoleName)

	return aws.StringValue(createRoleOutput.Role.Arn), err
}
