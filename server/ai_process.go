package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const maxProcessingRetryDuration = 500 * time.Millisecond
const defaultTextToImageModelID = "stabilityai/sdxl-turbo"
const defaultImageToImageModelID = "stabilityai/sdxl-turbo"
const defaultImageToVideoModelID = "stabilityai/stable-video-diffusion-img2vid-xt"

type ServiceUnavailableError struct {
	err error
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

	newMedia := make([]worker.Media, len(resp.Images))
	for i, media := range resp.Images {
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

		newMedia[i] = worker.Media{Url: newUrl, Seed: media.Seed}
	}

	resp.Images = newMedia

	return resp, nil
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

	resp, err := client.TextToImageWithResponse(ctx, req, setHeaders)
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

	return resp.JSON200, nil
}

func processImageToImage(ctx context.Context, params aiRequestParams, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	newMedia := make([]worker.Media, len(resp.Images))
	for i, media := range resp.Images {
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

		newMedia[i] = worker.Media{Url: newUrl, Seed: media.Seed}
	}

	resp.Images = newMedia

	return resp, nil
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

	resp, err := client.ImageToImageWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf, setHeaders)
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

	return resp.JSON200, nil
}

func processImageToVideo(ctx context.Context, params aiRequestParams, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	resp, err := processAIRequest(ctx, params, req)
	if err != nil {
		return nil, err
	}

	// HACK: Re-use worker.ImageResponse to return results
	videos := make([]worker.Media, len(resp.Images))
	for i, media := range resp.Images {
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
			Url:  newUrl,
			Seed: media.Seed,
		}
	}

	resp.Images = videos

	return resp, nil
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

	resp, err := client.ImageToVideoWithBody(ctx, mw.FormDataContentType(), &buf, setHeaders)
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

	return &res, nil
}

func processAIRequest(ctx context.Context, params aiRequestParams, req interface{}) (*worker.ImageResponse, error) {
	var cap core.Capability
	var modelID string
	var submitFn func(context.Context, aiRequestParams, *AISession) (*worker.ImageResponse, error)

	switch v := req.(type) {
	case worker.TextToImageJSONRequestBody:
		cap = core.Capability_TextToImage
		modelID = defaultTextToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (*worker.ImageResponse, error) {
			return submitTextToImage(ctx, params, sess, v)
		}
	case worker.ImageToImageMultipartRequestBody:
		cap = core.Capability_ImageToImage
		modelID = defaultImageToImageModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (*worker.ImageResponse, error) {
			return submitImageToImage(ctx, params, sess, v)
		}
	case worker.ImageToVideoMultipartRequestBody:
		cap = core.Capability_ImageToVideo
		modelID = defaultImageToVideoModelID
		if v.ModelId != nil {
			modelID = *v.ModelId
		}
		submitFn = func(ctx context.Context, params aiRequestParams, sess *AISession) (*worker.ImageResponse, error) {
			return submitImageToVideo(ctx, params, sess, v)
		}
	default:
		return nil, errors.New("unknown AI request type")
	}

	var resp *worker.ImageResponse

	tries := 0
	retryDuration := time.Now().Add(maxProcessingRetryDuration)
	for time.Now().Before(retryDuration) {
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

		clog.Infof(ctx, "Error submitting request cap=%v modelID=%v try=%v orch=%v err=%v", cap, modelID, tries, sess.Transcoder(), err)

		params.sessManager.Remove(ctx, sess)
	}

	if resp == nil {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}

	return resp, nil
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
