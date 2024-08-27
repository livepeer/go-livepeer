package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRemoteAIResults = &core.RemoteAIWorkerResult{
	Results: []core.AIResult{},
	Files:   map[string][]byte{},
	Err:     nil,
}

func TestAIWorkerResults_ErrorsWhenAuthHeaderMissing(t *testing.T) {
	var l lphttp

	var w = httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/AIResults", nil)
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

	r, err := http.NewRequest(http.MethodGet, "/AIResults", nil)
	require.NoError(t, err)

	r.Header.Set("Authorization", protoVerAIWorker)
	r.Header.Set("Credentials", "BAD CREDENTIALS")

	code, body := aiResultsTest(l, w, r)
	require.Equal(t, http.StatusUnauthorized, code)
	require.Contains(t, body, "Unauthorized")
}

func TestAIWorkerResults_ErrorsWhenContentTypeMissing(t *testing.T) {
	var l lphttp
	l.orchestrator = newStubOrchestrator()
	l.orchestrator.TranscoderSecret()
	var w = httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/AIResults", nil)
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

	r, err := http.NewRequest(http.MethodGet, "/AIResults", nil)
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
	var req worker.ImageToImageMultipartRequestBody
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

func TestRemoteAIWorker_Error(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	//test request
	var req worker.TextToImageJSONRequestBody
	modelID := "livepeer/model1"
	req.Prompt = "test prompt"
	req.ModelId = &modelID

	assert := assert.New(t)
	assert.Nil(nil)
	var resultRead int
	resultData := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.NoError(err)
		w.Write([]byte("result binary data"))
		resultRead++
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
	assert.NotNil(string(body))

	//error in worker, good request
	errText := "Some error"
	wkr.err = fmt.Errorf(errText)

	reqJson, _ := json.Marshal(req)
	notify.RequestData = reqJson
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal(errText, string(body))

	//pipeline not compatible
	wkr.err = nil
	reqJson, _ = json.Marshal(req)
	notify.Pipeline = "test-no-pipeline"
	notify.TaskId = 743
	notify.RequestData = reqJson

	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("743", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal("no workers can process job requested", string(body))

	// unrecoverable error
	// send the response and panic
	notify.Pipeline = "text-to-image"
	wkr.err = core.NewUnrecoverableError(errors.New("some error"))
	panicked := false
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("some error", string(body))
	assert.True(panicked)
}

func aiResultsTest(l lphttp, w *httptest.ResponseRecorder, r *http.Request) (int, string) {
	handler := l.AIResults()
	handler.ServeHTTP(w, r)
	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(body)
}

func newMockAIOrchestratorServer() *httptest.Server {
	n, _ := core.NewLivepeerNode(&eth.StubClient{}, "./tmp", nil)
	n.NodeType = core.OrchestratorNode
	n.AIWorkerManager = core.NewRemoteAIWorkerManager()
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	return srv
}

func connectWorker(n *core.LivepeerNode) {
	strm := &StubAIWorkerServer{}
	caps := createStubAIWorkerCapabilities()
	go func() { n.AIWorkerManager.Manage(strm, caps.ToNetCapabilities()) }()
	time.Sleep(1 * time.Millisecond)
}

func createStubAIWorkerCapabilities() *core.Capabilities {
	//create capabilities and constraints the ai worker sends to orch
	constraints := make(core.PerCapabilityConstraints)
	constraints[core.Capability_TextToImage] = &core.CapabilityConstraints{Models: make(core.ModelConstraints)}
	constraints[core.Capability_TextToImage].Models["livepeer/model1"] = &core.ModelConstraint{Warm: true, Capacity: 2}
	caps := core.NewCapabilities(core.DefaultCapabilities(), core.MandatoryOCapabilities())
	caps.SetPerCapabilityConstraints(constraints)

	return caps
}

type StubAIWorkerServer struct {
	manager      *core.RemoteAIWorkerManager
	SendError    error
	JobError     error
	DelayResults bool

	common.StubServerStream
}

func (s *StubAIWorkerServer) Send(n *net.NotifyAIJob) error {
	var images []worker.Media
	media := worker.Media{Nsfw: false, Seed: 111, Url: "image_url"}
	images = append(images, media)
	res := core.RemoteAIWorkerResult{
		Results: worker.ImageResponse{Images: images},
		Files:   make(map[string][]byte),
		Err:     nil,
	}
	if s.JobError != nil {
		res.Err = s.JobError
	}
	if s.SendError != nil {
		return s.SendError
	}

	return nil
}

type stubAIWorker struct {
	called int
	err    error
}

func (a stubAIWorker) TextToImage(ctx context.Context, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.ImageResponse{
			Images: []worker.Media{
				{
					Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
					Nsfw: false,
					Seed: 111,
				},
			},
		}, nil
	}

}

func (a stubAIWorker) ImageToImage(ctx context.Context, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.ImageResponse{
			Images: []worker.Media{
				{
					Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
					Nsfw: false,
					Seed: 111,
				},
			},
		}, nil
	}
}

func (a stubAIWorker) ImageToVideo(ctx context.Context, req worker.ImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.VideoResponse{
			Frames: [][]worker.Media{
				{
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 111,
					},
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 111,
					},
				},
				{
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 111,
					},
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 111,
					},
				},
			},
		}, nil
	}
}

func (a stubAIWorker) Upscale(ctx context.Context, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.ImageResponse{
			Images: []worker.Media{
				{
					Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
					Nsfw: false,
					Seed: 111,
				},
			},
		}, nil
	}
}

func (a stubAIWorker) AudioToText(ctx context.Context, req worker.AudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.TextResponse{Text: "Transcribed text"}, nil
	}
}

func (a stubAIWorker) Warm(ctx context.Context, arg1, arg2 string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	a.called++
	return nil
}

func (a stubAIWorker) Stop(ctx context.Context) error {
	a.called++
	return nil
}

func (a stubAIWorker) HasCapacity(pipeline, modelID string) bool {
	a.called++
	return true
}
