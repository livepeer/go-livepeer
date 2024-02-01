package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
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

		newMedia[i] = worker.Media{Url: newUrl}
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

	if resp.JSON422 != nil {
		// TODO: Handle JSON422 struct
		return nil, errors.New("orchestrator returned 422")
	}

	if resp.JSON200 == nil {
		return nil, errors.New("orchestrator did not return a response")
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

		newMedia[i] = worker.Media{Url: newUrl}
	}

	resp.Images = newMedia

	return resp, nil
}

func submitImageToImage(ctx context.Context, url string, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if err := mw.WriteField("prompt", req.Prompt); err != nil {
		return nil, err
	}
	if err := mw.WriteField("model_id", *req.ModelId); err != nil {
		return nil, err
	}

	if err := mw.Close(); err != nil {
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

	if resp.JSON422 != nil {
		// TODO: Handle JSON422 struct
		return nil, errors.New("orchestrator returned 422")
	}

	if resp.JSON200 == nil {
		return nil, errors.New("orchestrator did not return a response")
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

	var urls []string
	op := func() error {
		var err error
		urls, err = submitImageToVideo(ctx, orchUrl, req)
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
	videos := make([]worker.Media, len(urls))
	for i, url := range urls {
		data, err := downloadSeg(ctx, url)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(url)
		newUrl, err := params.os.SaveData(ctx, name, bytes.NewReader(data), nil, 0)
		if err != nil {
			return nil, err
		}

		videos[i] = worker.Media{
			Url: newUrl,
		}
	}

	resp := &worker.ImageResponse{Images: videos}
	return resp, nil
}

func submitImageToVideo(ctx context.Context, url string, req worker.ImageToVideoMultipartRequestBody) ([]string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	writer, err := mw.CreateFormFile("image", req.Image.Filename())
	if err != nil {
		return nil, err
	}
	imageSize := req.Image.FileSize()
	imageRdr, err := req.Image.Reader()
	if err != nil {
		return nil, err
	}
	copied, err := io.Copy(writer, imageRdr)
	if err != nil {
		return nil, err
	}
	if copied != imageSize {
		return nil, fmt.Errorf("failed to copy image to multipart request imageBytes=%v copiedBytes=%v", imageSize, copied)
	}

	if err := mw.WriteField("model_id", *req.ModelId); err != nil {
		return nil, err
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	r, err := http.NewRequestWithContext(ctx, "POST", url+"/image-to-video", &buf)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := sendReqWithTimeout(r, imageToVideoTimeout)
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

	var tr net.TranscodeResult
	if err := proto.Unmarshal(data, &tr); err != nil {
		return nil, err
	}

	var tdata *net.TranscodeData
	switch res := tr.Result.(type) {
	case *net.TranscodeResult_Error:
		return nil, errors.New(res.Error)
	case *net.TranscodeResult_Data:
		tdata = res.Data
	default:
		return nil, errors.New("UnknownResponse")
	}

	urls := make([]string, len(tdata.Segments))
	for i, seg := range tdata.Segments {
		urls[i] = seg.Url
	}

	return urls, nil
}
