package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
)

const protoVerLPT = "Livepeer-Transcoder-1.0"
const transcodingErrorMimeType = "livepeer/transcoding-error"

var errSecret = errors.New("Invalid secret")

// Standalone Transcoder

// RunTranscoder is main routing of standalone transcoder
// Exiting it will terminate executable
func RunTranscoder(n *core.LivepeerNode, orchAddr string, capacity int) {
	expb := backoff.NewExponentialBackOff()
	expb.MaxInterval = time.Minute
	expb.MaxElapsedTime = 0
	backoff.Retry(func() error {
		glog.Info("Registering transcoder to ", orchAddr)
		err := runTranscoder(n, orchAddr, capacity)
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
		if status.Code(err) == codes.Canceled {
			return core.NewRemoteTranscoderFatalError(fmt.Errorf("Execution interrupted"))
		}
	}
	return err
}

func runTranscoder(n *core.LivepeerNode, orchAddr string, capacity int) error {
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
	r, err := c.RegisterTranscoder(ctx, &net.RegisterRequest{Secret: n.OrchSecret, Capacity: int64(capacity)})
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

	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var wg sync.WaitGroup
	for {
		notify, err := r.Recv()
		if err := checkTranscoderError(err); err != nil {
			glog.Infof(`End of stream recieve cylcle because of err="%v", waiting for running transcode jobs to complete`, err)
			wg.Wait()
			return err
		}
		wg.Add(1)
		go func() {
			runTranscode(n, orchAddr, httpc, notify)
			wg.Done()
		}()
	}
}

func runTranscode(n *core.LivepeerNode, orchAddr string, httpc *http.Client, notify *net.NotifySegment) {
	profiles, err := common.TxDataToVideoProfile(hex.EncodeToString(notify.Profiles))
	if err != nil {
		glog.Info("Unable to deserialize profiles ", err)
	}

	glog.Infof("Transcoding taskId=%d url=%s", notify.TaskId, notify.Url)
	var contentType string
	var body bytes.Buffer

	tData, err := n.Transcoder.Transcode(notify.Url, profiles)
	glog.V(common.VERBOSE).Infof("Transcoding done for taskId=%d url=%s err=%v", notify.TaskId, notify.Url, err)
	if err != nil {
		glog.Error("Unable to transcode ", err)
		body.Write([]byte(err.Error()))
		contentType = transcodingErrorMimeType
	} else {
		boundary := randName()
		w := multipart.NewWriter(&body)
		for _, v := range tData {
			w.SetBoundary(boundary)
			hdrs := textproto.MIMEHeader{
				"Content-Type":   {"video/MP2T"},
				"Content-Length": {strconv.Itoa(len(v))},
			}
			fw, err := w.CreatePart(hdrs)
			if err != nil {
				glog.Error("Could not create multipart part ", err)
			}
			io.Copy(fw, bytes.NewBuffer(v))
		}
		w.Close()
		contentType = "multipart/mixed; boundary=" + boundary
	}
	req, err := http.NewRequest("POST", "https://"+orchAddr+"/transcodeResults", &body)
	if err != nil {
		glog.Error("Error posting results ", err)
	}
	req.Header.Set("Authorization", protoVerLPT)
	req.Header.Set("Credentials", n.OrchSecret)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("TaskId", strconv.FormatInt(notify.TaskId, 10))
	resp, err := httpc.Do(req)
	if err != nil {
		glog.Error("Error submitting results ", err)
	}
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	glog.V(common.VERBOSE).Infof("Transcoding done results sent for taskId=%d url=%s err=%v", notify.TaskId, notify.Url, err)
}

// Orchestrator gRPC

func (h *lphttp) RegisterTranscoder(req *net.RegisterRequest, stream net.Transcoder_RegisterTranscoderServer) error {
	from := common.GetConnectionAddr(stream.Context())
	glog.Infof("Got a RegisterTranscoder request from transcoder=%s", from)

	if req.Secret != h.orchestrator.TranscoderSecret() {
		glog.Info(errSecret.Error())
		return errSecret
	}

	// blocks until stream is finished
	h.orchestrator.ServeTranscoder(stream, int(req.Capacity))
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
			glog.Error("Unable to read transcoding error body ", err)
			res.Err = err
		} else {
			res.Err = fmt.Errorf(string(body))
		}
		glog.Error("Trascoding error ", res.Err)
		orch.TranscoderResults(tid, &res)
		return
	}

	var segments [][]byte
	if "multipart/mixed" == mediaType {
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
			body, err := ioutil.ReadAll(p)
			if err != nil {
				glog.Error("Error reading body ", err)
				res.Err = err
				break
			}
			segments = append(segments, body)
		}
		res.Segments = segments
		orch.TranscoderResults(tid, &res)
	}
	if res.Err != nil {
		http.Error(w, res.Err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("OK"))
}

// utils

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return hex.EncodeToString(x)
}
