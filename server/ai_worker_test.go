package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/stretchr/testify/assert"
)

func TestRemoteAIWorker_Error(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	//test request
	var req worker.GenTextToImageJSONRequestBody
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

func TestRunAIJob(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image.png" {
			data, err := os.ReadFile("../test/ai/image")
			if err != nil {
				t.Fatalf("failed to read test image: %v", err)
			}
			imgData, err := base64.StdEncoding.DecodeString(string(data))
			if err != nil {
				t.Fatalf("failed to decode base64 test image: %v", err)
			}
			w.Write(imgData)
			return
		} else if r.URL.Path == "/audio.mp3" {
			data, err := os.ReadFile("../test/ai/audio")
			if err != nil {
				t.Fatalf("failed to read test audio: %v", err)
			}
			imgData, err := base64.StdEncoding.DecodeString(string(data))
			if err != nil {
				t.Fatalf("failed to decode base64 test audio: %v", err)
			}
			w.Write(imgData)
			return
		}

	}))
	defer ts.Close()
	parsedURL, _ := url.Parse(ts.URL)

	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	assert := assert.New(t)

	tests := []struct {
		name            string
		notify          *net.NotifyAIJob
		expectedErr     string
		expectedOutputs int
	}{
		{
			name: "TextToImage_Success",
			notify: &net.NotifyAIJob{
				TaskId:      1,
				Pipeline:    "text-to-image",
				ModelID:     "livepeer/model1",
				Url:         "",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name: "ImageToImage_Success",
			notify: &net.NotifyAIJob{
				TaskId:      2,
				Pipeline:    "image-to-image",
				ModelID:     "livepeer/model1",
				Url:         parsedURL.String() + "/image.png",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name: "Upscale_Success",
			notify: &net.NotifyAIJob{
				TaskId:      3,
				Pipeline:    "upscale",
				ModelID:     "livepeer/model1",
				Url:         parsedURL.String() + "/image.png",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name: "ImageToVideo_Success",
			notify: &net.NotifyAIJob{
				TaskId:      4,
				Pipeline:    "image-to-video",
				ModelID:     "livepeer/model1",
				Url:         parsedURL.String() + "/image.png",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 2,
		},
		{
			name: "AudioToText_Success",
			notify: &net.NotifyAIJob{
				TaskId:      5,
				Pipeline:    "audio-to-text",
				ModelID:     "livepeer/model1",
				Url:         parsedURL.String() + "/audio.mp3",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name: "SegmentAnything2_Success",
			notify: &net.NotifyAIJob{
				TaskId:      6,
				Pipeline:    "segment-anything-2",
				ModelID:     "livepeer/model1",
				Url:         parsedURL.String() + "/image.png",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name: "UnsupportedPipeline",
			notify: &net.NotifyAIJob{
				TaskId:      7,
				Pipeline:    "unsupported-pipeline",
				ModelID:     "livepeer/model1",
				Url:         "",
				RequestData: []byte(`{"prompt":"test prompt"}`),
			},
			expectedErr: "no workers can process job requested",
		},
		{
			name: "InvalidRequestData",
			notify: &net.NotifyAIJob{
				TaskId:      8,
				Pipeline:    "text-to-image",
				ModelID:     "livepeer/model1",
				Url:         "",
				RequestData: []byte(`invalid json`),
			},
			expectedErr: "AI request not correct for text-to-image pipeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wkr := &stubAIWorker{}
			node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)

			node.OrchSecret = "verbigsecret"
			node.AIWorker = wkr
			node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId(tt.notify.Pipeline, tt.notify.ModelID)

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
			drivers.NodeStorage = drivers.NewMemoryDriver(parsedURL)
			runAIJob(node, parsedURL.Host, httpc, tt.notify)
			time.Sleep(3 * time.Millisecond)

			//assert.Equal(tt.expectedCalled, wkr.called)
			if tt.expectedErr != "" {
				assert.NotNil(body)
				assert.Contains(string(body), tt.expectedErr)
				assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
			} else {
				assert.NotNil(body)
				assert.NotEqual(aiWorkerErrorMimeType, headers.Get("Content-Type"))
			}
		})
	}
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

func createStubAIWorkerCapabilitiesForPipelineModelId(pipeline, modelId string) *core.Capabilities {
	//create capabilities and constraints the ai worker sends to orch
	cap, err := core.PipelineToCapability(pipeline)
	if err != nil {
		return nil
	}
	constraints := make(core.PerCapabilityConstraints)
	constraints[cap] = &core.CapabilityConstraints{Models: make(core.ModelConstraints)}
	constraints[cap].Models[modelId] = &core.ModelConstraint{Warm: true, Capacity: 1}
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

func (a stubAIWorker) TextToImage(ctx context.Context, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
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

func (a stubAIWorker) ImageToImage(ctx context.Context, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
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

func (a stubAIWorker) ImageToVideo(ctx context.Context, req worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
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

func (a stubAIWorker) Upscale(ctx context.Context, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
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

func (a stubAIWorker) AudioToText(ctx context.Context, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.TextResponse{Text: "Transcribed text"}, nil
	}
}

func (a stubAIWorker) SegmentAnything2(ctx context.Context, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	a.called++
	if a.err != nil {
		return nil, a.err
	} else {
		return &worker.MasksResponse{
			Masks:  "[[[2.84, 2.83, ...], [2.92, 2.91, ...], [3.22, 3.56, ...], ...]]",
			Scores: "[0.50, 0.37, ...]",
			Logits: "[[[2.84, 2.66, ...], [3.59, 5.20, ...], [5.07, 5.68, ...], ...]]",
		}, nil
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
