package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/stream"
)

const maxProcessingRetries = 4
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

func getOrchestratorsForAIRequest(ctx context.Context, params aiRequestParams, cap core.Capability, modelID string) ([]*net.OrchestratorInfo, error) {
	// No warm constraints applied here because we don't want to filter out orchs based on warm criteria at discovery time
	// Instead, we want all orchs that support the model and then will prioritize orchs that have a warm model at selection time
	constraints := map[core.Capability]*core.Constraints{
		cap: {
			Models: map[string]*core.ModelConstraint{
				modelID: {
					Warm: false,
				},
			},
		},
	}
	caps := core.NewCapabilitiesWithConstraints(append(core.DefaultCapabilities(), cap), nil, constraints)

	// Set numOrchs to the pool size so that discovery tries to find maximum # of compatible orchs within a timeout
	numOrchs := params.node.OrchestratorPool.Size()
	orchDesc, err := params.node.OrchestratorPool.GetOrchestrators(ctx, numOrchs, newSuspender(), caps, common.ScoreAtLeast(0))
	if err != nil {
		return nil, err
	}

	orchInfos := orchDesc.GetRemoteInfos()

	// Sort and prioritize orchs that have warm model
	sort.Slice(orchInfos, func(i, j int) bool {
		iConstraints, ok := orchInfos[i].Capabilities.Constraints[uint32(cap)]
		if !ok {
			return false
		}

		iModelConstraint, ok := iConstraints.Models[modelID]
		if !ok {
			return false
		}

		// If warm i goes before j, else j goes before i
		return iModelConstraint.Warm
	})

	return orchInfos, nil
}

func processTextToImage(ctx context.Context, params aiRequestParams, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	modelID := defaultTextToImageModelID
	if req.ModelId != nil {
		modelID = *req.ModelId
	}

	var resp *worker.ImageResponse

	tries := 0
	for tries < maxProcessingRetries {
		tries++

		sess, err := params.sessManager.Select(ctx, core.Capability_TextToImage, modelID)
		if err != nil {
			clog.Infof(ctx, "Error selecting TextToImage session err=%v", err)
			continue
		}

		if sess == nil {
			break
		}

		resp, err = submitTextToImage(ctx, params, sess, req)
		if err == nil {
			params.sessManager.Complete(ctx, sess)
			break
		}

		clog.Infof(ctx, "Error submitting TextToImage request try=%v orch=%v err=%v", tries, sess.Transcoder(), err)

		params.sessManager.Remove(ctx, sess)

	}

	if resp == nil {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}

	newMedia := make([]worker.Media, len(resp.Images))
	for i, media := range resp.Images {
		var data bytes.Buffer
		if err := worker.ReadImageB64DataUrl(media.Url, bufio.NewWriter(&data)); err != nil {
			return nil, err
		}

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

	// genSegCreds expects a stream.HLSSegment so in order to reuse it here we pass a dummy object
	segCreds, err := genSegCreds(sess.BroadcastSession, &stream.HLSSegment{}, nil, false)
	if err != nil {
		return nil, err
	}

	priceInfo, err := common.RatPriceInfo(sess.OrchestratorInfo.GetPriceInfo())
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

	// If the # of inference/denoising steps becomes configurable, a possible updated formula could be height * width * steps
	// If additional parameters that influence compute cost become configurable, then the formula should be reconsidered
	outPixels := int64(*req.Height) * int64(*req.Height)
	fee, err := estimateAIFee(outPixels, priceInfo)
	if err != nil {
		return nil, err
	}

	balUpdate, err := newBalanceUpdate(sess.BroadcastSession, fee)
	if err != nil {
		return nil, err
	}
	defer completeBalanceUpdate(sess.BroadcastSession, balUpdate)

	payment, err := genPayment(ctx, sess.BroadcastSession, balUpdate.NumTickets)
	if err != nil {
		return nil, err
	}

	// As soon as the request is sent to the orch consider the balance update's credit as spent
	balUpdate.Status = CreditSpent

	setHeaders := func(_ context.Context, req *http.Request) error {
		req.Header.Set(segmentHeader, segCreds)
		req.Header.Set(paymentHeader, payment)
		return nil
	}

	resp, err := client.TextToImageWithResponse(ctx, req, setHeaders)
	if err != nil {
		return nil, err
	}

	// We treat a response as "receiving change" where the change is the difference between the credit and debit for the update
	balUpdate.Status = ReceivedChange
	if priceInfo != nil {
		balUpdate.Debit = fee
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	return resp.JSON200, nil
}

func processImageToImage(ctx context.Context, params aiRequestParams, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	modelID := defaultImageToImageModelID
	if req.ModelId != nil {
		modelID = *req.ModelId
	}

	orchInfos, err := getOrchestratorsForAIRequest(ctx, params, core.Capability_ImageToImage, modelID)
	if err != nil {
		return nil, err
	}

	if len(orchInfos) == 0 {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}

	var resp *worker.ImageResponse

	// Round robin up to maxProcessingRetries times
	orchIdx := 0
	tries := 0
	for tries < maxProcessingRetries {
		orchUrl := orchInfos[orchIdx].Transcoder

		var err error
		resp, err = submitImageToImage(ctx, orchUrl, req)
		if err == nil {
			break
		}

		clog.Infof(ctx, "Error submitting ImageToImage request try=%v orch=%v err=%v", tries, orchUrl, err)

		tries++
		orchIdx++
		// Wrap back around
		if orchIdx >= len(orchInfos) {
			orchIdx = 0
		}
	}

	if resp == nil {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}

	newMedia := make([]worker.Media, len(resp.Images))
	for i, media := range resp.Images {
		var data bytes.Buffer
		if err := worker.ReadImageB64DataUrl(media.Url, bufio.NewWriter(&data)); err != nil {
			return nil, err
		}

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

func submitImageToImage(ctx context.Context, url string, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToImageMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(url, worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	resp, err := client.ImageToImageWithBodyWithResponse(ctx, mw.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		// TODO: Replace trim newline with better error spec from O
		return nil, errors.New(strings.TrimSuffix(string(resp.Body), "\n"))
	}

	return resp.JSON200, nil
}

func processImageToVideo(ctx context.Context, params aiRequestParams, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	modelID := defaultImageToVideoModelID
	if req.ModelId != nil {
		modelID = *req.ModelId
	}

	orchInfos, err := getOrchestratorsForAIRequest(ctx, params, core.Capability_ImageToVideo, modelID)
	if err != nil {
		return nil, err
	}

	if len(orchInfos) == 0 {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
	}

	var resp *worker.ImageResponse

	// Round robin up to maxProcessingRetries times
	orchIdx := 0
	tries := 0
	for tries < maxProcessingRetries {
		orchUrl := orchInfos[orchIdx].Transcoder

		var err error
		resp, err = submitImageToVideo(ctx, orchUrl, req)
		if err == nil {
			break
		}

		clog.Infof(ctx, "Error submitting ImageToVideo request try=%v orch=%v err=%v", tries, orchUrl, err)

		tries++
		orchIdx++
		// Wrap back around
		if orchIdx >= len(orchInfos) {
			orchIdx = 0
		}
	}

	if resp == nil {
		return nil, &ServiceUnavailableError{err: errors.New("no orchestrators available")}
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

func submitImageToVideo(ctx context.Context, url string, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw, err := worker.NewImageToVideoMultipartWriter(&buf, req)
	if err != nil {
		return nil, err
	}

	client, err := worker.NewClientWithResponses(url, worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	resp, err := client.ImageToVideoWithBody(ctx, mw.FormDataContentType(), &buf)
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

	var res worker.ImageResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

func estimateAIFee(outPixels int64, priceInfo *big.Rat) (*big.Rat, error) {
	if priceInfo == nil {
		return nil, nil
	}

	fee := new(big.Rat).SetInt64(outPixels)
	fee.Mul(fee, priceInfo)

	return fee, nil
}
