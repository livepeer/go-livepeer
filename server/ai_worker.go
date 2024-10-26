package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/glog"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const protoVerAIWorker = "Livepeer-AI-Worker-1.0"
const aiWorkerErrorMimeType = "livepeer/ai-worker-error"

// Orchestrator gRPC
func (h *lphttp) RegisterAIWorker(req *net.RegisterAIWorkerRequest, stream net.AIWorker_RegisterAIWorkerServer) error {
	from := common.GetConnectionAddr(stream.Context())
	glog.Infof("Got a RegisterAIWorker request from aiworker=%s ", from)

	if req.Secret != h.orchestrator.TranscoderSecret() {
		glog.Errorf("err=%q", errSecret.Error())
		return errSecret
	}
	// handle case of legacy Transcoder which do not advertise capabilities
	if req.Capabilities == nil {
		req.Capabilities = core.NewCapabilities(core.DefaultCapabilities(), nil).ToNetCapabilities()
	}
	// blocks until stream is finished
	h.orchestrator.ServeAIWorker(stream, req.Capabilities)
	return nil
}

// Standalone AIWorker

// RunAIWorker is main routing of standalone aiworker
// Exiting it will terminate executable
func RunAIWorker(n *core.LivepeerNode, orchAddr string, caps *net.Capabilities) {
	expb := backoff.NewExponentialBackOff()
	expb.MaxInterval = time.Minute
	expb.MaxElapsedTime = 0
	backoff.Retry(func() error {
		glog.Info("Registering AI worker to ", orchAddr)
		err := runAIWorker(n, orchAddr, caps)
		glog.Info("Unregistering AI worker: ", err)
		if _, fatal := err.(core.RemoteAIWorkerFatalError); fatal {
			glog.Info("Terminating AI Worker because of ", err)
			// Returning nil here will make `backoff` to stop trying to reconnect and exit
			return nil
		}
		// By returning error we tell `backoff` to try to connect again
		return err
	}, expb)
}

func checkAIWorkerError(err error) error {
	if err != nil {
		s := status.Convert(err)
		if s.Message() == errSecret.Error() { // consider this unrecoverable
			return core.NewRemoteAIWorkerFatalError(errSecret)
		}
		if s.Message() == errZeroCapacity.Error() { // consider this unrecoverable
			return core.NewRemoteAIWorkerFatalError(errZeroCapacity)
		}
		if status.Code(err) == codes.Canceled {
			return core.NewRemoteAIWorkerFatalError(errInterrupted)
		}
	}
	return err
}

func runAIWorker(n *core.LivepeerNode, orchAddr string, caps *net.Capabilities) error {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	conn, err := grpc.Dial(orchAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		glog.Error("Did not connect AI worker to orchesrator: ", err)
		return err
	}
	defer conn.Close()

	c := net.NewAIWorkerClient(conn)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	// Silence linter
	defer cancel()
	r, err := c.RegisterAIWorker(ctx, &net.RegisterAIWorkerRequest{Secret: n.OrchSecret, Capabilities: caps})
	if err := checkAIWorkerError(err); err != nil {
		glog.Error("Could not register aiworker to orchestrator ", err)
		return err
	}

	// Catch interrupt signal to shut down transcoder
	exitc := make(chan os.Signal)
	signal.Notify(exitc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(exitc)
	go func() {
		select {
		case sig := <-exitc:
			glog.Infof("Exiting Livepeer AIWorker: %v", sig)
			// Cancelling context will close connection to orchestrator
			cancel()
			return
		}
	}()

	httpc := &http.Client{Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var wg sync.WaitGroup
	for {
		notify, err := r.Recv()
		if err := checkAIWorkerError(err); err != nil {
			glog.Infof(`End of stream receive cycle because of err=%q, waiting for running aiworker jobs to complete`, err)
			wg.Wait()
			return err
		}
		wg.Add(1)
		go func() {
			runAIJob(n, orchAddr, httpc, notify)
			wg.Done()
		}()
	}
}

type AIJobRequestData struct {
	InputUrl string          `json:"input_url"`
	Request  json.RawMessage `json:"request"`
}

func runAIJob(n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifyAIJob) {
	var contentType string
	var body bytes.Buffer

	ctx := clog.AddVal(context.Background(), "taskId", strconv.FormatInt(notify.TaskId, 10))
	clog.Infof(ctx, "Received AI job, validating request")

	var processFn func(context.Context) (interface{}, error)
	var resp interface{} // this is used for video as well because Frames received are transcoded to an MP4
	var err error
	var resultType string
	var reqOk bool
	var modelID string
	var input []byte

	start := time.Now()
	var reqData AIJobRequestData
	err = json.Unmarshal(notify.AIJobData.RequestData, &reqData)
	if err != nil {
		sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, err)
		return
	}

	switch notify.AIJobData.Pipeline {
	case "text-to-image":
		var req worker.GenTextToImageJSONRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		modelID = *req.ModelId
		resultType = "image/png"
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.TextToImage(ctx, req)
		}
		reqOk = true
	case "image-to-image":
		var req worker.GenImageToImageMultipartRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		input, err = core.DownloadData(ctx, reqData.InputUrl)
		if err != nil {
			break
		}
		modelID = *req.ModelId
		resultType = "image/png"
		req.Image.InitFromBytes(input, "image")
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.ImageToImage(ctx, req)
		}
		reqOk = true
	case "upscale":
		var req worker.GenUpscaleMultipartRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		input, err = core.DownloadData(ctx, reqData.InputUrl)
		if err != nil {
			break
		}
		modelID = *req.ModelId
		resultType = "image/png"
		req.Image.InitFromBytes(input, "image")
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.Upscale(ctx, req)
		}
		reqOk = true
	case "image-to-video":
		var req worker.GenImageToVideoMultipartRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		input, err = core.DownloadData(ctx, reqData.InputUrl)
		if err != nil {
			break
		}
		modelID = *req.ModelId
		resultType = "video/mp4"
		req.Image.InitFromBytes(input, "image")
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.ImageToVideo(ctx, req)
		}
		reqOk = true
	case "audio-to-text":
		var req worker.GenAudioToTextMultipartRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		input, err = core.DownloadData(ctx, reqData.InputUrl)
		if err != nil {
			break
		}
		modelID = *req.ModelId
		resultType = "application/json"
		req.Audio.InitFromBytes(input, "audio")
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.AudioToText(ctx, req)
		}
		reqOk = true
	case "segment-anything-2":
		var req worker.GenSegmentAnything2MultipartRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		input, err = core.DownloadData(ctx, reqData.InputUrl)
		if err != nil {
			break
		}
		modelID = *req.ModelId
		resultType = "application/json"
		req.Image.InitFromBytes(input, "image")
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.SegmentAnything2(ctx, req)
		}
		reqOk = true
	case "llm":
		var req worker.GenLLMFormdataRequestBody
		err = json.Unmarshal(reqData.Request, &req)
		if err != nil || req.ModelId == nil {
			break
		}
		modelID = *req.ModelId
		resultType = "application/json"
		if req.Stream != nil && *req.Stream {
			resultType = "text/event-stream"
		}
		processFn = func(ctx context.Context) (interface{}, error) {
			return n.LLM(ctx, req)
		}
		reqOk = true
	default:
		err = errors.New("AI request pipeline type not supported")
	}

	if !reqOk {
		resp = nil
		err = fmt.Errorf("AI request validation failed for %v pipeline err=%v", notify.AIJobData.Pipeline, err)
		sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, err)
		return
	}

	// process the request
	clog.Infof(ctx, "Processing AI job pipeline=%s modelID=%s", notify.AIJobData.Pipeline, modelID)

	// reserve the capabilities to process this request, release after work is done
	err = n.ReserveAICapability(notify.AIJobData.Pipeline, modelID)
	if err != nil {
		clog.Errorf(ctx, "No capability avaiable to process requested AI job with this node taskId=%d pipeline=%s modelID=%s err=%q", notify.TaskId, notify.AIJobData.Pipeline, modelID, core.ErrNoCompatibleWorkersAvailable)
		sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, core.ErrNoCompatibleWorkersAvailable)
		return
	}

	// do the work and release the GPU for next job
	resp, err = processFn(ctx)
	n.ReleaseAICapability(notify.AIJobData.Pipeline, modelID)

	clog.V(common.VERBOSE).InfofErr(ctx, "AI job processing done for taskId=%d pipeline=%s modelID=%s dur=%v", notify.TaskId, notify.AIJobData.Pipeline, modelID, time.Since(start), err)
	if err != nil {
		if _, ok := err.(core.UnrecoverableError); ok {
			defer panic(err)
		}
		sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, err)
		return
	}

	boundary := common.RandName()
	w := multipart.NewWriter(&body)

	if resp != nil {
		if resultType == "text/event-stream" {
			streamChan, ok := resp.(<-chan worker.LlmStreamChunk)
			if ok {
				sendStreamingAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, httpc, resultType, streamChan)
				return
			} else {
				sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, fmt.Errorf("streaming not supported"))
				return
			}
		}

		// create the multipart/mixed response to send to Orchestrator
		// Parse data from runner to send back to orchestrator
		//   ***-to-image gets base64 encoded string of binary image from runner
		//   image-to-video processes frames from runner and returns ImageResponse with url to local file
		imgResp, isImg := resp.(*worker.ImageResponse)
		if isImg {
			var imgBuf bytes.Buffer
			for i, image := range imgResp.Images {
				// read the data to binary and replace the url
				length := 0
				switch resultType {
				case "image/png":
					err := worker.ReadImageB64DataUrl(image.Url, &imgBuf)
					if err != nil {
						clog.Errorf(ctx, "AI Worker failed to save image from data url err=%q", err)
						sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, err)
						return
					}
					length = imgBuf.Len()
					imgResp.Images[i].Url = fmt.Sprintf("%v.png", core.RandomManifestID()) // update json response to track filename attached

					// create the part
					w.SetBoundary(boundary)
					hdrs := textproto.MIMEHeader{
						"Content-Type":        {resultType},
						"Content-Length":      {strconv.Itoa(length)},
						"Content-Disposition": {"attachment; filename=" + imgResp.Images[i].Url},
					}
					fw, err := w.CreatePart(hdrs)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, nil, err)
						return
					}
					io.Copy(fw, &imgBuf)
					imgBuf.Reset()
				case "video/mp4":
					// transcoded result is saved as local file
					// TODO: enhance this to return the []bytes from transcoding in n.ImageToVideo create the part
					f, err := os.ReadFile(image.Url)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, nil, err)
						return
					}
					defer os.Remove(image.Url)
					imgResp.Images[i].Url = fmt.Sprintf("%v.mp4", core.RandomManifestID())
					w.SetBoundary(boundary)
					hdrs := textproto.MIMEHeader{
						"Content-Type":        {resultType},
						"Content-Length":      {strconv.Itoa(len(f))},
						"Content-Disposition": {"attachment; filename=" + imgResp.Images[i].Url},
					}
					fw, err := w.CreatePart(hdrs)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, nil, err)
						return
					}
					io.Copy(fw, bytes.NewBuffer(f))
				}
			}
			// update resp for image.Url updates
			resp = imgResp
		}

		// add the json to the response
		// NOTE: audio-to-text has no file attachment because the response is json
		jsonResp, err := json.Marshal(resp)

		if err != nil {
			clog.Errorf(ctx, "Could not marshal json response err=%q", err)
			sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, nil, err)
			return
		}

		w.SetBoundary(boundary)
		hdrs := textproto.MIMEHeader{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonResp))},
		}
		fw, err := w.CreatePart(hdrs)
		if err != nil {
			clog.Errorf(ctx, "Could not create multipart part err=%q", err)
		}
		io.Copy(fw, bytes.NewBuffer(jsonResp))
	}

	w.Close()
	contentType = "multipart/mixed; boundary=" + boundary
	sendAIResult(ctx, n, orchAddr, notify.AIJobData.Pipeline, modelID, httpc, contentType, &body, nil)
}

func sendAIResult(ctx context.Context, n *core.LivepeerNode, orchAddr string, pipeline string, modelID string, httpc *http.Client,
	contentType string, body *bytes.Buffer, err error,
) {
	taskId := clog.GetVal(ctx, "taskId")
	clog.Infof(ctx, "sending results back to Orchestrator")
	if err != nil {
		clog.Errorf(ctx, "Unable to process AI job err=%q", err)
		body.Write([]byte(err.Error()))
		contentType = aiWorkerErrorMimeType
	}
	resultUrl := "https://" + orchAddr + "/aiResults"
	req, err := http.NewRequest("POST", resultUrl, body)
	if err != nil {
		clog.Errorf(ctx, "Error posting results to orch=%s taskId=%d url=%s err=%q", orchAddr,
			taskId, resultUrl, err)
		return
	}
	req.Header.Set("Authorization", protoVerAIWorker)
	req.Header.Set("Credentials", n.OrchSecret)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("TaskId", taskId)
	req.Header.Set("Pipeline", pipeline)

	// TODO consider adding additional information in response header from the addlData field (e.g. transcoding includes Pixels)

	uploadStart := time.Now()
	resp, err := httpc.Do(req)
	if err != nil {
		clog.Errorf(ctx, "Error submitting results err=%q", err)
	} else {
		rbody, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if rerr != nil {
				clog.Errorf(ctx, "Orchestrator returned HTTP statusCode=%v with unreadable body err=%q", resp.StatusCode, rerr)
			} else {
				clog.Errorf(ctx, "Orchestrator returned HTTP statusCode=%v err=%q", resp.StatusCode, string(rbody))
			}
		}
	}
	uploadDur := time.Since(uploadStart)
	clog.V(common.VERBOSE).InfofErr(ctx, "AI job processing done results sent for taskId=%d uploadDur=%v", taskId, pipeline, modelID, uploadDur, err)

	if monitor.Enabled {
		monitor.AIResultUploaded(ctx, uploadDur, pipeline, modelID, orchAddr)
	}
}

func sendStreamingAIResult(ctx context.Context, n *core.LivepeerNode, orchAddr string, pipeline string, httpc *http.Client,
	contentType string, streamChan <-chan worker.LlmStreamChunk,
) {
	clog.Infof(ctx, "sending streaming results back to Orchestrator")
	taskId := clog.GetVal(ctx, "taskId")

	pReader, pWriter := io.Pipe()
	req, err := http.NewRequest("POST", "https://"+orchAddr+"/aiResults", pReader)
	if err != nil {
		clog.Errorf(ctx, "Failed to forward stream to target URL err=%q", err)
		pWriter.CloseWithError(err)
		return
	}

	req.Header.Set("Authorization", protoVerAIWorker)
	req.Header.Set("Credentials", n.OrchSecret)
	req.Header.Set("TaskId", taskId)
	req.Header.Set("Pipeline", pipeline)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// start separate go routine to forward the streamed response
	go func() {
		fwdResp, err := httpc.Do(req)
		if err != nil {
			clog.Errorf(ctx, "Failed to forward stream to target URL err=%q", err)
			pWriter.CloseWithError(err)
			return
		}
		defer fwdResp.Body.Close()
		io.Copy(io.Discard, fwdResp.Body)
	}()

	for chunk := range streamChan {
		data, err := json.Marshal(chunk)
		if err != nil {
			clog.Errorf(ctx, "Error marshaling stream chunk: %v", err)
			continue
		}
		fmt.Fprintf(pWriter, "data: %s\n\n", data)

		if chunk.Done {
			pWriter.Close()
			clog.Infof(ctx, "streaming results finished")
			return
		}
	}
}
