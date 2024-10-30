package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
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
	oapitypes "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
)

func TestRemoteAIWorker_Error(t *testing.T) {
	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

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
	//send empty request data
	notify := createAIJob(742, "text-to-image-empty", "", "")
	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.NotNil(string(body))

	//error in worker, good request
	notify = createAIJob(742, "text-to-image", "livepeer/model1", "")
	errText := "Some error"
	wkr.Err = errors.New(errText)

	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("742", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal(errText, string(body))

	// unrecoverable error
	// send the response and panic
	wkr.Err = core.NewUnrecoverableError(errors.New("some error"))
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

	//pipeline not compatible
	wkr.Err = nil
	notify = createAIJob(743, "unsupported-pipeline", "livepeer/model1", "")

	runAIJob(node, parsedURL.Host, httpc, notify)
	time.Sleep(3 * time.Millisecond)

	assert.NotNil(body)
	assert.Equal("743", headers.Get("TaskId"))
	assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
	assert.Equal(node.OrchSecret, headers.Get("Credentials"))
	assert.Equal(protoVerAIWorker, headers.Get("Authorization"))
	assert.Equal("AI request validation failed for", string(body)[:32])

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
	modelId := "livepeer/model1"
	tests := []struct {
		inputFile       oapitypes.File
		name            string
		notify          *net.NotifyAIJob
		pipeline        string
		expectedErr     string
		expectedOutputs int
	}{
		{
			name:            "TextToImage_Success",
			notify:          createAIJob(1, "text-to-image", modelId, ""),
			pipeline:        "text-to-image",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "ImageToImage_Success",
			notify:          createAIJob(2, "image-to-image", modelId, parsedURL.String()+"/image.png"),
			pipeline:        "image-to-image",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "Upscale_Success",
			notify:          createAIJob(3, "upscale", modelId, parsedURL.String()+"/image.png"),
			pipeline:        "upscale",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "ImageToVideo_Success",
			notify:          createAIJob(4, "image-to-video", modelId, parsedURL.String()+"/image.png"),
			pipeline:        "image-to-video",
			expectedErr:     "",
			expectedOutputs: 2,
		},
		{
			name:            "AudioToText_Success",
			notify:          createAIJob(5, "audio-to-text", modelId, parsedURL.String()+"/audio.mp3"),
			pipeline:        "audio-to-text",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "SegmentAnything2_Success",
			notify:          createAIJob(6, "segment-anything-2", modelId, parsedURL.String()+"/image.png"),
			pipeline:        "segment-anything-2",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "LLM_Success",
			notify:          createAIJob(7, "llm", modelId, ""),
			pipeline:        "llm",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "TextToSpeech_Success",
			notify:          createAIJob(8, "text-to-speech", modelId, ""),
			pipeline:        "text-to-speech",
			expectedErr:     "",
			expectedOutputs: 1,
		},
		{
			name:            "UnsupportedPipeline",
			notify:          createAIJob(8, "unsupported-pipeline", modelId, ""),
			pipeline:        "unsupported-pipeline",
			expectedErr:     "AI request validation failed for",
			expectedOutputs: 0,
		},
		{
			name:            "InvalidRequestData",
			notify:          createAIJob(9, "text-to-image-invalid", modelId, ""),
			pipeline:        "text-to-image",
			expectedErr:     "AI request validation failed for",
			expectedOutputs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wkr := stubAIWorker{}
			node, _ := core.NewLivepeerNode(nil, "/tmp/thisdirisnotactuallyusedinthistest", nil)

			node.OrchSecret = "verbigsecret"
			node.AIWorker = &wkr
			node.Capabilities = createStubAIWorkerCapabilitiesForPipelineModelId(tt.pipeline, modelId)

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

			_, params, _ := mime.ParseMediaType(headers.Get("Content-Type"))
			//this part tests the multipart response reading in AIResults()
			results := parseMultiPartResult(bytes.NewBuffer(body), params["boundary"], tt.pipeline)
			json.Unmarshal(body, &results)
			if tt.expectedErr != "" {
				assert.NotNil(body)
				assert.Contains(string(body), tt.expectedErr)
				assert.Equal(aiWorkerErrorMimeType, headers.Get("Content-Type"))
			} else {
				assert.NotNil(body)
				assert.NotEqual(aiWorkerErrorMimeType, headers.Get("Content-Type"))

				switch tt.pipeline {
				case "text-to-image":
					t2iResp, ok := results.Results.(worker.ImageResponse)
					assert.True(ok)
					assert.Equal("1", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 1)
					expectedResp, _ := wkr.TextToImage(context.Background(), worker.GenTextToImageJSONRequestBody{})
					assert.Equal(expectedResp.Images[0].Seed, t2iResp.Images[0].Seed)
				case "image-to-image":
					i2iResp, ok := results.Results.(worker.ImageResponse)
					assert.True(ok)
					assert.Equal("2", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 1)
					expectedResp, _ := wkr.ImageToImage(context.Background(), worker.GenImageToImageMultipartRequestBody{})
					assert.Equal(expectedResp.Images[0].Seed, i2iResp.Images[0].Seed)
				case "upscale":
					upsResp, ok := results.Results.(worker.ImageResponse)
					assert.True(ok)
					assert.Equal("3", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 1)
					expectedResp, _ := wkr.Upscale(context.Background(), worker.GenUpscaleMultipartRequestBody{})
					assert.Equal(expectedResp.Images[0].Seed, upsResp.Images[0].Seed)
				case "image-to-video":
					vidResp, ok := results.Results.(worker.ImageResponse)
					assert.True(ok)
					assert.Equal("4", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 1)
					expectedResp, _ := wkr.ImageToVideo(context.Background(), worker.GenImageToVideoMultipartRequestBody{})
					assert.Equal(expectedResp.Frames[0][0].Seed, vidResp.Images[0].Seed)
				case "audio-to-text":
					res, _ := json.Marshal(results.Results)
					var jsonRes worker.TextResponse
					json.Unmarshal(res, &jsonRes)

					assert.Equal("5", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 0)
					expectedResp, _ := wkr.AudioToText(context.Background(), worker.GenAudioToTextMultipartRequestBody{})
					assert.Equal(expectedResp, &jsonRes)
				case "segment-anything-2":
					res, _ := json.Marshal(results.Results)
					var jsonRes worker.MasksResponse
					json.Unmarshal(res, &jsonRes)

					assert.Equal("6", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 0)
					expectedResp, _ := wkr.SegmentAnything2(context.Background(), worker.GenSegmentAnything2MultipartRequestBody{})
					assert.Equal(expectedResp, &jsonRes)
				case "llm":
					res, _ := json.Marshal(results.Results)
					var jsonRes worker.LLMResponse
					json.Unmarshal(res, &jsonRes)

					assert.Equal("7", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 0)
					expectedResp, _ := wkr.LLM(context.Background(), worker.GenLLMFormdataRequestBody{})
					assert.Equal(expectedResp, &jsonRes)
				case "text-to-speech":
					audResp, ok := results.Results.(worker.AudioResponse)
					assert.True(ok)
					assert.Equal("8", headers.Get("TaskId"))
					assert.Equal(len(results.Files), 1)
					expectedResp, _ := wkr.TextToSpeech(context.Background(), worker.GenTextToSpeechJSONRequestBody{})
					var respFile bytes.Buffer
					worker.ReadAudioB64DataUrl(expectedResp.Audio.Url, &respFile)
					assert.Equal(len(results.Files[audResp.Audio.Url]), respFile.Len())
				}
			}
		})
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
		req = worker.GenLLMFormdataRequestBody{Prompt: "tell me a story", ModelId: &modelId}
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

type stubResult struct {
	Attachment []byte
	Result     string
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
	Called int
	Err    error
}

func (a *stubAIWorker) TextToImage(ctx context.Context, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
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

func (a *stubAIWorker) ImageToImage(ctx context.Context, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.ImageResponse{
			Images: []worker.Media{
				{
					Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
					Nsfw: false,
					Seed: 112,
				},
			},
		}, nil
	}
}

func (a *stubAIWorker) ImageToVideo(ctx context.Context, req worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.VideoResponse{
			Frames: [][]worker.Media{
				{
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 113,
					},
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 131,
					},
					{
						Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
						Nsfw: false,
						Seed: 311,
					},
				},
			},
		}, nil
	}
}

func (a *stubAIWorker) Upscale(ctx context.Context, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.ImageResponse{
			Images: []worker.Media{
				{
					Url:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII=",
					Nsfw: false,
					Seed: 114,
				},
			},
		}, nil
	}
}

func (a *stubAIWorker) AudioToText(ctx context.Context, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.TextResponse{Text: "Transcribed text"}, nil
	}
}

func (a *stubAIWorker) SegmentAnything2(ctx context.Context, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.MasksResponse{
			Masks:  "[[[2.84, 2.83, ...], [2.92, 2.91, ...], [3.22, 3.56, ...], ...]]",
			Scores: "[0.50, 0.37, ...]",
			Logits: "[[[2.84, 2.66, ...], [3.59, 5.20, ...], [5.07, 5.68, ...], ...]]",
		}, nil
	}
}

func (a *stubAIWorker) LLM(ctx context.Context, req worker.GenLLMFormdataRequestBody) (interface{}, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.LLMResponse{Response: "output tokens", TokensUsed: 10}, nil
	}
}

func (a *stubAIWorker) ImageToText(ctx context.Context, req worker.GenImageToTextMultipartRequestBody) (*worker.ImageToTextResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.ImageToTextResponse{Text: "Transcribed text"}, nil
	}
}

func (a *stubAIWorker) TextToSpeech(ctx context.Context, req worker.GenTextToSpeechJSONRequestBody) (*worker.AudioResponse, error) {
	a.Called++
	if a.Err != nil {
		return nil, a.Err
	} else {
		return &worker.AudioResponse{Audio: worker.MediaURL{
			Url: "data:audio/wav;base64,UklGRjwLAABXQVZFZm10IBAAAAABAAEAgLsAAIC7AAABAAgAZGF0YbYKAAB+f39+fn5+fn5+fn5+fn5/f35+fn5/f39/f39/f39/f39/f39/f39/f39/f39/f39/f39/fn5+fn9/f39/f39/f39/f39/f39/f39/f39/fn5+f39/f39+fn5/f35+fn5+fn5+fn5+fn5/fn5+fn5+fn5+fn5+fn5+f39/f39/f39/f39/f39/f35+fn9/f39/f39/f39/f39/f39/f39/f39/f39/f39/f39/f39/f39/f39/f35+f39/f39/f39/f39/f39/f39/f39/gIGBgYGBgoKCgYGBgYGBgYGBgYGBgYGCgoKCgoKCgoKCgoKDg4SEhYWGhoaFhIODgoODhISFhYWEhISEhIWGh4mJiIaHi5SdoJuQg3l2eH2Eio2OjIqHhoeKjY+PjoyLi42QkpKRjoyKi4yNj5KZoaenn5OFe3Z4foePlZiXlpSUlZiam5uZl5WVlpeYl5aTkZGSlJeZmpqYl5aXmpydnp+goaGhoJ6cm5yfoqaoqampqampqamqq6ytra6wsrS1trW0srGxs7W3ubu9vsDAwcHAwL/AwcLCwsLCwsHBwcHBwsLCw8PDw8PDw8LCwsPDw8TExMXFxcXExMTDw8TExMXGx8fHxsbFxcXFxsbHx8fHyMjIyMjHx8fHyMjJycnJycnJysrKysrKysrJycnJycnKysrKy8vLzMzLy8vLy8vLy8vLy8vLzMzMzMzMy8vKysnJyMjIyMjIyMfIyMjHx8bGxsbFxcXEwb66uLa0sq+qo5yVjYZ/eXFnW09FPz08Ozo4NjQzMjExMTAvLi0rKScmJSQjIiAeHRwbGxoZGBYVEhEPDw4ODg4NDQ0NDQ0NDQ0NDQ4ODg4NDg0NDQ0NDQ0NDQ0NDQ0ODg4ODg4ODg4NDQ0NDg4ODg4ODg4ODw8PDw8PDw8PDw8PDw8PEBAQEA8PDw8ODg0NDg8PDw4ODg8PDw8ODg8PDw8ODg4ODw8PDw8PDw8PDg4ODg8QEBEQEBAPDxAQEBETFRcYGhscHB0dHR4fICEhIiIjJSYnKCkpKiorLS4vMTIyMzQ2Nzo8Pj4+QUhUY3N/iZOdqbW/xMbGxsfIycrKy8zNz9DR0tPU1NXV1dXW2Nrc3uDi4+Tl5ubn5+jp6+zu7+/v7u7t7ezs6+rp6urr6+vr6ujn5ubn6Onp6uno5+fm5ubm5+fo6Ojp6enp6Ofm5ufn6Ojp6Ojo5+bl5ebm5+fn5+fm5uXl5eXm5ubl5eTk5OTk5ebm5uXk4+Pj4+Tj4uLi4uPj5OTj4uHg4ODh4uLj4+Li4eHi4uPj4+Pi4uHg4ODh4eHh4eLj5OPh397d3d7f4ePk5OPg3tzb29ze4OHi4uHf3dza2tra29ze3+Dg4N7c29rZ2tra2tnZ2dnZ2drb29zb2tjX1tbW1dXW19jZ2tnZ2NjW1dXV1tfY2NfW1dXU1NTU1dXW19fX2NfX19fW1tXU09PU1dXW19fX1dTT09PU1dbW1tfX19jZ2trZ2dnZ2tra2tra2tra2tvc3d3c29rZ2djY2NjZ2trb2tnY2NjY2NjY19fY2dra2djW1dTT0tPU1tjZ2NbV09PT09PS0dDPz9DR0tPS0dDPzs7Oz9DR0dDQ0NHR0M/Ozs3Nzs7P0NDPzs3MzM3O0NDR0M/NzMvMzs/R0dHQzs7NzMzLy8zNzc7Ozs3LycfGxsbHycnJycjHxsbHyMnKysnJyMjHxMC8urzAxcnKysnGwLixra6yub7Cw8K+uLCooZ+iqLG5vr63qZeHfXyFkp6lpJ+Yko2Ig356eXuAhYuOjomAdWxobHaCjJGQioJ6dXJydXp/hYmMjImEf3t6foOHiYmFgXt2cG1tcHiCi5KVlIx/bVtPTFZoe4mPi4F1aF9bW19mbHBxb2tlX1lWWF9qdXx+enFnXVdVWF1jaGxvcnZ5eXh0bmhkYWFkaGttbGplYV5eY2t1fX96bl5OQz5CTl5sdXVuY1hQS0pMUVhfY2RgWVFIQj9CTFxteXpxYVFHQ0RJT1ZaXV1dXmBiYl9aVE9PUFJUU1FNSUVDREtWYWdkWk5EPjw7Oz5FTlVXUkpCPTs7Oz0+Pz8+PT0+PTw7PD9DRURAPDo7Ozs6Ojo7PDs7Ozs8PDs6OTk5Ojs7Ozo6OTk5ODc3O0FIS0hCOzg3ODc3ODxBRUVCPjs5OTg4OTs+Pz47OTg4Nzk9REpMSEE6NjY4Ojw9Pj4+Pj08Ozw8PT0+Pj8/QD8/Pj08Ozk3OD5LWWFfVEY8NzY3OkFKVFpYUEQ8OT1GTlFPSEE8OTo+R1RhaWdbTUI8PD5CSVFXWllWUk9NTU5SVlpeYGFfXFdST05PUVJTVFheaXR5dmlYSD06PklZand6dGZYT09YZG1tZFZIPz9GU2V3hImEdWRVTElMVGFvfISGgntzbGdmaG92foOGhoWAe3Z0dHNxb3F6iJilqKKRemJRTVlwjKOtqJmIe3qDkpycjXVeUldsjKvBx7uihGxfYW+DlaKoqaeloqCbk4uGiI+bp7G1s6yjmpWWmZualpWZo7C8wb6wmYFuZ3GHorjCva2YiISMnauvoopuWllriqrByL2khGdWV2iAlqOloJiRjo6OjIV8dniBj5ylp6CSg3h0eYGGhYB9f4mWoaSdjHRbSkdUbougpZmCa19jcoKIfmlQPTZBXIKksqeJY0Q0M0BVbX2Ad2pgXmJlZF1SSEJETl1sdHNpWUpCQUdPU1FMSU1YZGplWEpAOjg6Q1NkbmtbRzo6RlVdWlFGPjg1PE5pf4V2W0Q3NDhATFlhX1VJRktSVVFJQT4+QEdSYGprYlJEPT5ESUpHQT9FU2Nta15OQTk3O0VUZW9rWUM1OUxgaWJSQTUvMT5XeI+Pelk+MDA3QU5cYlxNPz1FUVVRSEA6NzY6RVNeXlRHPjk5Ojo6OTY0NT5MV1hQRDo1MzEyN0FKTUk/ODg8QkRBPDk2MzI2Q1Znal5JOC8vMjc8QENDPzo4Ojw9PT09PTs5ODpARklHQz88Ozo5OTg4ODk8QkdIRT85NjU1Njg8QEJBPj0/QkVEQT05NjQ2PU1famZWQzUyNjs+QEJEQ0A+PkNIS0lEPzs6Oz9DSU1OTEhEQkFBQEA+PDs8QEZKTElEPzs5Ojw/QkNCQD49PkFFSEdEQT05NzpFV2hvZ1VCNzU6QUdLTU1JREA/Q0lOT0tGQkFCQ0ZKUFRTT0lEREVGRkNBQENITE5MSUVCQUFBQkNFRkVCQEJJUVVSS0NAPjw/Tmd+iH1jSTs6P0ZOVl1eV0xDQkpVXV1WTUZFRklLT1ZdYmFbU01IRUNBQUVMVVxdWE9HRERGR0dISUtJRUBAR1RfYVpQRkE9OTxMaIKMgGdMPTo9RE1WW1lQR0RKV2JlYFZLQ0BAREpQVlxhZWZjXFFHQDo5QFBkc3VpVUU9PUBDR01TVVBHQ0hWZGhfT0E4NTY/Um6Hj4FmSjgxMztJWWVnXE5GR1BZXlxXUEpGRUhNU1hcXmJlZmVfV0xCPD5KXW94cmJPQj0+QkhRWVtWS0NIWGt0bl1KPDUzOEtphpaPdFU/NjY+TF1rcGlbUE9WX2VmY15ZVFJTVllbXWJqdHp5cGJSRj9BTmN5h4Z2YU5CP0NNWWZsaV5UUlxtenlrV0U4MzdHZYefoYtjdWUgHAAAAAEAAAAAAAAAVAUAAGRhdGEAAAAAAAAAAFQFAABMSVNUGgAAAGFkdGxsYWJsDgAAAAAAAABNYXJrZXIgMQAATElTVBQAAABJTkZPSUFSVAgAAABBbnhpb24AAA=="},
		}, nil
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
