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

	expected = Config{
		Options:     defaultConfigOptions,
		ProjectName: "default",
		Pipeline: []Step{
			Step{
				Script: ScriptStep{
					Inline: "test",
				},
				Type: "script",
			},
		},
	}

	assert.Equal(t, expected, actual)
}
