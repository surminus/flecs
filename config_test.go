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
  - script:
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
region: eu-west-2

environments:
  test:
    region: eu-west-2

pipeline:
  - script:
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

	expected = Config{
		Options: options,
		Environments: map[string]ConfigOptions{
			"test": ConfigOptions{
				Region: "eu-west-2",
			},
		},
		EnvironmentName: "test",
		ProjectName:     "default",
	}

	assert.Equal(t, expected, actual)
}

func TestLoadConfigFullPipeline(t *testing.T) {
	yamlConfig = `---
pipeline:
  - script:
      name: Inline test
      inline: test
  - script:
      name: Path test
      path: /foo/bar
  - docker:
      name: Docker test
      dockerfile: Dockerfile
      repository: test/repo
  - service:
      name: Service test
      service: test
  - task:
      name: Task test
      command: uptime
      container: alpine
      definition: test
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.Nil(t, err)

	options := defaultConfigOptions
	options.Pipeline = []Step{
		Step{
			Script: ScriptStep{
				Name:   "Inline test",
				Inline: "test",
			},
			Type: "script",
		},
		Step{
			Script: ScriptStep{
				Name: "Path test",
				Path: "/foo/bar",
			},
			Type: "script",
		},
		Step{
			Docker: DockerStep{
				Name:       "Docker test",
				Dockerfile: "Dockerfile",
				Repository: "test/repo",
			},
			Type: "docker",
		},
		Step{
			Service: ServiceStep{
				Name:    "Service test",
				Service: "test",
			},
			Type: "service",
		},
		Step{
			Task: TaskStep{
				Name:       "Task test",
				Command:    "uptime",
				Container:  "alpine",
				Definition: "test",
			},
			Type: "task",
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
  - foo:
      name: Not a real step
`

	actual, err = LoadConfig(yamlConfig, "", "", "", false)
	assert.NotNil(t, err)
}