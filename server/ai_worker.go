package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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
			glog.Info("Terminating aiworker because of ", err)
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
	glog.Infof("runAIWorker orchAddr=%v ", orchAddr)

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
			runAIWork(n, orchAddr, httpc, notify)
			wg.Done()
		}()

	}
}

func runAIWork(n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifyAIJob) {
	glog.Infof("Processing AI job taskID=%d url=%s", notify.TaskId, notify.Url)

	var contentType, fname string
	var body bytes.Buffer
	var tData *core.TranscodeData

	//TODO: should we pass through the request ID or any other request information? Or is TaskID sufficient to trace back to O?
	//ctx := clog.AddManifestID(context.Background(), string(md.ManifestID))
	//if md.AuthToken != nil {
	//	ctx = clog.AddOrchSessionID(ctx, md.AuthToken.SessionId)
	//}
	//ctx = clog.AddSeqNo(ctx, uint64(md.Seq))
	ctx := clog.AddVal(context.Background(), "taskId", strconv.FormatInt(notify.TaskId, 10))
	if !n.CheckWorkerAICapability(notify.Pipeline, notify.Url) {
		clog.Errorf(ctx, "Requested capabilities for AI job are not compatible with this node taskId=%d url=%s pipeline=%s modelID=%s err=%q", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, errCapabilities)
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, errCapabilities)
		return
	}

	var params interface{}
	err := json.Unmarshal(notify.RequestData, &params)
	if err != nil {
		glog.Errorf("Unable to parse requestData taskId=%d url=%s pipeline=%s modelID=%s err=%q", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, err)
		sendAIResult(context.Background(), n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
		// TODO short-circuit error handling
		// See https://github.com/livepeer/go-livepeer/issues/1518
	}

	//download the input file if applicable
	var input []byte
	if notify.Url != "" {
		input, err = core.GetSegmentData(ctx, notify.Url)
		if err != nil {
			clog.Errorf(ctx, "AI Worker cannot get input file from taskId=%d url=%s err=%q", notify.TaskId, notify.Url, err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
			return
		}

		// Write it to disk
		if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
			err = os.Mkdir(n.WorkDir, 0700)
			if err != nil {
				clog.Errorf(ctx, "AI Worker cannot create workdir err=%q", err)
				sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
				return
			}
		}
		// Create input file from segment. Removed after transcoding done
		fname = path.Join(n.WorkDir, common.RandName()+".tempfile")
		if err = os.WriteFile(fname, input, 0600); err != nil {
			clog.Errorf(ctx, "AI Worker cannot write file err=%q", err)
			sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
			return
		}
		defer os.Remove(fname)
		clog.V(common.DEBUG).Infof(ctx, "AI job input file from taskId=%d url=%s saved to file=%s", notify.TaskId, notify.Url, fname)

	}

	start := time.Now()
	var resp *worker.ImageResponse
	var respVid *worker.VideoResponse

	switch params.(type) {
	case worker.TextToImageJSONRequestBody:
		req := params.(worker.TextToImageJSONRequestBody)
		resp, err = n.AIWorker.TextToImage(ctx, req)
	case worker.ImageToImageMultipartRequestBody:
		req := params.(worker.ImageToImageMultipartRequestBody)
		req.Image.InitFromBytes(input, "image")
		resp, err = n.AIWorker.ImageToImage(ctx, req)
	case worker.ImageToVideoMultipartRequestBody:
		req := params.(worker.ImageToVideoMultipartRequestBody)
		respVid, err = n.AIWorker.ImageToVideo(ctx, req)
	case worker.UpscaleMultipartRequestBody:
		req := params.(worker.UpscaleMultipartRequestBody)
		resp, err = n.AIWorker.Upscale(ctx, req)
	default:
		resp = nil
		err = errors.New("AI request pipeline type not supported")
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
	}

	clog.V(common.VERBOSE).InfofErr(ctx, "AI job processing done for taskId=%d url=%s pipeline=%s modelID=%s dur=%v", notify.TaskId, notify.Url, notify.Pipeline, notify.ModelID, time.Since(start), err)
	if err != nil {
		if _, ok := err.(core.UnrecoverableError); ok {
			defer panic(err)
		}
		sendAIResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
	}

	boundary := common.RandName()
	w := multipart.NewWriter(&body)
	for i, v := range tData.Segments {
		ctyp, err := common.ProfileFormatMimeType(profiles[i].Format)
		if err != nil {
			clog.Errorf(ctx, "Could not find mime type err=%q", err)
			continue
		}
		w.SetBoundary(boundary)
		hdrs := textproto.MIMEHeader{
			"Content-Type":   {ctyp},
			"Content-Length": {strconv.Itoa(len(v.Data))},
			"Pixels":         {strconv.FormatInt(v.Pixels, 10)},
		}
		fw, err := w.CreatePart(hdrs)
		if err != nil {
			clog.Errorf(ctx, "Could not create multipart part err=%q", err)
		}
		io.Copy(fw, bytes.NewBuffer(v.Data))
		// Add perceptual hash data as a part if generated
		if md.CalcPerceptualHash {
			w.SetBoundary(boundary)
			hdrs := textproto.MIMEHeader{
				"Content-Type":   {"application/octet-stream"},
				"Content-Length": {strconv.Itoa(len(v.PHash))},
			}
			fw, err := w.CreatePart(hdrs)
			if err != nil {
				clog.Errorf(ctx, "Could not create multipart part err=%q", err)
			}
			io.Copy(fw, bytes.NewBuffer(v.PHash))
		}
	}
	w.Close()
	contentType = "multipart/mixed; boundary=" + boundary
	sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
}

func sendAIResult(ctx context.Context, n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifyAIJob,
	contentType string, body *bytes.Buffer, tData *core.TranscodeData, err error,
) {
	if err != nil {
		clog.Errorf(ctx, "Unable to process AI job err=%q", err)
		body.Write([]byte(err.Error()))
		contentType = transcodingErrorMimeType
	}
	req, err := http.NewRequest("POST", "https://"+orchAddr+"/aiResults", body)
	if err != nil {
		clog.Errorf(ctx, "Error posting results to orch=%s staskId=%d url=%s err=%q", orchAddr,
			notify.TaskId, notify.Url, err)
		return
	}
	req.Header.Set("Authorization", protoVerLPT)
	req.Header.Set("Credentials", n.OrchSecret)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("TaskId", strconv.FormatInt(notify.TaskId, 10))

	pixels := int64(0)
	if tData != nil {
		pixels = tData.Pixels
	}
	req.Header.Set("Pixels", strconv.FormatInt(pixels, 10))
	uploadStart := time.Now()
	resp, err := httpc.Do(req)
	if err != nil {
		clog.Errorf(ctx, "Error submitting results err=%q", err)
	} else {
		rbody, rerr := ioutil.ReadAll(resp.Body)
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
	clog.V(common.VERBOSE).InfofErr(ctx, "Transcoding done results sent for taskId=%d url=%s uploadDur=%v", notify.TaskId, notify.Url, uploadDur, err)

	if monitor.Enabled {
		monitor.SegmentUploaded(ctx, 0, uint64(notify.TaskId), uploadDur, "")
	}
}
