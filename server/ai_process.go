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
	"math/big"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

const processingRetryTimeout = 2 * time.Second
const defaultTextToImageModelID = "stabilityai/sdxl-turbo"
const defaultImageToImageModelID = "stabilityai/sdxl-turbo"
const defaultImageToVideoModelID = "stabilityai/stable-video-diffusion-img2vid-xt"
const defaultUpscaleModelID = "stabilityai/stable-diffusion-x4-upscaler"
const defaultSpeechToTextModelID = "openai/whisper-large-v3"

type ServiceUnavailableError struct {
	err error
}

type BadRequestError struct {
	err error
}

func (e *BadRequestError) Error() string {
	return e.err.Error()
}

func (e *ServiceUnavailableError) Error() string {
	return e.err.Error()
}

type aiRequestParams struct {
	node        *core.LivepeerNode
	os          drivers.OSSession
	sessManager *AISessionManager
}

func processTextToImage(ctx context.Context, params aiRequestParams, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp := resp.(*worker.ImageResponse)

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var data bytes.Buffer
		writer := bufio.NewWriter(&data)
		if err := worker.ReadImageB64DataUrl(media.Url, writer); err != nil {
			return nil, err
		}
		writer.Flush()

		name := string(core.RandomManifestID()) + ".png"
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data.Bytes()), nil, 0)
		if err != nil {
			return nil, err
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitTextToImage(ctx context.Context, params aiRequestParams, sess *AISession, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
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

	outPixels := int64(*req.Height) * int64(*req.Width)
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.TextToImageWithResponse(ctx, req, setHeaders)
	took := time.Since(start)
	if err != nil {
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

	// TODO: Refine this rough estimate in future iterations.
	// TODO: Default values for the number of images and inference steps are currently hardcoded.
	// These should be managed by the nethttpmiddleware. Refer to issue LIV-412 for more details.
	numImages := float64(1)
	if req.NumImagesPerPrompt != nil {
		numImages = float64(*req.NumImagesPerPrompt)
	}
	numInferenceSteps := float64(50)
	if req.NumInferenceSteps != nil {
		numInferenceSteps = float64(*req.NumInferenceSteps)
	}
	sess.LatencyScore = took.Seconds() / float64(outPixels) / (numImages * numInferenceSteps)

	return resp.JSON200, nil
}

func processImageToImage(ctx context.Context, params aiRequestParams, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp := resp.(*worker.ImageResponse)

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var data bytes.Buffer
		writer := bufio.NewWriter(&data)
		if err := worker.ReadImageB64DataUrl(media.Url, writer); err != nil {
			return nil, err
		}
		writer.Flush()

		name := string(core.RandomManifestID()) + ".png"
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data.Bytes()), nil, 0)
		if err != nil {
			return nil, err
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitImageToImage(ctx context.Context, params aiRequestParams, sess *AISession, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToImageMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		return nil, err
	}
	outPixels := int64(config.Height) * int64(config.Width)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.ImageToImageWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
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
	sess.LatencyScore = took.Seconds() / float64(outPixels)

	return resp.JSON200, nil
}

func processImageToVideo(ctx context.Context, params aiRequestParams, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	// HACK: Re-use worker.ImageResponse to return results
	// TODO: Refactor to return worker.VideoResponse
	imgResp := resp.(*worker.ImageResponse)

	videos := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		data, err := downloadSeg(ctx, media.Url)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(media.Url)
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data), nil, 0)
		if err != nil {
			return nil, err
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

func submitImageToVideo(ctx context.Context, params aiRequestParams, sess *AISession, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToVideoMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
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
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.ImageToVideoWithBody(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
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
		return nil, err
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = took.Seconds() / float64(outPixels)

	return &res, nil
}

func processUpscale(ctx context.Context, params aiRequestParams, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	imgResp := resp.(*worker.ImageResponse)

	newMedia := make([]worker.Media, len(imgResp.Images))
	for i, media := range imgResp.Images {
		var data bytes.Buffer
		writer := bufio.NewWriter(&data)
		if err := worker.ReadImageB64DataUrl(media.Url, writer); err != nil {
			return nil, err
		}
		writer.Flush()

		name := string(core.RandomManifestID()) + ".png"
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data.Bytes()), nil, 0)
		if err != nil {
			return nil, err
		}

		newMedia[i] = worker.Media{Nsfw: media.Nsfw, Seed: media.Seed, Url: newUrl}
	}

	imgResp.Images = newMedia

	return imgResp, nil
}

func submitUpscale(ctx context.Context, params aiRequestParams, sess *AISession, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewUpscaleMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(imageRdr)
	if err != nil {
		return nil, err
	}
	outPixels := int64(config.Height) * int64(config.Width)

	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.UpscaleWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
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
	sess.LatencyScore = took.Seconds() / float64(outPixels)

	return resp.JSON200, nil
}

func submitSpeechToText(ctx context.Context, params aiRequestParams, sess *AISession, req worker.SpeechToTextMultipartRequestBody) (*worker.TextResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewSpeechToTextMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(sess.Transcoder(), worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	outPixels, err := common.CalculateAudioDuration(req.Audio)
	if err != nil {
		return nil, err
	}
	outPixels *= 1000 // Convert to milliseconds
	setHeaders, balUpdate, err := prepareAIPayment(ctx, sess, outPixels)
	if err != nil {
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	start := time.Now()
	resp, err := client.SpeechToTextWithBody(ctx, mw.FormDataContentType(), &buf, setHeaders)
	took := time.Since(start)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
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
		return nil, err
	}

	// TODO: Refine this rough estimate in future iterations
	sess.LatencyScore = took.Seconds() / float64(outPixels)

	return &res, nil
}

func processSpeechToText(ctx context.Context, params aiRequestParams, req worker.SpeechToTextMultipartRequestBody) (*worker.TextResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	txtResp := resp.(*worker.TextResponse)

	return txtResp, nil
}

func processAIRequest(ctx context.Context, params aiRequestParams, req interface{}) (interface{}, error) {
	var cap core.Capability
	var modelID string
	var submitFn func(context.Context, aiRequestParams, *AISession) (interface{}, error)

	switch v := req.(type) {
	case worker.TextToImageJSONRequestBody:
		cap = core.Capability_TextToImage
		modelID = defaultTextToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitTextToImage(ctx, params, sess, v)
		}
	case worker.ImageToImageMultipartRequestBody:
		cap = core.Capability_ImageToImage
		modelID = defaultImageToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitImageToImage(ctx, params, sess, v)
		}
	case worker.ImageToVideoMultipartRequestBody:
		cap = core.Capability_ImageToVideo
		modelID = defaultImageToVideoModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		// Assuming submitImageToVideo returns a VideoResponse
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitImageToVideo(ctx, params, sess, v)
		}
	case worker.UpscaleMultipartRequestBody:
		cap = core.Capability_Upscale
		modelID = defaultUpscaleModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitUpscale(ctx, params, sess, v)
		}
	case worker.SpeechToTextMultipartRequestBody:
		cap = core.Capability_SpeechToText
		modelID = defaultSpeechToTextModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (interface{}, error) {
			return submitSpeechToText(ctx, params, sess, v)
		}
	// Add more cases as needed...
	default:
		return nil, fmt.Errorf("unsupported request type %T", req)
	}

	//var resp *worker.ImageResponse
	var resp interface{}

	cctx, cancel := context.WithTimeout(ctx, processingRetryTimeout)
	defer cancel()

	tries := 0
	for {
		select {
		case <-cctx.Done():
			return nil, &ServiceUnavailableError{err: fmt.Errorf("no orchestrators available within %v timeout", processingRetryTimeout)}
		default:
		}

		tries++
		sess, err := params.sessManager.Select(ctx, cap, modelID)
		if err != nil {
			clog.Infof(ctx, "Error selecting session cap=%v modelID=%v err=%v", cap, modelID, err)
			continue
		}

		if sess == nil {
			break
		}

		resp, err = submitFn(ctx, params, sess)
		if err == nil {
			params.sessManager.Complete(ctx, sess)
			break
		}
		if errors.Is(err, common.ErrorCalculatingDuration) || errors.Is(err, common.ErrUnsupportedFormat) {
			return nil, &BadRequestError{err}
		}

		clog.Infof(ctx, "Error submitting request cap=%v modelID=%v try=%v orch=%v err=%v", cap, modelID, tries, sess.Transcoder(), err)
		params.sessManager.Remove(ctx, sess)
	}

	if resp == nil {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}
	return resp.(interface{}), nil
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
		return nil, nil, err
	}

	// As soon as the request is sent to the orch consider the balance update's credit as spent
	balUpdate.Status = CreditSpent

	setHeaders := func(_ context.Context, req *http.Request) error {
		req.Header.Set(segmentHeader, segCreds)
		req.Header.Set(paymentHeader, payment)
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
