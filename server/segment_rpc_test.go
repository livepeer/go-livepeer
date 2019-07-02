package server

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func serveSegmentHandler(orch Orchestrator) http.Handler {
	lp := lphttp{
		orchestrator: orch,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lp.ServeSegment(w, r)
	})
}
func TestServeSegment_GetPaymentError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	headers := map[string]string{
		paymentHeader: "foo",
	}
	resp := httpPostResp(handler, nil, headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusPaymentRequired, resp.StatusCode)
	assert.Contains(strings.TrimSpace(string(body)), "base64")
}

func TestServeSegment_VerifySegCredsError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: "foo",
	}
	resp := httpPostResp(handler, nil, headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusForbidden, resp.StatusCode)
	assert.Equal(errSegEncoding.Error(), strings.TrimSpace(string(body)))
}

func TestServeSegment_MismatchHashError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
	}
	creds, err := genSegCreds(s, &stream.HLSSegment{})
	require.Nil(t, err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader([]byte("foo")), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert := assert.New(t)
	assert.Equal(http.StatusForbidden, resp.StatusCode)
	assert.Equal("Forbidden", strings.TrimSpace(string(body)))
}

func TestServeSegment_TranscodeSegError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)
	orch.On("TranscodeSeg", md, seg).Return(nil, errors.New("TranscodeSeg error"))

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Error)
	assert.True(ok)
	assert.Equal("TranscodeSeg error", res.Error)
}

func TestServeSegment_OSSaveDataError(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)

	mos := &mockOSSession{}

	mos.On("SaveData", mock.Anything, mock.Anything).Return("", errors.New("SaveData error"))

	tRes := &core.TranscodeResult{
		Data: [][]byte{
			[]byte("foo"),
		},
		Sig: []byte("foo"),
		OS:  mos,
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(0, len(res.Data.Segments))
}

func TestServeSegment_ReturnSingleTranscodedSegmentData(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)

	tRes := &core.TranscodeResult{
		Data: [][]byte{
			[]byte("foo"),
		},
		Sig: []byte("foo"),
		OS:  drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(1, len(res.Data.Segments))
}

func TestServeSegment_ReturnMultipleTranscodedSegmentData(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
			ffmpeg.P240p30fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(nil)

	tRes := &core.TranscodeResult{
		Data: [][]byte{
			[]byte("foo"),
			[]byte("foo"),
		},
		Sig: []byte("foo"),
		OS:  drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	res, ok := tr.Result.(*net.TranscodeResult_Data)
	assert.True(ok)
	assert.Equal([]byte("foo"), res.Data.Sig)
	assert.Equal(2, len(res.Data.Segments))
}

func TestServeSegment_UpdateOrchestratorInfo(t *testing.T) {
	orch := &mockOrchestrator{}
	handler := serveSegmentHandler(orch)

	require := require.New(t)

	orch.On("VerifySig", mock.Anything, mock.Anything, mock.Anything).Return(true)

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		Profiles: []ffmpeg.VideoProfile{
			ffmpeg.P720p60fps16x9,
		},
	}
	seg := &stream.HLSSegment{Data: []byte("foo")}
	creds, err := genSegCreds(s, seg)
	require.Nil(err)

	md, err := verifySegCreds(orch, creds, ethcommon.Address{})
	require.Nil(err)

	params := &net.TicketParams{
		Recipient:         []byte("foo"),
		FaceValue:         big.NewInt(100).Bytes(),
		WinProb:           big.NewInt(100).Bytes(),
		RecipientRandHash: []byte("bar"),
		Seed:              []byte("baz"),
	}

	// Return an acceptable payment error to trigger an update to orchestrator info
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(errors.New("some error")).Once()
	orch.On("TicketParams", mock.Anything).Return(params, nil).Once()

	uri, err := url.Parse("http://google.com")
	require.Nil(err)
	orch.On("ServiceURI").Return(uri)

	tRes := &core.TranscodeResult{
		Data: [][]byte{
			[]byte("foo"),
		},
		Sig: []byte("foo"),
		OS:  drivers.NewMemoryDriver(nil).NewSession(""),
	}
	orch.On("TranscodeSeg", md, seg).Return(tRes, nil)

	headers := map[string]string{
		paymentHeader: "",
		segmentHeader: creds,
	}
	resp := httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(err)

	var tr net.TranscodeResult
	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert := assert.New(t)
	assert.Equal(http.StatusOK, resp.StatusCode)

	assert.Equal(uri.String(), tr.Info.Transcoder)
	assert.Equal(params.Recipient, tr.Info.TicketParams.Recipient)
	assert.Equal(params.FaceValue, tr.Info.TicketParams.FaceValue)
	assert.Equal(params.WinProb, tr.Info.TicketParams.WinProb)
	assert.Equal(params.RecipientRandHash, tr.Info.TicketParams.RecipientRandHash)
	assert.Equal(params.Seed, tr.Info.TicketParams.Seed)

	// Return an acceptable payment error to trigger an update to orchestrator info
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(errors.New("some other error")).Once()
	orch.On("TicketParams", mock.Anything).Return(params, nil).Once()

	resp = httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	require.Nil(err)

	err = proto.Unmarshal(body, &tr)
	require.Nil(err)

	assert.Equal(http.StatusOK, resp.StatusCode)

	assert.Equal(uri.String(), tr.Info.Transcoder)
	assert.Equal(params.Recipient, tr.Info.TicketParams.Recipient)
	assert.Equal(params.FaceValue, tr.Info.TicketParams.FaceValue)
	assert.Equal(params.WinProb, tr.Info.TicketParams.WinProb)
	assert.Equal(params.RecipientRandHash, tr.Info.TicketParams.RecipientRandHash)
	assert.Equal(params.Seed, tr.Info.TicketParams.Seed)

	// Test orchestratorInfo error
	orch.On("ProcessPayment", net.Payment{}, s.ManifestID).Return(errors.New("some error")).Once()
	orch.On("TicketParams", mock.Anything).Return(nil, errors.New("TicketParams error")).Once()

	resp = httpPostResp(handler, bytes.NewReader(seg.Data), headers)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	require.Nil(err)

	assert.Equal(http.StatusInternalServerError, resp.StatusCode)
	assert.Equal("Internal Server Error", strings.TrimSpace(string(body)))
}

func TestSubmitSegment_GenSegCredsError(t *testing.T) {
	b := stubBroadcaster2()
	b.signErr = errors.New("Sign error")

	s := &BroadcastSession{
		Broadcaster: b,
		ManifestID:  core.RandomManifestID(),
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Sign error", err.Error())
}

func TestSubmitSegment_HttpPostError(t *testing.T) {
	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: "https://foo.com",
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "connection refused")
}

func TestSubmitSegment_Non200StatusCode(t *testing.T) {
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server error", http.StatusInternalServerError)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "Server error", err.Error())
}

func TestSubmitSegment_ProtoUnmarshalError(t *testing.T) {
	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("foo"))
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
		},
	}

	_, err := SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Contains(t, err.Error(), "proto")
}

func TestSubmitSegment_TranscodeResultError(t *testing.T) {
	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Error{Error: "TranscodeResult error"},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(t, err)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
		},
	}

	_, err = SubmitSegment(s, &stream.HLSSegment{}, 0)

	assert.Equal(t, "TranscodeResult error", err.Error())
}

func TestSubmitSegment_Success(t *testing.T) {
	require := require.New(t)

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{
					&net.TranscodedSegmentData{Url: "foo"},
				},
				Sig: []byte("bar"),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	var runChecks func(r *http.Request)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		if runChecks != nil {
			runChecks(r)
		}

		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
		},
	}

	assert := assert.New(t)

	runChecks = func(r *http.Request) {
		assert.Equal("video/MP2T", r.Header.Get("Content-Type"))

		data, err := ioutil.ReadAll(r.Body)
		require.Nil(err)

		assert.Equal([]byte("dummy"), data)
	}

	tdata, err := SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal(1, len(tdata.Segments))
	assert.Equal("foo", tdata.Segments[0].Url)
	assert.Equal([]byte("bar"), tdata.Sig)

	// Test when input data is uploaded
	runChecks = func(r *http.Request) {
		assert.Equal("application/vnd+livepeer.uri", r.Header.Get("Content-Type"))

		data, err := ioutil.ReadAll(r.Body)
		require.Nil(err)

		assert.Equal([]byte("foo"), data)
	}

	SubmitSegment(s, &stream.HLSSegment{Name: "foo", Data: []byte("dummy")}, 0)
}

func TestSubmitSegment_UpdateOrchestratorInfo(t *testing.T) {
	require := require.New(t)

	params := pm.TicketParams{
		Recipient:         ethcommon.Address{},
		FaceValue:         big.NewInt(100),
		WinProb:           big.NewInt(100),
		RecipientRandHash: pm.RandHash(),
		Seed:              big.NewInt(100),
	}

	tr := &net.TranscodeResult{
		Result: &net.TranscodeResult_Data{
			Data: &net.TranscodeData{
				Segments: []*net.TranscodedSegmentData{
					&net.TranscodedSegmentData{Url: "foo"},
				},
				Sig: []byte("bar"),
			},
		},
		Info: &net.OrchestratorInfo{
			Transcoder: "http://google.com",
			TicketParams: &net.TicketParams{
				Recipient:         params.Recipient.Bytes(),
				FaceValue:         params.FaceValue.Bytes(),
				WinProb:           params.WinProb.Bytes(),
				RecipientRandHash: params.RecipientRandHash.Bytes(),
				Seed:              params.Seed.Bytes(),
			},
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	ts, mux := stubTLSServer()
	defer ts.Close()
	mux.HandleFunc("/segment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	// Test with no pm.Sender in BroadcastSession

	s := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		ManifestID:  core.RandomManifestID(),
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: ts.URL,
		},
	}

	assert := assert.New(t)

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("http://google.com", s.OrchestratorInfo.Transcoder)
	assert.Equal(tr.Info.TicketParams.Recipient, s.OrchestratorInfo.TicketParams.Recipient)
	assert.Equal(tr.Info.TicketParams.FaceValue, s.OrchestratorInfo.TicketParams.FaceValue)
	assert.Equal(tr.Info.TicketParams.WinProb, s.OrchestratorInfo.TicketParams.WinProb)
	assert.Equal(tr.Info.TicketParams.RecipientRandHash, s.OrchestratorInfo.TicketParams.RecipientRandHash)
	assert.Equal(tr.Info.TicketParams.Seed, s.OrchestratorInfo.TicketParams.Seed)

	// Test with pm.Sender in BroadcastSession

	sender := &pm.MockSender{}
	s.OrchestratorInfo = &net.OrchestratorInfo{
		Transcoder: ts.URL,
	}
	s.Sender = sender

	ticket := &pm.Ticket{
		Recipient:              ethcommon.Address{},
		Sender:                 ethcommon.Address{},
		FaceValue:              big.NewInt(100),
		WinProb:                big.NewInt(100),
		SenderNonce:            0,
		RecipientRandHash:      pm.RandHash(),
		CreationRound:          5,
		CreationRoundBlockHash: [32]byte{5},
	}

	sender.On("CreateTicket", mock.Anything).Return(ticket, big.NewInt(7), []byte("bar"), nil)
	sender.On("StartSession", params).Return("foobar")

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("foobar", s.PMSessionID)
	sender.AssertCalled(t, "StartSession", params)

	// Test does not crash if OrchestratorInfo.TicketParams is nil

	s.OrchestratorInfo = &net.OrchestratorInfo{
		Transcoder: ts.URL,
	}
	s.Sender = nil

	// Update stub server to return OrchestratorInfo without TicketParams
	tr.Info = &net.OrchestratorInfo{
		Transcoder: "http://google.com",
	}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal("http://google.com", s.OrchestratorInfo.Transcoder)

	// Test update OrchestratorOS

	s.OrchestratorInfo = &net.OrchestratorInfo{
		Transcoder: ts.URL,
	}

	tr.Info = &net.OrchestratorInfo{
		Transcoder: "http://google.com",
		Storage: []*net.OSInfo{
			&net.OSInfo{
				StorageType: 1,
				S3Info: &net.S3OSInfo{
					Host: "http://apple.com",
				},
			},
		},
	}
	buf, err = proto.Marshal(tr)
	require.Nil(err)

	_, err = SubmitSegment(s, &stream.HLSSegment{Data: []byte("dummy")}, 0)

	assert.Nil(err)
	assert.Equal(tr.Info.Storage[0].StorageType, s.OrchestratorOS.GetInfo().StorageType)
	assert.Equal(tr.Info.Storage[0].S3Info.Host, s.OrchestratorOS.GetInfo().S3Info.Host)
}

func stubTLSServer() (*httptest.Server, *http.ServeMux) {
	mux := http.NewServeMux()

	ts := httptest.NewUnstartedServer(mux)
	ts.TLS = &tls.Config{
		// This config option fixes the "unexpected ALPN protocol ""; want h2" error
		NextProtos: []string{http2.NextProtoTLS},
	}
	ts.StartTLS()

	return ts, mux
}
