package server

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIWorkerResults_ErrorsWhenAuthHeaderMissing(t *testing.T) {
	var l lphttp

	var w = httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "Unauthorized")
}

func TestAIWorkerResults_ErrorsWhenCredentialsInvalid(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "BAD CREDENTIALS")

	code, body := aiResultsTest(l, w, r)
	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "invalid secret")
}

func TestAIWorkerResults_ErrorsWhenContentTypeMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusUnsupportedMediaType, code)
	require.Contains(t, body, "mime: no media type")
}

func TestAIWorkerResults_ErrorsWhenTaskIDMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodPost, "/aiResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "")
	r.Header.Set("Content-Type", "application/json")

	code, body := aiResultsTest(l, w, r)

	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "Invalid Task ID")
}

func TestAIWorkerResults_BadRequestType(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	//test request
	var req worker.GenImageToImageMultipartRequestBody
	modelID := "livepeer/model1"
	req.Prompt = "test prompt"
	req.ModelId = &modelID

	assert := assert.New(t)
	assert.Nil(nil)
	resultData := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("result binary data"))
	}))
	defer resultData.Close()
	notify := &net.NotifyAIJob{
		TaskId:      742,
		Pipeline:    "text-to-image",
		ModelID:     "livepeer/model1",
		Url:         "",
		RequestData: nil,
	}
	wkr := &stubAIWorker{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.AIWorker = wkr
	node.Capabilities = createStubAIWorkerCapabilities()

	var headers http.Header
	var body []byte
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := io.ReadAll(r.Body)
		assert.NoError(err)
		headers = r.Header
		body = out
		w.Write(nil)
	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)
	//send empty request data
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.Equal(0, wkr.called)
	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal("AI request not correct", string(body)[0:22])
}
