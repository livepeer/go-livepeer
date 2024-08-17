package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipelineToCapability(t *testing.T) {
	good := "audio-to-text"
	bad := "text-to-text"

	cap, err := PipelineToCapability(good)
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_AudioToText)

	cap, err = PipelineToCapability(bad)
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)
}
