package main

// Task runs a one-off task in the cluster, with a given command, task definition
// and whatever other implementation details ECS requires.
//
// Optionally, will query Cloudwatch Logs for any output if the `awslogs` log type
// is set up a log group name is provided
type TaskStep struct {
	Command        string `yaml:"command"`
	Description    string `yaml:"description"`
	Name           string `yaml:"name"`
	TaskDefinition string `yaml:"definition"`
}
