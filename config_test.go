package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	expected   Config
	actual     Config
	yamlConfig string
)

var defaultConfigOptions = ConfigOptions{
	ClusterName:  "flecs",
	ECRRegion:    "eu-west-1",
	LogGroupName: "/flecs/default",
	Region:       "eu-west-1",
}

func TestLoadConfigBasicError(t *testing.T) {
	// Should fail if pipeline configuration is not found
	_, err := LoadConfig(yamlConfig, "", "", "", false)
	assert.NotNil(t, err)
}

func TestLoadConfigBasicPipeline(t *testing.T) {
	yamlConfig = `---
pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.Nil(t, err)

	options := defaultConfigOptions
	options.Pipeline = []Step{
		Step{
			Script: ScriptStep{
				Inline: "test",
			},
			Type: "script",
		},
	}

	expected = Config{
		Options:     options,
		ProjectName: "default",
	}

	assert.Equal(t, expected, actual)
}

func TestLoadConfigEnvironments(t *testing.T) {
	yamlConfig = `---
region: us-east-1

environments:
  test:
    region: eu-west-2
    cluster_name: test-cluster

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "", "", false)
	assert.Nil(t, err)

	options := defaultConfigOptions
	options.Pipeline = []Step{
		Step{
			Script: ScriptStep{
				Inline: "test",
			},
			Type: "script",
		},
	}

	options.Region = "eu-west-2"
	options.ECRRegion = options.Region
	options.ClusterName = "test-cluster"

	expected = Config{
		Options: options,
		Environments: map[string]ConfigOptions{
			"test": ConfigOptions{
				Region:      "eu-west-2",
				ClusterName: "test-cluster",
			},
		},
		EnvironmentName: "test",
		ProjectName:     "default",
	}

	assert.Equal(t, expected, actual)
}

func TestLoadConfigEnvironmentNotExist(t *testing.T) {
	yamlConfig = `---
region: us-east-1

environments:
  test:
    region: eu-west-2

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "production", "", "", false)
	assert.Nil(t, err)

	assert.Equal(t, "us-east-1", actual.Options.Region)
}

func TestLoadConfigFullPipeline(t *testing.T) {
	yamlConfig = `---
pipeline:
  - type: script
    name: Inline test
    inline: test
  - type: script
    name: Path test
    path: /foo/bar
  - type: docker
    name: Docker test
    dockerfile: Dockerfile
    repository: test/repo
  - type: service
    name: Service test
    service: test
  - type: task
    name: Task test
    task: test
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.Nil(t, err)

	options := defaultConfigOptions
	options.Pipeline = []Step{
		Step{
			Script: ScriptStep{
				Inline: "test",
			},
			Type: "script",
			Name: "Inline test",
		},
		Step{
			Script: ScriptStep{
				Path: "/foo/bar",
			},
			Type: "script",
			Name: "Path test",
		},
		Step{
			Docker: DockerStep{
				Dockerfile: "Dockerfile",
				Repository: "test/repo",
			},
			Type: "docker",
			Name: "Docker test",
		},
		Step{
			Service: ServiceStep{
				Service: "test",
			},
			Type: "service",
			Name: "Service test",
		},
		Step{
			Task: TaskStep{
				Task: "test",
			},
			Type: "task",
			Name: "Task test",
		},
	}

	expected = Config{
		Options:     options,
		ProjectName: "default",
	}

	assert.Equal(t, expected, actual)
}

func TestLoadConfigPipelineError(t *testing.T) {
	yamlConfig = `---
pipeline:
  - type: foo
    name: Not a real step
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.NotNil(t, err)

	yamlConfig = `---
pipeline:
  - name: Missing step type
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.NotNil(t, err)
}

func TestLoadConfigProjectName(t *testing.T) {
	yamlConfig = `---
pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "", "", "my-project", false)
	assert.Nil(t, err)

	assert.Equal(t, "my-project", actual.ProjectName)
}

func TestLoadConfigTag(t *testing.T) {
	yamlConfig = `---
pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "", "some-tag", "", false)
	assert.Nil(t, err)

	assert.Equal(t, "some-tag", actual.Tag)
}

func TestLoadConfigMergeEnvVars(t *testing.T) {
	yamlConfig = `---
environment_variables:
  yellow: banana

environments:
  test:
    environment_variables:
      red: grapes

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "", "", false)
	assert.Nil(t, err)

	expected := map[string]string{
		"yellow": "banana",
		"red":    "grapes",
	}

	assert.Equal(t, expected, actual.Options.EnvironmentVariables)
}

func TestLoadConfigMergeSecrets(t *testing.T) {
	yamlConfig = `---
secrets:
  yellow: banana

environments:
  test:
    secrets:
      red: grapes

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "", "", false)
	assert.Nil(t, err)

	expected := map[string]string{
		"yellow": "banana",
		"red":    "grapes",
	}

	assert.Equal(t, expected, actual.Options.Secrets)
}

func TestLoadConfigExpressions(t *testing.T) {
	yamlConfig = `---
cluster_name: {{ environment }}-cluster

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "some-tag", "my-project", false)
	assert.Nil(t, err)

	assert.Equal(t, "test-cluster", actual.Options.ClusterName)

	yamlConfig = `---
cluster_name: {{ project_name }}-cluster

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "some-tag", "my-project", false)
	assert.Nil(t, err)

	assert.Equal(t, "my-project-cluster", actual.Options.ClusterName)

	yamlConfig = `---
cluster_name: {{project_name}}-cluster
log_group_name: {{ environment }}/{{ tag }}

pipeline:
  - type: script
    inline: test
`

	actual, err = LoadConfig(yamlConfig, "test", "some-tag", "my-project", false)
	assert.Nil(t, err)

	assert.Equal(t, "my-project-cluster", actual.Options.ClusterName)
	assert.Equal(t, "test/some-tag", actual.Options.LogGroupName)
}
