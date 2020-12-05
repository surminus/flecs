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
