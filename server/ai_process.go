package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"math/big"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

const (
	defaultTextToImageModelID      = "stabilityai/sdxl-turbo"
	defaultImageToImageModelID     = "stabilityai/sdxl-turbo"
	defaultImageToVideoModelID     = "stabilityai/stable-video-diffusion-img2vid-xt"
	defaultUpscaleModelID          = "stabilityai/stable-diffusion-x4-upscaler"
	defaultAudioToTextModelID      = "openai/whisper-large-v3"
	defaultLLMModelID              = "meta-llama/llama-3.1-8B-Instruct"
	defaultSegmentAnything2ModelID = "facebook/sam2-hiera-large"
	defaultImageToTextModelID      = "Salesforce/blip-image-captioning-large"
	defaultLiveVideoToVideoModelID = "noop"
	defaultTextToSpeechModelID     = "parler-tts/parler-tts-large-v1"

	maxTries         = 20
	maxSameSessTries = 3
)

var errWrongFormat = fmt.Errorf("result not in correct format")

type ServiceUnavailableError struct {
	err error
}

func (e *ServiceUnavailableError) Error() string {
	return e.err.Error()
}

type BadRequestError struct {
	err error
}

func (e *BadRequestError) Error() string {
	return e.err.Error()
}

// parseBadRequestError checks if the error is a bad request error and returns a BadRequestError.
func parseBadRequestError(err error) *BadRequestError {
	if err == nil {
		return nil
	}
	if err, ok := err.(*BadRequestError); ok {
		return err
	}

	const errorCode = "returned 400"
	if !strings.Contains(err.Error(), errorCode) {
		return nil
	}

	parts := strings.SplitN(err.Error(), errorCode, 2)
	detail := strings.TrimSpace(parts[1])
	if detail == "" {
		detail = "bad request"
	}

	return &BadRequestError{err: errors.New(detail)}
}

type aiRequestParams struct {
	node        *core.LivepeerNode
	os          drivers.OSSession
	sessManager *AISessionManager

	liveParams *liveRequestParams
}

// For live video pipelines
type liveRequestParams struct {
	segmentReader *media.SwitchableSegmentReader
	stream        string
	requestID     string
	streamID      string
	manifestID    string
	pipelineID    string
	pipeline      string
	orchestrator  string

	paymentSender          LivePaymentSender
	paymentProcessInterval time.Duration
	outSegmentTimeout      time.Duration

	// list of RTMP output destinations
	rtmpOutputs []string
	// prefix to identify local (MediaMTX) RTMP hosts
	localRTMPPrefix string

	// Stops the pipeline with an error. Also kicks the input
	kickInput func(error)
	// Cancels the execution for the given Orchestrator session
	kickOrch context.CancelCauseFunc

	// Report an error event
	sendErrorEvent func(error)

	// State for the stream processing
	// startTime is the time when the first request is sent to the orchestrator
	startTime time.Time
	// sess is passed from the orchestrator selection, ugly hack
	sess *AISession

	// Everything below needs to be protected by `mu` for concurrent modification + access
	mu sync.Mutex

	// when the write for the last segment started
	lastSegmentTime time.Time
}

// CalculateTextToImageLatencyScore computes the time taken per pixel for an text-to-image request.
func CalculateTextToImageLatencyScore(took time.Duration, req worker.GenTextToImageJSONRequestBody, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	// TODO: Default values for the number of inference steps is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	numInferenceSteps := float64(50)
	if req.NumInferenceSteps != nil {
		numInferenceSteps = math.Max(1, float64(*req.NumInferenceSteps))
	}
	// Handle special case for SDXL-Lightning model.
	if strings.HasPrefix(*req.ModelId, "ByteDance/SDXL-Lightning") {
		numInferenceSteps = math.Max(1, core.ParseStepsFromModelID(req.ModelId, 8))
	}

	return took.Seconds() / float64(outPixels) / numInferenceSteps
}

func processTextToImage(ctx context.Context, params aiRequestParams, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp, ok := resp.(*worker.ImageResponse)
	if !ok {
		return nil, errWrongFormat
	}

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var result []byte
		var data bytes.Buffer
		var name string
		writer := bufio.NewWriter(&data)
		err := worker.ReadImageB64DataUrl(media.Url, writer)
		if err == nil {
			// orchestrator sent base64 encoded result in .Url
			name = string(core.RandomManifestID()) + ".png"
			writer.Flush()
			result = data.Bytes()
		} else {
			// orchestrator sent download url, get the data
			name = filepath.Base(media.Url)
			result, err = core.DownloadData(ctx, media.Url)
			if err != nil {
				return nil, err
			}
		}

		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(result), nil, 0)

		if err != nil {
			return nil, fmt.Errorf("error saving image to objectStore: %w", err)
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitTextToImage(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))

	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if req.Height == nil {
		req.Height = new(int)
		*req.Height = 512
	}
	if req.Width == nil {
		req.Width = new(int)
		*req.Width = 512
	}

	// TODO: Default values for the number of images is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	defaultNumImages := 1
	if req.NumImagesPerPrompt == nil {
		req.NumImagesPerPrompt = &defaultNumImages
	} else {
		*req.NumImagesPerPrompt = int(math.Max(1, float64(*req.NumImagesPerPrompt)))
	}

	outPixels := int64(*req.Height) * int64(*req.Width) * int64(*req.NumImagesPerPrompt)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenTextToImageWithResponse(ctx, req, setHeaders)
	took := time.Since(start)

	// TODO: Refine this rough estimate in future iterations.
	sess.LatencyScore = CalculateTextToImageLatencyScore(took, req, outPixels)

	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "text-to-image", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return resp.JSON200, nil
}

// CalculateImageToImageLatencyScore computes the time taken per pixel for an image-to-image request.
func CalculateImageToImageLatencyScore(took time.Duration, req worker.GenImageToImageMultipartRequestBody, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	// TODO: Default values for the number of inference steps is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	numInferenceSteps := float64(100)
	if req.NumInferenceSteps != nil {
		numInferenceSteps = math.Max(1, float64(*req.NumInferenceSteps))
	}
	// Handle special case for SDXL-Lightning model.
	if strings.HasPrefix(*req.ModelId, "ByteDance/SDXL-Lightning") {
		numInferenceSteps = math.Max(1, core.ParseStepsFromModelID(req.ModelId, 8))
	}

	return took.Seconds() / float64(outPixels) / numInferenceSteps
}

func processImageToImage(ctx context.Context, params aiRequestParams, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp, ok := resp.(*worker.ImageResponse)
	if !ok {
		return nil, errWrongFormat
	}

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var result []byte
		var data bytes.Buffer
		var name string
		writer := bufio.NewWriter(&data)
		err := worker.ReadImageB64DataUrl(media.Url, writer)
		if err == nil {
			// orchestrator sent bae64 encoded result in .Url
			name = string(core.RandomManifestID()) + ".png"
			writer.Flush()
			result = data.Bytes()
		} else {
			// orchestrator sent download url, get the data
			name = filepath.Base(media.Url)
			result, err = core.DownloadData(ctx, media.Url)
			if err != nil {
				return nil, err
			}
		}

		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(result), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("error saving image to objectStore: %w", err)
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitImageToImage(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	// TODO: Default values for the number of images is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	defaultNumImages := 1
	if req.NumImagesPerPrompt == nil {
		req.NumImagesPerPrompt = &defaultNumImages
	} else {
		*req.NumImagesPerPrompt = int(math.Max(1, float64(*req.NumImagesPerPrompt)))
	}

	var buf bytes.Buffer
	mw, err := worker.NewImageToImageMultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	outPixels := int64(config.Height) * int64(config.Width) * int64(*req.NumImagesPerPrompt)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenImageToImageWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)

	// TODO: Refine this rough estimate in future iterations.
	sess.LatencyScore = CalculateImageToImageLatencyScore(took, req, outPixels)

	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-image", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "image-to-image", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return resp.JSON200, nil
}

// CalculateImageToVideoLatencyScore computes the time taken per pixel for an image-to-video request.
func CalculateImageToVideoLatencyScore(took time.Duration, req worker.GenImageToVideoMultipartRequestBody, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	// TODO: Default values for the number of inference steps is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	numInferenceSteps := float64(25)
	if req.NumInferenceSteps != nil {
		numInferenceSteps = math.Max(1, float64(*req.NumInferenceSteps))
	}

	return took.Seconds() / float64(outPixels) / numInferenceSteps
}

func processImageToVideo(ctx context.Context, params aiRequestParams, req worker.GenImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	// HACK: Re-use worker.ImageResponse to return results
	// TODO: Refactor to return worker.VideoResponse
	imgResp, ok := resp.(*worker.ImageResponse)
	if !ok {
		return nil, errWrongFormat
	}

	videos := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		data, err := core.DownloadData(ctx, media.Url)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(media.Url)
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("error saving video to objectStore: %w", err)
		}

		videos[i] = worker.Media{
			Nsfw: media.Nsfw,
			Seed: media.Seed,
			Url:  newUrl,
		}

	}

	imgResp.Images = videos

	return imgResp, nil
}

func submitImageToVideo(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToVideoMultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if req.Height == nil {
		req.Height = new(int)
		*req.Height = 576
	}
	if req.Width == nil {
		req.Width = new(int)
		*req.Width = 1024
	}
	frames := int64(25)

	outPixels := int64(*req.Height) * int64(*req.Width) * frames
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenImageToVideoWithBody(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(data))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	var res worker.ImageResponse
	if err := json.Unmarshal(data, &res); err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-video", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = CalculateImageToVideoLatencyScore(took, req, outPixels)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "image-to-video", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return &res, nil
}

// CalculateUpscaleLatencyScore computes the time taken per pixel for an upscale request.
func CalculateUpscaleLatencyScore(took time.Duration, req worker.GenUpscaleMultipartRequestBody, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	// TODO: Default values for the number of inference steps is currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	numInferenceSteps := float64(75)
	if req.NumInferenceSteps != nil {
		numInferenceSteps = math.Max(1, float64(*req.NumInferenceSteps))
	}

	return took.Seconds() / float64(outPixels) / numInferenceSteps
}

func processUpscale(ctx context.Context, params aiRequestParams, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp, ok := resp.(*worker.ImageResponse)
	if !ok {
		return nil, errWrongFormat
	}

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var result []byte
		var data bytes.Buffer
		var name string
		writer := bufio.NewWriter(&data)
		err := worker.ReadImageB64DataUrl(media.Url, writer)
		if err == nil {
			// orchestrator sent bae64 encoded result in .Url
			name = string(core.RandomManifestID()) + ".png"
			writer.Flush()
			result = data.Bytes()
		} else {
			// orchestrator sent download url, get the data
			name = filepath.Base(media.Url)
			result, err = core.DownloadData(ctx, media.Url)
			if err != nil {
				return nil, err
			}
		}

		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(result), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("error saving image to objectStore: %w", err)
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitUpscale(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewUpscaleMultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	outPixels := int64(config.Height) * int64(config.Width)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenUpscaleWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "upscale", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = CalculateUpscaleLatencyScore(took, req, outPixels)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "upscale", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return resp.JSON200, nil
}

// CalculateSegmentAnything2LatencyScore computes the time taken per pixel for a segment-anything-2 request.
func CalculateSegmentAnything2LatencyScore(took time.Duration, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	return took.Seconds() / float64(outPixels)
}

func processSegmentAnything2(ctx context.Context, params aiRequestParams, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	txtResp := resp.(*worker.MasksResponse)

	return txtResp, nil
}

func submitSegmentAnything2(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewSegmentAnything2MultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	outPixels := int64(config.Height) * int64(config.Width)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenSegmentAnything2WithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "segment-anything-2", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = CalculateSegmentAnything2LatencyScore(took, outPixels)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "segment-anything-2", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return resp.JSON200, nil
}

// CalculateTextToSpeechLatencyScore computes the time taken per character for a TextToSpeech request.
func CalculateTextToSpeechLatencyScore(took time.Duration, inCharacters int64) float64 {
	if inCharacters <= 0 {
		return 0
	}

	return took.Seconds() / float64(inCharacters)
}

func processTextToSpeech(ctx context.Context, params aiRequestParams, req worker.GenTextToSpeechJSONRequestBody) (*worker.AudioResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	audioResp, ok := resp.(*worker.AudioResponse)
	if !ok {
		return nil, errWrongFormat
	}

	var result []byte
	var data bytes.Buffer
	var name string
	writer := bufio.NewWriter(&data)
	err = worker.ReadAudioB64DataUrl(audioResp.Audio.Url, writer)
	if err == nil {
		// orchestrator sent bae64 encoded result in .Url
		name = string(core.RandomManifestID()) + ".wav"
		writer.Flush()
		result = data.Bytes()
	} else {
		// orchestrator sent download url, get the data
		name = filepath.Base(audioResp.Audio.Url)
		result, err = core.DownloadData(ctx, audioResp.Audio.Url)
		if err != nil {
			return nil, err
		}
	}

	newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(result), nil, 0)
	if err != nil {
		return nil, fmt.Errorf("error saving image to objectStore: %w", err)
	}

	audioResp.Audio.Url = newUrl
	return audioResp, nil
}

func submitTextToSpeech(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenTextToSpeechJSONRequestBody) (*worker.AudioResponse, error) {
	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-speech", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if req.Text == nil {
		return nil, &BadRequestError{errors.New("text field is required")}
	}

	textLength := len(*req.Text)
	clog.V(common.VERBOSE).Infof(ctx, "Submitting text-to-speech request with text length: %d", textLength)
	inCharacters := int64(textLength)
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, inCharacters)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-speech", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenTextToSpeechWithResponse(ctx, req, setHeaders)
	took := time.Since(start)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-speech", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = CalculateSegmentAnything2LatencyScore(took, inCharacters)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "text-to-speech", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	var res worker.AudioResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "text-to-speech", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	return &res, nil
}

// CalculateAudioToTextLatencyScore computes the time taken per second of audio for an audio-to-text request.
func CalculateAudioToTextLatencyScore(took time.Duration, durationSeconds int64) float64 {
	if durationSeconds <= 0 {
		return 0
	}

	return took.Seconds() / float64(durationSeconds)
}

func processAudioToText(ctx context.Context, params aiRequestParams, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	txtResp, ok := resp.(*worker.TextResponse)
	if !ok {
		return nil, errWrongFormat
	}

	return txtResp, nil
}

func submitAudioToText(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	durationSeconds, err := common.CalculateAudioDuration(req.Audio)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	// Add the duration to the request via 'metadata' field.
	metadata := map[string]string{
		"duration": strconv.Itoa(int(durationSeconds)),
	}
	metadataStr := encodeReqMetadata(metadata)
	req.Metadata = &metadataStr

	var buf bytes.Buffer
	mw, err := worker.NewAudioToTextMultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	clog.V(common.VERBOSE).Infof(ctx, "Submitting audio-to-text media with duration: %d seconds", durationSeconds)
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, durationSeconds*1000)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenAudioToTextWithBody(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(data))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	var res worker.TextResponse
	if err := json.Unmarshal(data, &res); err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "audio-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = CalculateAudioToTextLatencyScore(took, durationSeconds)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "audio-to-text", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return &res, nil
}

const initPixelsToPay = 60 * 30 * 720 * 1280 // 60 seconds, 30fps, 1280p

func submitLiveVideoToVideo(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenLiveVideoToVideoJSONRequestBody) (any, error) {
	sess = sess.Clone()

	// Storing sess in the liveParams; it's ugly, but we need to pass it back and don't want to break this function interface
	params.liveParams.sess = sess
	params.liveParams.startTime = time.Now()

	var paymentHeaders worker.RequestEditorFn
	if hasRemoteSigner(params) {
		rpp, ok := params.liveParams.paymentSender.(*remotePaymentSender)
		if !ok {
			return nil, errors.New("remote sender was not the correct type")
		}
		res, err := rpp.RequestPayment(ctx, &SegmentInfoSender{
			sess: sess.BroadcastSession,
		})
		if err != nil {
			return nil, err
		}
		paymentHeaders = func(_ context.Context, req *http.Request) error {
			req.Header.Set(segmentHeader, res.SegCreds)
			req.Header.Set(paymentHeader, res.Payment)
			req.Header.Set("Authorization", protoVerAIWorker)
			return nil
		}
	} else {

		// Live Video should not reuse the existing session balance, because it could lead to not sending the init
		// payment, which in turns may cause "Insufficient Balance" on the Orchestrator's side.
		// It works differently than other AI Jobs, because Live Video is accounted by mid on the Orchestrator's side.
		clearSessionBalance(sess.BroadcastSession, core.RandomManifestID())

		var (
			balUpdate *BalanceUpdate
			err       error
		)
		paymentHeaders, balUpdate, err = prepareAIPayment(ctx, sess, initPixelsToPay)
		if err != nil {
			if monitor.Enabled {
				monitor.AIRequestError(err.Error(), "LiveVideoToVideo", *req.ModelId, sess.OrchestratorInfo)
			}
			return nil, err
		}
		defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)
	}

	// Send request to orchestrator
	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "LiveVideoToVideo", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	reqTimeout := 5 * time.Second
	reqCtx, cancel := context.WithTimeout(ctx, reqTimeout)
	defer cancel()
	resp, err := client.GenLiveVideoToVideoWithResponse(reqCtx, req, paymentHeaders)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	if resp.JSON200.ControlUrl == nil {
		return nil, errors.New("control URL is missing")
	}

	return resp, nil
}

func CalculateLLMLatencyScore(took time.Duration, tokensUsed int) float64 {
	if tokensUsed <= 0 {
		return 0
	}

	return took.Seconds() / float64(tokensUsed)
}

func processLLM(ctx context.Context, params aiRequestParams, req worker.GenLLMJSONRequestBody) (interface{}, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	if req.Stream != nil && *req.Stream {
		streamChan, ok := resp.(chan *worker.LLMResponse)
		if !ok {
			return nil, errors.New("unexpected response type for streaming request")
		}
		return streamChan, nil
	}

	llmResp, ok := resp.(*worker.LLMResponse)
	if !ok {
		return nil, errors.New("unexpected response type")
	}

	return llmResp, nil
}

func submitLLM(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenLLMJSONRequestBody) (interface{}, error) {

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "llm", *req.Model, sess.OrchestratorInfo)
		}
		return nil, err
	}

	// TODO: Improve pricing
	if req.MaxTokens == nil {
		req.MaxTokens = new(int)
		*req.MaxTokens = 256
	}
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, int64(*req.MaxTokens))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "llm", *req.Model, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenLLM(ctx, req, setHeaders)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "llm", *req.Model, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	// TODO: move to after receive stream response in handleSSEStream and handleNonStreamingResponse to count input tokens
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	if req.Stream != nil && *req.Stream {
		return handleSSEStream(ctx, resp.Body, sess, req, start)
	}

	return handleNonStreamingResponse(ctx, resp.Body, sess, req, start)
}

func handleSSEStream(ctx context.Context, body io.ReadCloser, sess *AISession, req worker.GenLLMJSONRequestBody, start time.Time) (chan *worker.LLMResponse, error) {
	streamChan := make(chan *worker.LLMResponse, 100)
	go func() {
		defer close(streamChan)
		defer body.Close()
		scanner := bufio.NewScanner(body)
		var totalTokens worker.LLMTokenUsage
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				var chunk worker.LLMResponse
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					clog.Errorf(ctx, "Error unmarshaling SSE data: %v", err)
					continue
				}
				totalTokens = chunk.Usage
				streamChan <- &chunk
				//check if stream is finished
				if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			clog.Errorf(ctx, "Error reading SSE stream: %v", err)
		}

		took := time.Since(start)
		sess.LatencyScore = CalculateLLMLatencyScore(took, totalTokens.TotalTokens)

		if monitor.Enabled {
			var pricePerAIUnit float64
			if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
				pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
			}
			monitor.AIRequestFinished(ctx, "llm", *req.Model, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
		}
	}()

	return streamChan, nil
}

func handleNonStreamingResponse(ctx context.Context, body io.ReadCloser, sess *AISession, req worker.GenLLMJSONRequestBody, start time.Time) (*worker.LLMResponse, error) {
	data, err := io.ReadAll(body)
	defer body.Close()
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "llm", *req.Model, sess.OrchestratorInfo)
		}
		return nil, err
	}

	var res worker.LLMResponse
	if err := json.Unmarshal(data, &res); err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "llm", *req.Model, sess.OrchestratorInfo)
		}
		return nil, err
	}

	took := time.Since(start)

	sess.LatencyScore = CalculateLLMLatencyScore(took, res.Usage.TotalTokens)

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}
		monitor.AIRequestFinished(ctx, "llm", *req.Model, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return &res, nil
}

func CalculateImageToTextLatencyScore(took time.Duration, outPixels int64) float64 {
	if outPixels <= 0 {
		return 0
	}

	return took.Seconds() / float64(outPixels)
}

func submitImageToText(ctx context.Context, params aiRequestParams, sess *AISession, req worker.GenImageToTextMultipartRequestBody) (*worker.ImageToTextResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToTextMultipartWriter(&buf, req)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	inPixels := int64(config.Height) * int64(config.Width)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, inPixels)
	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.GenImageToTextWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)

	// TODO: Refine this rough estimate in future iterations.
	sess.LatencyScore = CalculateImageToTextLatencyScore(took, inPixels)

	if err != nil {
		if monitor.Enabled {
			monitor.AIRequestError(err.Error(), "image-to-text", *req.ModelId, sess.OrchestratorInfo)
		}
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	if balUpdate != nil {
		balUpdate.Status = ReceivedChange
	}

	if monitor.Enabled {
		var pricePerAIUnit float64
		if priceInfo := sess.OrchestratorInfo.GetPriceInfo(); priceInfo != nil && priceInfo.PixelsPerUnit != 0 {
			pricePerAIUnit = float64(priceInfo.PricePerUnit) / float64(priceInfo.PixelsPerUnit)
		}

		monitor.AIRequestFinished(ctx, "image-to-text", *req.ModelId, monitor.AIJobInfo{LatencyScore: sess.LatencyScore, PricePerUnit: pricePerAIUnit}, sess.OrchestratorInfo)
	}

	return resp.JSON200, nil
}

func processImageToText(ctx context.Context, params aiRequestParams, req worker.GenImageToTextMultipartRequestBody) (*worker.ImageToTextResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	txtResp := resp.(*worker.ImageToTextResponse)

	return txtResp, nil
}

func processAIRequest(ctx context.Context, params aiRequestParams, req interface{}) (interface{}, error) {
	var cap core.Capability
	var modelID string
	var submitFn func(context.Context, aiRequestParams, *AISession) (interface{}, error)

	switch v := req.(type) {
	case worker.GenTextToImageJSONRequestBody:
		cap = core.Capability_TextToImage
		modelID = defaultTextToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitTextToImage(ctx, params, sess, v)
		}
		ctx = clog.AddVal(ctx, "prompt", v.Prompt)
	case worker.GenImageToImageMultipartRequestBody:
		cap = core.Capability_ImageToImage
		modelID = defaultImageToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitImageToImage(ctx, params, sess, v)
		}
		ctx = clog.AddVal(ctx, "prompt", v.Prompt)
	case worker.GenImageToVideoMultipartRequestBody:
		cap = core.Capability_ImageToVideo
		modelID = defaultImageToVideoModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitImageToVideo(ctx, params, sess, v)
		}
	case worker.GenUpscaleMultipartRequestBody:
		cap = core.Capability_Upscale
		modelID = defaultUpscaleModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitUpscale(ctx, params, sess, v)
		}
		ctx = clog.AddVal(ctx, "prompt", v.Prompt)
	case worker.GenAudioToTextMultipartRequestBody:
		cap = core.Capability_AudioToText
		modelID = defaultAudioToTextModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitAudioToText(ctx, params, sess, v)
		}
	case worker.GenLLMJSONRequestBody:
		cap = core.Capability_LLM
		modelID = defaultLLMModelID
		if v.Model != nil {
			modelID = *v.Model
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitLLM(ctx, params, sess, v)
		}

	case worker.GenSegmentAnything2MultipartRequestBody:
		cap = core.Capability_SegmentAnything2
		modelID = defaultSegmentAnything2ModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitSegmentAnything2(ctx, params, sess, v)
		}
	case worker.GenImageToTextMultipartRequestBody:
		cap = core.Capability_ImageToText
		modelID = defaultImageToTextModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitImageToText(ctx, params, sess, v)
		}
	case worker.GenTextToSpeechJSONRequestBody:
		cap = core.Capability_TextToSpeech
		modelID = defaultTextToSpeechModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitTextToSpeech(ctx, params, sess, v)
		}
	case worker.GenLiveVideoToVideoJSONRequestBody:
		cap = core.Capability_LiveVideoToVideo
		modelID = defaultLiveVideoToVideoModelID
		if v.ModelId != nil && *v.ModelId != "" {
			modelID = *v.ModelId
		} else {
			// set default model
			v.ModelId = &modelID
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitLiveVideoToVideo(ctx, params, sess, v)
		}
	default:
		return nil, fmt.Errorf("unsupported request type %T", req)
	}
	capName := cap.String()
	if capName != "Live video to video" {
		ctx = clog.AddVal(ctx, "capability", capName)
	}
	ctx = clog.AddVal(ctx, "model_id", modelID)

	clog.V(common.VERBOSE).Infof(ctx, "Received AI request model_id=%s", modelID)
	start := time.Now()
	defer clog.Infof(ctx, "Processed AI request model_id=%v took=%v", modelID, time.Since(start))

	var resp interface{}

	processingRetryTimeout := params.node.AIProcesssingRetryTimeout
	cctx, cancel := context.WithTimeout(ctx, processingRetryTimeout)
	defer cancel()

	tries := 0
	sessTries := map[string]int{}
	var retryableSessions []*AISession
	for tries < maxTries {
		select {
		case <-cctx.Done():
			err := cctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("no orchestrators available within %v timeout", processingRetryTimeout)
				if monitor.Enabled {
					monitor.AIRequestError(err.Error(), monitor.ToPipeline(capName), modelID, nil)
				}
			}
			return nil, &ServiceUnavailableError{err: err}
		default:
		}

		tries++
		sess, err := params.sessManager.Select(ctx, cap, modelID)
		if err != nil {
			clog.Infof(ctx, "Error selecting session modelID=%v err=%v", modelID, err)
			if cap == core.Capability_LiveVideoToVideo && sess != nil {
				// for live video, remove the session from the pool to avoid retrying it
				params.sessManager.Remove(ctx, sess)
			}
			continue
		}
		if sess == nil {
			break
		}
		sessTries[sess.Transcoder()]++
		if params.liveParams != nil {
			if params.liveParams.orchestrator != "" && !strings.Contains(sess.Transcoder(), params.liveParams.orchestrator) {
				// user requested a specific orchestrator, so ignore all the others
				clog.Infof(ctx, "Skipping orchestrator=%s because user request specific orchestrator=%s", sess.Transcoder(), params.liveParams.orchestrator)
				retryableSessions = append(retryableSessions, sess)
				continue
			}
		}

		resp, err = submitFn(ctx, params, sess)
		if err == nil {
			params.sessManager.Complete(ctx, sess)
			break
		}

		// Don't suspend the session if the error is a transient error.
		if isRetryableError(err) && sessTries[sess.Transcoder()] < maxSameSessTries {
			clog.Infof(ctx, "Error submitting request with retryable error modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
			params.sessManager.Complete(ctx, sess)
			continue
		}

		// retry some specific errors with another session. re-check for retryable errors in case max retries were hit above
		if isRetryableError(err) || isInvalidTicketSenderNonce(err) || isNoCapacityError(err) {
			clog.Infof(ctx, "Error submitting request with non-retryable error modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
			if cap == core.Capability_LiveVideoToVideo {
				// for live video, remove the session from the pool to avoid retrying it
				params.sessManager.Remove(ctx, sess)
			} else {
				// for non realtime video, get the session back to the pool as soon as the request completes
				retryableSessions = append(retryableSessions, sess)
			}
			continue
		}

		// Suspend the session on other errors.
		clog.Infof(ctx, "Error submitting request modelID=%v try=%v orch=%v err=%v", modelID, tries, sess.Transcoder(), err)
		params.sessManager.Remove(ctx, sess) //TODO: Improve session selection logic for live-video-to-video

		if errors.Is(err, common.ErrAudioDurationCalculation) {
			return nil, &BadRequestError{err}
		}

		if badRequestErr := parseBadRequestError(err); badRequestErr != nil {
			return nil, badRequestErr
		}
	}

	//add retryable sessions back to selector
	for _, sess := range retryableSessions {
		params.sessManager.Complete(ctx, sess)
	}

	if resp == nil {
		errMsg := "no orchestrators available"
		if monitor.Enabled {
			monitor.AIRequestError(errMsg, monitor.ToPipeline(capName), modelID, nil)
		}
		monitor.SendQueueEventAsync("stream_trace", map[string]interface{}{
			"type":        "gateway_no_orchestrators_available",
			"timestamp":   time.Now().UnixMilli(),
			"stream_id":   params.liveParams.streamID,
			"pipeline_id": params.liveParams.pipelineID,
			"request_id":  params.liveParams.requestID,
			"orchestrator_info": map[string]interface{}{
				"address": "",
				"url":     "",
			},
		})
		return nil, &ServiceUnavailableError{err: errors.New(errMsg)}
	}
	return resp, nil
}

// isRetryableError checks if the error is a transient error that can be retried.
func isRetryableError(err error) bool {
	return errContainsMsg(err, "ticketparams expired")
}

func isInvalidTicketSenderNonce(err error) bool {
	return errContainsMsg(err, "invalid ticket sendernonce")
}

func isNoCapacityError(err error) bool {
	return errContainsMsg(err, "insufficient capacity")
}

func errContainsMsg(err error, msgs ...string) bool {
	errMsg := strings.ToLower(err.Error())
	for _, msg := range msgs {
		if strings.Contains(errMsg, msg) {
			return true
		}
	}
	return false
}
func prepareAIPayment(ctx context.Context, sess *AISession, outPixels int64) (worker.RequestEditorFn, *BalanceUpdate, error) {
	// genSegCreds expects a stream.HLSSegment so in order to reuse it here we pass a dummy object
	segCreds, err := genSegCreds(sess.BroadcastSession, &stream.HLSSegment{}, nil, false)
	if err != nil {
		return nil, nil, err
	}

	priceInfo, err := common.RatPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
	if err != nil {
		return nil, nil, err
	}

	// At the moment, outPixels is expected to just be height * width * frames
	// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * frames * steps
	// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
	fee, err := estimateAIFee(outPixels, priceInfo)
	if err != nil {
		return nil, nil, err
	}

	balUpdate, err := newBalanceUpdate(sess.BroadcastSession, fee)
	if err != nil {
		return nil, nil, err
	}
	balUpdate.Debit = fee

	payment, err := genPayment(ctx, sess.BroadcastSession, balUpdate.NumTickets)
	if err != nil {
		clog.Errorf(ctx, "Could not create payment err=%q", err)

		if monitor.Enabled {
			monitor.PaymentCreateError(ctx)
		}

		return nil, nil, err
	}

	// As soon as the request is sent to the orch consider the balance update's credit as spent
	balUpdate.Status = CreditSpent
	if monitor.Enabled {
		monitor.TicketValueSent(ctx, balUpdate.NewCredit)
		monitor.TicketsSent(ctx, balUpdate.NumTickets)
	}

	setHeaders := func(_ context.Context, req *http.Request) error {
		req.Header.Set(segmentHeader, segCreds)
		req.Header.Set(paymentHeader, payment)
		req.Header.Set("Authorization", protoVerAIWorker)
		return nil
	}

	return setHeaders, balUpdate, nil
}

func estimateAIFee(outPixels int64, priceInfo *big.Rat) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	fee := new(big.Rat).SetInt64(outPixels)
	fee.Mul(fee, priceInfo)

	return fee, nil
}

// encodeReqMetadata encodes a map of metadata into a JSON string.
func encodeReqMetadata(metadata map[string]string) string {
	metadataBytes, _ := json.Marshal(metadata)
	return string(metadataBytes)
}

func hasRemoteSigner(params aiRequestParams) bool {
	return params.node != nil && params.node.RemoteSignerAddr != nil
}
