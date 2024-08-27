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
	"path"
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

var downloadInputFile = core.GetSegmentData

// Orchestrator gRPC
func (h *lphttp) RegisterAIWorker(req *net.RegisterAIWorkerRequest, stream net.AIWorker_RegisterAIWorkerServer) error {
	from := common.GetConnectionAddr(stream.Context())
	glog.Infof("Got a RegisterAIWorker request from aiworker=%s ", from)

	if req.Secret != h.orchestrator.TranscoderSecret() {
		glog.Errorf("err=%q", errSecret.Error())
		return errSecret
	}
	if req.Capacity <= 0 {
		glog.Errorf("err=%q", errZeroCapacity.Error())
		return errZeroCapacity
	}
	// handle case of legacy Transcoder which do not advertise capabilities
	if req.Capabilities == nil {
		req.Capabilities = core.NewCapabilities(core.DefaultCapabilities(), nil).ToNetCapabilities()
	}
	// blocks until stream is finished
	h.orchestrator.ServeAIWorker(stream, int(req.Capacity), req.Capabilities)
	return nil
}

// Standalone AIWorker

// RunAIWorker is main routing of standalone aiworker
// Exiting it will terminate executable
func RunAIWorker(n *core.LivepeerNode, orchAddr string, capacity int, caps *net.Capabilities) {
	expb := backoff.NewExponentialBackOff()
	expb.MaxInterval = time.Minute
	expb.MaxElapsedTime = 0
	backoff.Retry(func() error {
		glog.Info("Registering AI worker to ", orchAddr)
		err := runAIWorker(n, orchAddr, capacity, caps)
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

func runAIWorker(n *core.LivepeerNode, orchAddr string, capacity int, caps *net.Capabilities) error {
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
	r, err := c.RegisterAIWorker(ctx, &net.RegisterAIWorkerRequest{Secret: n.OrchSecret, Capacity: int64(capacity),
		Capabilities: caps})
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

func runAIJob(n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifyAIJob) {
	glog.Infof("Processing AI job taskID=%d pipeline=%s modelID=%s input_url=%s", notify.TaskId, notify.Pipeline, notify.ModelID, notify.Url)

	var contentType, fname string
	var body bytes.Buffer
	var addlResultData interface{}

	//TODO: consider adding additional information to context for tracing back to Orchestrator and debugging

	ctx := clog.AddVal(context.Background(), "taskId", strconv.FormatInt(notify.TaskId, 10))

	//reserve the capabilities to process this request, release after work is done
	err := n.ReserveAICapability(notify.Pipeline, notify.ModelID)
	if err != nil {
		clog.Errorf(ctx, "No capability avaiable to process requested AI job with this node taskId=%d url=%s pipeline=%s modelID=%s err=%q", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, core.ErrNoCompatibleWorkersAvailable)
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, core.ErrNoCompatibleWorkersAvailable)
		return
	}
	defer n.ReleaseAICapability(notify.Pipeline, notify.ModelID)

	if err != nil {
		glog.Errorf("Unable to parse requestData taskId=%d url=%s pipeline=%s modelID=%s err=%q", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, err)
		sendAIResult(context.Background(), n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
		return
		// TODO short-circuit error handling
		// See https://github.com/livepeer/go-livepeer/issues/1518
	}

	//check working directory exists, create if not
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err = os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			clog.Errorf(ctx, "AI Worker cannot create workdir err=%q", err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
			return
		}
	}

	//download the input file if applicable
	var input []byte
	if notify.Url != "" {
		input, err = downloadInputFile(ctx, notify.Url)
		if err != nil {
			clog.Errorf(ctx, "AI Worker cannot get input file from taskId=%d url=%s err=%q", notify.TaskId, notify.Url, err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
			return
		}

		// Write it to disk
		// Create input file from segment. Removed after transcoding done
		fname = path.Join(n.WorkDir, common.RandName()+".tempfile")
		if err = os.WriteFile(fname, input, 0600); err != nil {
			clog.Errorf(ctx, "AI Worker cannot write file err=%q", err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
			return
		}
		defer os.Remove(fname)
		clog.V(common.DEBUG).Infof(ctx, "AI job input file from taskId=%d url=%s saved to file=%s", notify.TaskId, notify.Url, fname)
	}

	start := time.Now()
	var resp interface{} //this is used for video as well because Frames received are transcoded to an MP4
	var resultType string
	reqOk := true

	switch notify.Pipeline {
	case "text-to-image":
		var req worker.TextToImageJSONRequestBody
		err = json.Unmarshal(notify.RequestData, &req)
		if err != nil {
			reqOk = false
		}
		resultType = "image/png"
		resp, err = n.TextToImage(ctx, req)
	case "image-to-image":
		var req worker.ImageToImageMultipartRequestBody
		err = json.Unmarshal(notify.RequestData, &req)
		if err != nil {
			reqOk = false
		}
		resultType = "image/png"
		req.Image.InitFromBytes(input, "image")
		resp, err = n.ImageToImage(ctx, req)
	case "upscale":
		var req worker.UpscaleMultipartRequestBody
		err = json.Unmarshal(notify.RequestData, &req)
		if err != nil {
			reqOk = false
		}
		resultType = "image/png"
		req.Image.InitFromBytes(input, "image")
		resp, err = n.Upscale(ctx, req)
	case "image-to-video":
		var req worker.ImageToVideoMultipartRequestBody
		err = json.Unmarshal(notify.RequestData, &req)
		if err != nil {
			reqOk = false
		}
		resultType = "video/mp4"
		req.Image.InitFromBytes(input, "image")
		resp, err = n.ImageToVideo(ctx, req)
	case "audio-to-text":
		var req worker.AudioToTextMultipartRequestBody
		err = json.Unmarshal(notify.RequestData, &req)
		if err != nil {
			reqOk = false
		}
		resultType = "application/json"
		req.Audio.InitFromBytes(input, "audio")
		resp, err = n.AudioToText(ctx, req)
	default:
		resp = nil
		err = errors.New("AI request pipeline type not supported")
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
		return
	}

	if !reqOk {
		resp = nil
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
		return
	}

	clog.V(common.VERBOSE).InfofErr(ctx, "AI job processing done for taskId=%d url=%s pipeline=%s modelID=%s dur=%v", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, time.Since(start), err)
	if err != nil {
		if _, ok := err.(core.UnrecoverableError); ok {
			defer panic(err)
		}
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
		return
	}

	boundary := common.RandName()
	w := multipart.NewWriter(&body)

	if resp != nil {
		//create the multipart/mixed response to send to Orchestrator

		//Parse data from runner to send back to orchestrator
		//  ***-to-image gets base64 encoded string of binary image from runner
		//  image-to-video processes frames from runner and returns ImageResponse with url to local file
		imgResp, isImg := resp.(*worker.ImageResponse)
		if isImg {
			var imgBuf bytes.Buffer
			for i, image := range imgResp.Images {
				//read the data to binary and replace the url
				length := 0
				switch resultType {
				case "image/png":
					err := worker.ReadImageB64DataUrl(image.Url, &imgBuf)
					if err != nil {
						clog.Errorf(ctx, "AI Worker failed to save image from data url err=%q", err)
						sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, err)
						return
					}
					length = imgBuf.Len()
					imgResp.Images[i].Url = fmt.Sprintf("%v.png", core.RandomManifestID()) //update json response to track filename attached

					//create the part
					w.SetBoundary(boundary)
					hdrs := textproto.MIMEHeader{
						"Content-Type":        {resultType},
						"Content-Length":      {strconv.Itoa(length)},
						"Content-Disposition": {"attachment; filename=" + imgResp.Images[i].Url},
					}
					fw, err := w.CreatePart(hdrs)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, nil, addlResultData, err)
						return
					}
					io.Copy(fw, &imgBuf)
					imgBuf.Reset()
				case "video/mp4":
					//transcoded result is saved as local file  TODO: enhance this to return the []bytes from transcoding in n.ImageToVideo
					//create the part
					f, err := os.ReadFile(image.Url)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, nil, addlResultData, err)
						return
					}
					w.SetBoundary(boundary)
					hdrs := textproto.MIMEHeader{
						"Content-Type":        {resultType},
						"Content-Length":      {strconv.Itoa(len(f))},
						"Content-Disposition": {"attachment; filename=" + imgResp.Images[i].Url},
					}
					fw, err := w.CreatePart(hdrs)
					if err != nil {
						clog.Errorf(ctx, "Could not create multipart part err=%q", err)
						sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, nil, addlResultData, err)
						return
					}
					io.Copy(fw, bytes.NewBuffer(f))
				}
			}
			//update resp for image.Url updates
			resp = imgResp
		}

		//add the json to the response
		//   audio-to-text has no file attachment because the response is json
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			clog.Errorf(ctx, "Could not marshal json response err=%q", err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, nil, addlResultData, err)
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
	sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, addlResultData, nil)
}

func sendAIResult(ctx context.Context, n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifyAIJob,
	contentType string, body *bytes.Buffer, addlData interface{}, err error,
) {
	glog.Infof("sending results back to Orchestrator")
	if err != nil {
		clog.Errorf(ctx, "Unable to process AI job err=%q", err)
		body.Write([]byte(err.Error()))
		contentType = aiWorkerErrorMimeType
	}
	req, err := http.NewRequest("POST", "https://"+orchAddr+"/aiResults", body)
	if err != nil {
		clog.Errorf(ctx, "Error posting results to orch=%s staskId=%d url=%s err=%q", orchAddr,
			notify.TaskId, notify.Url, err)
		return
	}
	req.Header.Set("Authorization", protoVerAIWorker)
	req.Header.Set("Credentials", n.OrchSecret)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("TaskId", strconv.FormatInt(notify.TaskId, 10))

	//TODO consider adding additional information in response header from the addlData field (e.g. transcoding includes Pixels)

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
	clog.V(common.VERBOSE).InfofErr(ctx, "AI job processing done results sent for taskId=%d pipeline=%s modelID=%s uploadDur=%v", notify.TaskId, notify.Pipeline, notify.ModelID, uploadDur, err)

	if monitor.Enabled {
		monitor.AIResultUploaded(ctx, uploadDur, notify.Pipeline, notify.ModelID, orchAddr)
	}
}
