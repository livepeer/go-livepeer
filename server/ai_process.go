package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
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
	node *core.LivepeerNode
	os   drivers.OSSession
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

	orchInfos, err := getOrchestratorsForAIRequest(ctx, params, core.Capability_TextToImage, modelID)
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
		resp, err = submitTextToImage(ctx, orchUrl, req)
		if err == nil {
			break
		}

		clog.Infof(ctx, "Error submitting TextToImage request try=%v orch=%v err=%v", tries, orchUrl, err)

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

func submitTextToImage(ctx context.Context, url string, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	client, err := worker.NewClientWithResponses(url, worker.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	resp, err := client.TextToImageWithResponse(ctx, req)
	if err != nil {
		return nil, err
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