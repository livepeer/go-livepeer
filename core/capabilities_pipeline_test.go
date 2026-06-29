package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipelineLabelsFromCapabilities(t *testing.T) {
	caps := NewCapabilities(nil, nil)
	caps.SetPerCapabilityConstraints(PerCapabilityConstraints{
		Capability_LiveVideoToVideo: {
			Models: map[string]*ModelConstraint{
				"daydream": {},
			},
		},
		Capability_BYOC: {
			Models: map[string]*ModelConstraint{
				"custom-pipeline": {},
			},
		},
	})

	pipelines, modelIDs := caps.PipelineLabelsFromCapabilities(nil)
	assert.Equal(t, []string{"byoc", "live-video-to-video"}, pipelines)
	assert.Equal(t, []string{"custom-pipeline", "daydream"}, modelIDs)

	pipelines, modelIDs = caps.PipelineLabelsFromCapabilities([]string{"lv2v"})
	assert.Equal(t, []string{"live-video-to-video"}, pipelines)
	assert.Equal(t, []string{"daydream"}, modelIDs)

	pipelines, modelIDs = caps.PipelineLabelsFromCapabilities([]string{"byoc", "live-video-to-video"})
	assert.Equal(t, []string{"byoc", "live-video-to-video"}, pipelines)
	assert.Equal(t, []string{"custom-pipeline", "daydream"}, modelIDs)
}
