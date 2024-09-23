package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
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
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
)

const protoVerLPT = "Livepeer-Transcoder-1.0"
const transcodingErrorMimeType = "livepeer/transcoding-error"

var errSecret = errors.New("invalid secret")
var errZeroCapacity = errors.New("zero capacity")
var errInterrupted = errors.New("execution interrupted")
var errCapabilities = errors.New("incompatible segment capabilities")

// Standalone Transcoder

// RunTranscoder is main routing of standalone transcoder
// Exiting it will terminate executable
func RunTranscoder(n *core.LivepeerNode, orchAddr string, capacity int, caps []core.Capability) {
	expb := backoff.NewExponentialBackOff()
	expb.MaxInterval = time.Minute
	expb.MaxElapsedTime = 0
	backoff.Retry(func() error {
		glog.Info("Registering transcoder to ", orchAddr)
		err := runTranscoder(n, orchAddr, capacity, caps)
		glog.Info("Unregistering transcoder: ", err)
		if _, fatal := err.(core.RemoteTranscoderFatalError); fatal {
			glog.Info("Terminating transcoder because of ", err)
			// Returning nil here will make `backoff` to stop trying to reconnect and exit
			return nil
		}
		// By returning error we tell `backoff` to try to connect again
		return err
	}, expb)
}

func checkTranscoderError(err error) error {
	if err != nil {
		s := status.Convert(err)
		if s.Message() == errSecret.Error() { // consider this unrecoverable
			return core.NewRemoteTranscoderFatalError(errSecret)
		}
		if s.Message() == errZeroCapacity.Error() { // consider this unrecoverable
			return core.NewRemoteTranscoderFatalError(errZeroCapacity)
		}
		if status.Code(err) == codes.Canceled {
			return core.NewRemoteTranscoderFatalError(errInterrupted)
		}
	}
	return err
}

func runTranscoder(n *core.LivepeerNode, orchAddr string, capacity int, caps []core.Capability) error {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	conn, err := grpc.Dial(orchAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		glog.Error("Did not connect transcoder to orchesrator: ", err)
		return err
	}
	defer conn.Close()

	c := net.NewTranscoderClient(conn)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	// Silence linter
	defer cancel()
	r, err := c.RegisterTranscoder(ctx, &net.RegisterRequest{Secret: n.OrchSecret, Capacity: int64(capacity),
		Capabilities: core.NewCapabilities(caps, []core.Capability{}).ToNetCapabilities()})
	if err := checkTranscoderError(err); err != nil {
		glog.Error("Could not register transcoder to orchestrator ", err)
		return err
	}

	// Catch interrupt signal to shut down transcoder
	exitc := make(chan os.Signal)
	signal.Notify(exitc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(exitc)
	go func() {
		select {
		case sig := <-exitc:
			glog.Infof("Exiting Livepeer Transcoder: %v", sig)
			// Cancelling context will close connection to orchestrator
			cancel()
			return
		}
	}()

	httpc := &http.Client{Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var wg sync.WaitGroup
	for {
		notify, err := r.Recv()
		if err := checkTranscoderError(err); err != nil {
			glog.Infof(`End of stream receive cycle because of err=%q, waiting for running transcode jobs to complete`, err)
			wg.Wait()
			return err
		}
		if notify.SegData != nil && notify.SegData.AuthToken != nil && len(notify.SegData.AuthToken.SessionId) > 0 && len(notify.Url) == 0 {
			// session teardown signal
			n.Transcoder.EndTranscodingSession(notify.SegData.AuthToken.SessionId)
		} else {
			wg.Add(1)
			go func() {
				runTranscode(n, orchAddr, httpc, notify)
				wg.Done()
			}()
		}
	}
}

func runTranscode(n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifySegment) {

	glog.Infof("Transcoding taskId=%d url=%s", notify.TaskId, notify.Url)
	var contentType, fname string
	var body bytes.Buffer
	var tData *core.TranscodeData

	md, err := coreSegMetadata(notify.SegData)
	if err != nil {
		glog.Errorf("Unable to parse segData taskId=%d url=%s err=%q", notify.TaskId, notify.Url, err)
		sendTranscodeResult(context.Background(), n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
		// TODO short-circuit error handling
		// See https://github.com/livepeer/go-livepeer/issues/1518
	}
	profiles := md.Profiles
	ctx := clog.AddManifestID(context.Background(), string(md.ManifestID))
	if md.AuthToken != nil {
		ctx = clog.AddOrchSessionID(ctx, md.AuthToken.SessionId)
	}
	ctx = clog.AddSeqNo(ctx, uint64(md.Seq))
	ctx = clog.AddVal(ctx, "taskId", strconv.FormatInt(notify.TaskId, 10))
	if n.Capabilities != nil && !md.Caps.CompatibleWith(n.Capabilities.ToNetCapabilities()) {
		clog.Errorf(ctx, "Requested capabilities for segment are not compatible with this node taskId=%d url=%s err=%q", notify.TaskId, notify.Url, errCapabilities)
		sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, errCapabilities)
		return
	}
	data, err := core.GetSegmentData(ctx, notify.Url)
	if err != nil {
		clog.Errorf(ctx, "Transcoder cannot get segment from taskId=%d url=%s err=%q", notify.TaskId, notify.Url, err)
		sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
	}
	// Write it to disk
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err = os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			clog.Errorf(ctx, "Transcoder cannot create workdir err=%q", err)
			sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
			return
		}
	}
	// Create input file from segment. Removed after transcoding done
	fname = path.Join(n.WorkDir, common.RandName()+".tempfile")
	if err = ioutil.WriteFile(fname, data, 0600); err != nil {
		clog.Errorf(ctx, "Transcoder cannot write file err=%q", err)
		sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
	}
	defer os.Remove(fname)
	md.Fname = fname
	md.Metadata = core.MakeMetadata(notify.OrchId)
	clog.V(common.DEBUG).Infof(ctx, "Segment from taskId=%d url=%s saved to file=%s", notify.TaskId, notify.Url, fname)

	start := time.Now()
	tData, err = n.Transcoder.Transcode(ctx, md)
	clog.V(common.VERBOSE).InfofErr(ctx, "Transcoding done for taskId=%d url=%s dur=%v", notify.TaskId, notify.Url, time.Since(start), err)
	if err != nil {
		if _, ok := err.(core.UnrecoverableError); ok {
			defer panic(err)
		}
		sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
		return
	}
	if err == nil && len(tData.Segments) != len(profiles) {
		err = errors.New("segment / profile mismatch")
		sendTranscodeResult(ctx, n, orchAddr, httpc, notify, contentType, &body, tData, err)
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

func sendTranscodeResult(ctx context.Context, n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifySegment,
	contentType string, body *bytes.Buffer, tData *core.TranscodeData, err error,
) {
	if err != nil {
		clog.Errorf(ctx, "Unable to transcode err=%q", err)
		body.Write([]byte(err.Error()))
		contentType = transcodingErrorMimeType
	}
	req, err := http.NewRequest("POST", "https://"+orchAddr+"/transcodeResults", body)
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

// Orchestrator gRPC

func (h *lphttp) RegisterTranscoder(req *net.RegisterRequest, stream net.Transcoder_RegisterTranscoderServer) error {
	from := common.GetConnectionAddr(stream.Context())
	glog.Infof("Got a RegisterTranscoder request from transcoder=%s capacity=%d", from, req.Capacity)

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
	h.orchestrator.ServeTranscoder(stream, int(req.Capacity), req.Capabilities)
	return nil
}

// Orchestrator HTTP

func (h *lphttp) TranscodeResults(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator

	authType := r.Header.Get("Authorization")
	creds := r.Header.Get("Credentials")
	if protoVerLPT != authType {
		glog.Error("Invalid auth type ", authType)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if creds != orch.TranscoderSecret() {
		glog.Error("Invalid shared secret")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		glog.Error("Error getting mime type ", err)
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	tid, err := strconv.ParseInt(r.Header.Get("TaskId"), 10, 64)
	if err != nil {
		glog.Error("Could not parse task ID ", err)
		http.Error(w, "Invalid Task ID", http.StatusBadRequest)
		return
	}

	var res core.RemoteTranscoderResult
	if transcodingErrorMimeType == mediaType {
		w.Write([]byte("OK"))
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("Unable to read transcoding error body taskId=%v err=%q", tid, err)
			res.Err = err
		} else {
			res.Err = fmt.Errorf(string(body))
		}
		glog.Errorf("Transcoding error for taskId=%v err=%q", tid, res.Err)
		orch.TranscoderResults(tid, &res)
		return
	}

	decodedPixels, err := strconv.ParseInt(r.Header.Get("Pixels"), 10, 64)
	if err != nil {
		glog.Error("Could not parse decoded pixels", err)
		http.Error(w, "Invalid Pixels", http.StatusBadRequest)
		return
	}

	var segments []*core.TranscodedSegmentData
	if mediaType == "multipart/mixed" {
		start := time.Now()
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				glog.Error("Could not process multipart part ", err)
				res.Err = err
				break
			}
			body, err := common.ReadAtMost(p, common.MaxSegSize)
			if err != nil {
				glog.Error("Error reading body ", err)
				res.Err = err
				break
			}

			if len(p.Header.Values("Pixels")) > 0 {
				encodedPixels, err := strconv.ParseInt(p.Header.Get("Pixels"), 10, 64)
				if err != nil {
					glog.Error("Error getting pixels in header:", err)
					res.Err = err
					break
				}
				segments = append(segments, &core.TranscodedSegmentData{Data: body, Pixels: encodedPixels})
			} else if p.Header.Get("Content-Type") == "application/octet-stream" {
				// Perceptual hash data for last segment
				if len(segments) > 0 {
					segments[len(segments)-1].PHash = body
				} else {
					err := errors.New("Unknown perceptual hash")
					glog.Error("No previous segment present to attach perceptual hash data to: ", err)
					res.Err = err
					break
				}
			}
		}
		res.TranscodeData = &core.TranscodeData{
			Segments: segments,
			Pixels:   decodedPixels,
		}
		dlDur := time.Since(start)
		glog.V(common.VERBOSE).Infof("Downloaded results from remote transcoder=%s taskId=%d dur=%s", r.RemoteAddr, tid, dlDur)

		if monitor.Enabled {
			monitor.SegmentDownloaded(r.Context(), 0, uint64(tid), dlDur)
		}

		orch.TranscoderResults(tid, &res)
	}
	if res.Err != nil {
		http.Error(w, res.Err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("OK"))
}
