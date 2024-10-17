package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipelineToCapability(t *testing.T) {
	good := "audio-to-text"
	bad := "i-love-tests"

	cap, err := PipelineToCapability(good)
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_AudioToText)

	cap, err = PipelineToCapability(bad)
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)
}

func TestModelsWebhook(t *testing.T) {
	assert := assert.New(t)

	mockResponses := []string{
		`[{"name":"Model1", "model_id":"model1", "pipeline":"text-to-image", "warm":true}]`,
		`[{"name":"Model2", "model_id":"model2", "pipeline":"image-to-image", "warm":false}]`,
	}
	responseIndex := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponses[responseIndex]))
		responseIndex = (responseIndex + 1) % len(mockResponses)
	}))
	defer mockServer.Close()

	refreshInterval := 100 * time.Millisecond

	webhook, err := NewAIModelWebhook(mockServer.URL, refreshInterval)
	assert.NoError(err)
	assert.NotNil(webhook)
	defer webhook.Stop()

	configs := webhook.GetConfigs()
	assert.Len(configs, 1)
	assert.Equal("model1", configs[0].ModelID)
	assert.Equal("text-to-image", configs[0].Pipeline)
	assert.True(configs[0].Warm)

	time.Sleep(refreshInterval * 2)

	// Check if models are updated
	configs = webhook.GetConfigs()
	assert.Len(configs, 1)
	assert.Equal("model2", configs[0].ModelID)
	assert.Equal("image-to-image", configs[0].Pipeline)
	assert.False(configs[0].Warm)

	_, err = NewAIModelWebhook("http://invalid-url", refreshInterval)
	assert.Error(err)

	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer invalidServer.Close()

	invalidWebhook, err := NewAIModelWebhook(invalidServer.URL, refreshInterval)
	assert.Error(err)
	assert.Nil(invalidWebhook)
}

func TestModelsFile(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "aimodels*.json")
	assert.NoError(err)
	defer os.Remove(tmpfile.Name())

	initialContent := `[{"name":"Model1", "model_id":"model1", "pipeline":"text-to-image", "warm":true}]`
	_, err = tmpfile.Write([]byte(initialContent))
	assert.NoError(err)
	tmpfile.Close()

	refreshInterval := 100 * time.Millisecond

	webhook, err := NewAIModelWebhook("file://"+tmpfile.Name(), refreshInterval)
	assert.NoError(err)
	assert.NotNil(webhook)
	defer webhook.Stop()

	configs := webhook.GetConfigs()
	assert.Len(configs, 1)
	assert.Equal("model1", configs[0].ModelID)
	assert.Equal("text-to-image", configs[0].Pipeline)
	assert.True(configs[0].Warm)

	time.Sleep(refreshInterval / 2)
	updatedContent := `[{"name":"Model2", "model_id":"model2", "pipeline":"image-to-image", "warm":false}]`
	err = os.WriteFile(tmpfile.Name(), []byte(updatedContent), 0644)
	assert.NoError(err)

	time.Sleep(refreshInterval * 2)

	// Check if models are updated
	configs = webhook.GetConfigs()
	assert.Len(configs, 1)
	assert.Equal("model2", configs[0].ModelID)
	assert.Equal("image-to-image", configs[0].Pipeline)
	assert.False(configs[0].Warm)

	_, err = NewAIModelWebhook("file:///nonexistent/path", refreshInterval)
	assert.Error(err)

	invalidFile, err := os.CreateTemp("", "invalid*.json")
	assert.NoError(err)
	defer os.Remove(invalidFile.Name())
	_, err = invalidFile.Write([]byte(`invalid json`))
	assert.NoError(err)
	invalidFile.Close()

	invalidWebhook, err := NewAIModelWebhook("file://"+invalidFile.Name(), refreshInterval)
	assert.Error(err)
	assert.Nil(invalidWebhook)
}
