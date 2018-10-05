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
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
)

const ProtoVer_LPT = "Livepeer-Transcoder-1.0"
const TranscodingErrorMimeType = "livepeer/transcoding-error"

var SecretErr = errors.New("Invalid secret")

// Transcoder

func RunTranscoder(n *core.LivepeerNode, orchAddr string) {
	expb := backoff.NewExponentialBackOff()
	expb.MaxInterval = time.Minute
	expb.MaxElapsedTime = 0
	backoff.Retry(func() error {
		glog.Info("Registering transcoder to ", orchAddr)
		err := runTranscoder(n, orchAddr)
		glog.Info("Unregistering transcoder: ", err)
		if err != SecretErr {
			return err
		}
		glog.Info("Terminating transcoder")
		return nil
	}, expb)
}

func runTranscoder(n *core.LivepeerNode, orchAddr string) error {
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
	r, err := c.RegisterTranscoder(ctx, &net.RegisterRequest{Secret: n.OrchSecret})
	if err != nil {
		glog.Error("Could not register transcoder to orchestrator ", err)
		status := status.Convert(err)
		if status.Message() == SecretErr.Error() { // consider this unrecoverable
			return SecretErr
		}
		return err
	}

	httpc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	for {
		notify, err := r.Recv()
		if err != nil {
			status := status.Convert(err)
			if status.Message() == SecretErr.Error() { // consider this unrecoverable
				return SecretErr
			}
			return err
		}
		profiles, err := common.TxDataToVideoProfile(hex.EncodeToString(notify.Profiles))
		if err != nil {
			glog.Info("Unable to deserialize profiles ", err)
		}

		glog.Info("Transcoding ", notify.TaskId)
		var contentType string
		var body bytes.Buffer

		tData, err := n.Transcoder.Transcode(notify.Url, profiles)
		if err != nil {
			glog.Error("Unable to transcode ", err)
			body.Write([]byte(err.Error()))
			contentType = TranscodingErrorMimeType
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
					return err // XXX respond w error to orchestrator
				}
				io.Copy(fw, bytes.NewBuffer(v))
			}
			w.Close()
			contentType = "multipart/mixed; boundary=" + boundary
		}
		req, err := http.NewRequest("POST", "https://"+orchAddr+"/transcodeResults", &body)
		if err != nil {
			glog.Error("Error posting results ", err)
			return err
		}
		req.Header.Set("Authorization", ProtoVer_LPT)
		req.Header.Set("Credentials", n.OrchSecret)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("TaskId", strconv.FormatInt(notify.TaskId, 10))
		resp, err := httpc.Do(req)
		if err != nil {
			glog.Error("Error submitting results ", err)
			return err
		}
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}
	return nil
}

// Orchestrator gRPC

func (h *lphttp) RegisterTranscoder(req *net.RegisterRequest, stream net.Transcoder_RegisterTranscoderServer) error {
	glog.Info("Got a RegisterTranscoder request for ", req.Secret)

	if req.Secret != h.orchestrator.TranscoderSecret() {
		glog.Info(SecretErr.Error())
		return SecretErr
	}

	// blocks until stream is finished
	h.orchestrator.ServeTranscoder(stream)
	return nil
}

// Orchestrator HTTP

func (h *lphttp) TranscodeResults(w http.ResponseWriter, r *http.Request) {
	orch := h.orchestrator

	authType := r.Header.Get("Authorization")
	creds := r.Header.Get("Credentials")
	if ProtoVer_LPT != authType {
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
	if TranscodingErrorMimeType == mediaType {
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
