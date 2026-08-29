package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	oapitypes "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
)

func TestRemoteAIWorker_DeprecatedPipeline(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	assert := assert.New(t)

	wkr := stubAIWorker{}
	node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)
	node.OrchSecret = "verbigsecret"
	node.AIWorker = &wkr
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

	for _, pipeline := range []string{"text-to-image", "llm", "unsupported-pipeline"} {
		notify := createAIJob(743, pipeline, "livepeer/model1", "")
		runAIJob(node, parsedURL.Host, httpc, notify)
		time.Sleep(3 * time.Millisecond)

		assert.Equal("743", headers.Get("TaskId"))
		assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
		assert.Equal(node.OrchSecret, headers.Get("Credentials"))
		assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
		assert.Contains(string(body), "AI request validation failed for "+pipeline)
		assert.Contains(string(body), "is deprecated")
	}
}

func createAIJob(taskId int64, pipeline, modelId, inputUrl string) *net.NotifyAIJob {
	var req interface{}
	var inputFile oapitypes.File
	switch pipeline {
	case "text-to-image":
		req = worker.GenTextToImageJSONRequestBody{Prompt: "test prompt", ModelId: &modelId}
	case "image-to-image":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenImageToImageMultipartRequestBody{Prompt: "test prompt", ModelId: &modelId, Image: inputFile}
	case "upscale":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenUpscaleMultipartRequestBody{Prompt: "test prompt", ModelId: &modelId, Image: inputFile}
	case "image-to-video":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenImageToVideoMultipartRequestBody{ModelId: &modelId, Image: inputFile}
	case "audio-to-text":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenAudioToTextMultipartRequestBody{ModelId: &modelId, Audio: inputFile}
	case "segment-anything-2":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenSegmentAnything2MultipartRequestBody{ModelId: &modelId, Image: inputFile}
	case "llm":
		var msgs []worker.LLMMessage
		msgs = append(msgs, worker.LLMMessage{Role: "system", Content: "you are a robot"})
		msgs = append(msgs, worker.LLMMessage{Role: "user", Content: "tell me a story"})
		req = worker.GenLLMJSONRequestBody{Messages: msgs, Model: &modelId}
	case "image-to-text":
		inputFile.InitFromBytes(nil, inputUrl)
		req = worker.GenImageToImageMultipartRequestBody{Prompt: "test prompt", ModelId: &modelId, Image: inputFile}
	case "text-to-speech":
		desc := "a young adult"
		text := "let me tell you a story"
		req = worker.GenTextToSpeechJSONRequestBody{Description: &desc, ModelId: &modelId, Text: &text}
	case "unsupported-pipeline":
		req = worker.GenTextToImageJSONRequestBody{Prompt: "test prompt", ModelId: &modelId}
	case "text-to-image-invalid":
		pipeline = "text-to-image"
		req = []byte(`invalid json`)
	case "text-to-image-empty":
		pipeline = "text-to-image"
		req = worker.GenTextToImageJSONRequestBody{}
	}

	reqData, _ := json.Marshal(core.AIJobRequestData{Request: req, InputUrl: inputUrl})

	jobData := &net.AIJobData{
		Pipeline:    pipeline,
		RequestData: reqData,
	}
	notify := &net.NotifyAIJob{
		TaskId:    taskId,
		AIJobData: jobData,
	}
	return notify
}

func aiResultsTest(l lphttp, w *httptest.ResponseRecorder, r *http.Request) (int, string) {
	handler := l.AIResults()
	handler.ServeHTTP(w, r)
	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(body)
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
	Called int
	Err    error
}

func (a *stubAIWorker) GetLiveAICapacity(pipeline, modelID string) worker.Capacity {
	return worker.Capacity{}
}

func (a *stubAIWorker) LiveVideoToVideo(ctx context.Context, req worker.GenLiveVideoToVideoJSONRequestBody) (*worker.LiveVideoToVideoResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.LiveVideoToVideoResponse{}, nil
	}
}

func (a *stubAIWorker) Warm(ctx context.Context, arg1, arg2 string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	a.Called++
	return nil
}

func (a *stubAIWorker) Stop(ctx context.Context) error {
	a.Called++
	return nil
}

func (a *stubAIWorker) HasCapacity(pipeline, modelID string) bool {
	a.Called++
	return true
}

func (a *stubAIWorker) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	a.Called++
	return nil
}

func (a *stubAIWorker) HardwareInformation() []worker.HardwareInformation {
	a.Called++
	return []worker.HardwareInformation{}
}

func (a *stubAIWorker) Version() []worker.Version {
	a.Called++
	return []worker.Version{}
}
