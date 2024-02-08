package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-tools/drivers"
)

const imageToVideoTimeout = 5 * time.Minute
const textToImageRetryBackoff = 10 * time.Second
const imageToImageRetryBackoff = 10 * time.Second
const imageToVideoRetryBackoff = 1 * time.Minute
const maxProcessingRetries = 4

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

func processTextToImage(ctx context.Context, params aiRequestParams, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	// Discover 1 orchestrator
	// TODO: Discover multiple orchestrators
	caps := core.NewCapabilities(core.DefaultCapabilities(), nil)
	orchDesc, err := params.node.OrchestratorPool.GetOrchestrators(ctx, 1, newSuspender(), caps, common.ScoreAtLeast(0))
	if err != nil {
		return nil, err
	}
	orchInfos := orchDesc.GetRemoteInfos()

	if len(orchInfos) == 0 {
		return nil, errors.New("no orchestrators available")
	}

	orchUrl := orchInfos[0].Transcoder

	var resp *worker.ImageResponse
	op := func() error {
		var err error
		resp, err = submitTextToImage(ctx, orchUrl, req)
		return err
	}
	notify := func(err error, dur time.Duration) {
		clog.Infof(ctx, "Error submitting TextToImage request err=%v retrying after dur=%v", err, dur)
	}

	b := backoff.WithMaxRetries(backoff.NewConstantBackOff(textToImageRetryBackoff), maxProcessingRetries)
	if err := backoff.RetryNotify(op, b, notify); err != nil {
		return nil, &ServiceUnavailableError{err: err}
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
	// Discover 1 orchestrator
	// TODO: Discover multiple orchestrators
	caps := core.NewCapabilities(core.DefaultCapabilities(), nil)
	orchDesc, err := params.node.OrchestratorPool.GetOrchestrators(ctx, 1, newSuspender(), caps, common.ScoreAtLeast(0))
	if err != nil {
		return nil, err
	}
	orchInfos := orchDesc.GetRemoteInfos()

	if len(orchInfos) == 0 {
		return nil, errors.New("no orchestrators available")
	}

	orchUrl := orchInfos[0].Transcoder

	var resp *worker.ImageResponse
	op := func() error {
		var err error
		resp, err = submitImageToImage(ctx, orchUrl, req)
		return err
	}
	notify := func(err error, dur time.Duration) {
		clog.Infof(ctx, "Error submitting ImageToImage request err=%v retrying after dur=%v", err, dur)
	}

	b := backoff.WithMaxRetries(backoff.NewConstantBackOff(imageToImageRetryBackoff), maxProcessingRetries)
	if err := backoff.RetryNotify(op, b, notify); err != nil {
		return nil, &ServiceUnavailableError{err: err}
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
	// Discover 1 orchestrator
	// TODO: Discover multiple orchestrators
	caps := core.NewCapabilities(core.DefaultCapabilities(), nil)
	orchDesc, err := params.node.OrchestratorPool.GetOrchestrators(ctx, 1, newSuspender(), caps, common.ScoreAtLeast(0))
	if err != nil {
		return nil, err
	}
	orchInfos := orchDesc.GetRemoteInfos()

	if len(orchInfos) == 0 {
		return nil, errors.New("no orchestrators available")
	}

	orchUrl := orchInfos[0].Transcoder

	var resp *worker.ImageResponse
	op := func() error {
		var err error
		resp, err = submitImageToVideo(ctx, orchUrl, req)
		return err
	}
	notify := func(err error, dur time.Duration) {
		clog.Infof(ctx, "Error submitting ImageToVideo request err=%v retrying after dur=%v", err, dur)
	}

	b := backoff.WithMaxRetries(backoff.NewConstantBackOff(imageToVideoRetryBackoff), maxProcessingRetries)
	if err := backoff.RetryNotify(op, b, notify); err != nil {
		return nil, &ServiceUnavailableError{err: err}
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
