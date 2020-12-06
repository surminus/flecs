package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// TaskStep runs a one-off task in the cluster, with a given command, task definition
// and whatever other implementation details ECS requires.
type TaskStep struct {
	Command    string `yaml:"command"`
	Container  string `yaml:"container"`
	Definition string `yaml:"definition"`
	LaunchType string `yaml:"launch_type"`
	TaskName   string `yaml:"task_name"`
}

// Run performs the task step
func (t TaskStep) Run(c Client, cfg Config) (taskArn string, err error) {
	// Set up clients
	clients, err := c.InitClients()
	if err != nil {
		return taskArn, err
	}

	if t.TaskName == "" && t.Command == "" {
		return taskArn, fmt.Errorf("must specify either one of command or task_name")
	}

	if t.Definition == "" {
		return taskArn, fmt.Errorf("must specify task definition to use")
	}

	if t.LaunchType == "" {
		t.LaunchType = "FARGATE"
	}

	networkConfiguration, err := clients.NetworkConfiguration(cfg)
	if err != nil {
		return taskArn, err
	}

	// Register task definition
	definition, ok := cfg.Definitions[t.Definition]
	if !ok {
		return taskArn, fmt.Errorf("cannot find task definition called %s", t.Definition)
	}

	if len(definition.Containers) > 1 && t.Container == "" {
		return taskArn, fmt.Errorf("must specify container if more than one container in task definition")
	}

	taskName := fmt.Sprintf("%s-%s", "flecs", cfg.ProjectName)
	if t.TaskName != "" && t.TaskName != cfg.ProjectName {
		taskName = fmt.Sprintf("%s-%s", taskName, t.TaskName)
	} else {
		name := strings.Split(t.Command, " ")[0]
		taskName = fmt.Sprintf("%s-%s", taskName, name)
	}

	taskDefinitionArn, err := definition.Create(clients, cfg, taskName)
	if err != nil {
		return taskArn, err
	}
	Log.Infof("Registered task definition %s", taskDefinitionArn)

	runTaskInput := ecs.RunTaskInput{
		Cluster:              aws.String(cfg.Options.ClusterName),
		LaunchType:           aws.String(t.LaunchType),
		NetworkConfiguration: &networkConfiguration,
		TaskDefinition:       aws.String(taskDefinitionArn),
	}

	containerName := t.Container
	if containerName == "" {
		containerName = definition.Containers[0].Name
	}

	if t.Command != "" {
		command := strings.Split(t.Command, " ")

		overrides := ecs.TaskOverride{
			ContainerOverrides: []*ecs.ContainerOverride{
				&ecs.ContainerOverride{
					Command: aws.StringSlice(command),
					Name:    aws.String(containerName),
				},
			},
		}

		runTaskInput.SetOverrides(&overrides)
	}

	resp, err := clients.ECS.RunTask(&runTaskInput)
	if err != nil {
		return taskArn, err
	}

	err = t.checkFailures(resp.Failures)
	if err != nil {
		return taskArn, err
	}

	taskArn = aws.StringValue(resp.Tasks[0].TaskArn)

	Log.Infof("Waiting for task to finish: %s", taskArn)
	describeTasksInput := ecs.DescribeTasksInput{
		Cluster: aws.String(cfg.Options.ClusterName),
		Tasks:   aws.StringSlice([]string{taskArn}),
	}
	err = clients.ECS.WaitUntilTasksStopped(&describeTasksInput)
	if err != nil {
		return taskArn, err
	}

	describeTasksOutput, err := clients.ECS.DescribeTasks(&describeTasksInput)
	if err != nil {
		return taskArn, err
	}

	err = t.checkFailures(describeTasksOutput.Failures)
	if err != nil {
		return taskArn, err
	}

	task := describeTasksOutput.Tasks[0]

	for _, container := range task.Containers {
		if aws.Int64Value(container.ExitCode) != 0 {
			if aws.StringValue(container.Reason) != "" {
				return taskArn, fmt.Errorf("container %s failed: %s", aws.StringValue(container.Name), aws.StringValue(container.Reason))
			}

			return taskArn, fmt.Errorf("container %s failed", aws.StringValue(container.Name))
		}
	}

	logs := make(map[string][]string)
	taskID := t.taskIDfromARN(taskArn)

	for _, container := range task.Containers {
		logStreamName := fmt.Sprintf("%s/%s/%s", taskName, aws.StringValue(container.Name), taskID)
		_, err := t.waitForLogStream(clients, cfg, logStreamName)
		if err != nil {
			return taskArn, err
		}

		getLogEventsInput := cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(cfg.Options.LogGroupName),
			LogStreamName: aws.String(logStreamName),
		}
		events, err := clients.CloudWatchLogs.GetLogEvents(&getLogEventsInput)
		if err != nil {
			return taskArn, err
		}

		var logEvents []string
		for _, event := range events.Events {
			logEvents = append(logEvents, aws.StringValue(event.Message))

		}

		logs[aws.StringValue(container.Name)] = logEvents
	}

	for name, events := range logs {
		for _, e := range events {
			fmt.Printf("[%s]\t%s\n", name, e)
		}
	}

	return taskArn, err
}

func (t TaskStep) checkFailures(failures []*ecs.Failure) (err error) {
	if len(failures) > 0 {
		formattedFailures := ""
		for _, f := range failures {
			formattedFailures += fmt.Sprintf("==> %s / %s / %s <==", *f.Arn, *f.Detail, *f.Reason)
		}

		return fmt.Errorf(formattedFailures)
	}

	return err
}

func (t TaskStep) taskIDfromARN(taskArn string) (taskID string) {
	arnSplit := strings.Split(taskArn, "/")
	taskID = arnSplit[len(arnSplit)-1]
	return taskID
}

func (t TaskStep) waitForLogStream(c Clients, cfg Config, logStreamName string) (logStream cloudwatchlogs.LogStream, err error) {
	for count := 0; count < 30; count++ {
		resp, err := c.CloudWatchLogs.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName:        aws.String(cfg.Options.LogGroupName),
			LogStreamNamePrefix: aws.String(logStreamName),
		})
		if err != nil {
			return logStream, err
		}

		if len(resp.LogStreams) > 0 {
			logStream = *resp.LogStreams[0]
			return logStream, nil
		}

		Log.Infof("Waiting for log stream %s", logStreamName)
		time.Sleep(5 * time.Second)
	}

	return logStream, fmt.Errorf("Timed out waiting for log stream: %s/%s", cfg.Options.LogGroupName, logStreamName)
}
