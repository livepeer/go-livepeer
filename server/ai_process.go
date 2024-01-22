package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
)

const imageToVideoTimeout = 5 * time.Minute

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
	return submitTextToImage(ctx, orchUrl, req)
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
	return submitImageToImage(ctx, orchUrl, req)
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

	return resp.JSON200, nil
}

func processImageToVideo(ctx context.Context, params aiRequestParams, req worker.ImageToVideoMultipartRequestBody) ([]string, error) {
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
	urls, err := submitImageToVideo(ctx, orchUrl, req)
	if err != nil {
		return nil, err
	}

	newUrls := make([]string, len(urls))
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

		newUrls[i] = newUrl
	}

	return newUrls, nil
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
