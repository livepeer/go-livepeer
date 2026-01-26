package server

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendPayment(t *testing.T) {
	require := require.New(t)

	// given

	// Stub Orchestrator
	ts, mux := stubTLSServer()
	defer ts.Close()
	tr := &net.PaymentResult{
		Info: &net.OrchestratorInfo{
			Transcoder:   ts.URL,
			PriceInfo:    &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{ExpirationBlock: big.NewInt(100).Bytes()},
			AuthToken:    stubAuthToken,
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	mux.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	// Stub session
	sess := StubBroadcastSession(ts.URL)
	sess.Sender = mockSender()
	sess.Balances = core.NewAddressBalances(1 * time.Minute)
	sess.Balance = core.NewBalance(ethcommon.BytesToAddress(sess.OrchestratorInfo.Address), core.ManifestID(sess.OrchestratorInfo.AuthToken.SessionId), sess.Balances)

	// Create Payment sender and segment info
	paymentSender := livePaymentSender{}
	segmentInfo := &SegmentInfoSender{
		sess:     sess,
		inPixels: 1000000,
		priceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
	}

	// when
	err = paymentSender.SendPayment(context.TODO(), segmentInfo)

	// then
	require.Nil(err)
	// One segment costs 1000000
	// Spent cost for 1 segment => 1000000
	// The balance should be 0
	balance := sess.Balances.Balance(ethcommon.BytesToAddress(sess.OrchestratorInfo.Address), core.ManifestID(sess.OrchestratorInfo.AuthToken.SessionId))
	require.Equal(new(big.Rat).SetInt64(0), balance)
}

func mockSender() pm.Sender {
	sender := &pm.MockSender{}
	sender.On("StartSession", mock.Anything).Return("foo")
	sender.On("StopSession", mock.Anything).Times(3)
	sender.On("EV", mock.Anything).Return(big.NewRat(1000000, 1), nil)
	sender.On("CreateTicketBatch", mock.Anything, mock.Anything).Return(defaultTicketBatch(), nil)
	sender.On("ValidateTicketParams", mock.Anything).Return(nil)
	return sender
}

func TestAccountPayment(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		name   string
		credit *big.Rat
		expErr bool
	}{
		{
			name:   "No credit",
			credit: nil,
			expErr: true,
		},
		{
			name:   "Insufficient balance",
			credit: new(big.Rat).SetInt64(900000),
			expErr: true,
		},
		{
			name:   "Sufficient balance",
			credit: new(big.Rat).SetInt64(1100000),
			expErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			sessionID := "abcdef"
			sender := ethcommon.HexToAddress("0x0000000000000000000000000000000000000001")
			segmentInfo := &SegmentInfoReceiver{
				sender:    sender,
				sessionID: sessionID,
				inPixels:  1000000,
				priceInfo: &net.PriceInfo{
					PricePerUnit:  1,
					PixelsPerUnit: 1,
				},
			}

			node, _ := core.NewLivepeerNode(nil, "", nil)
			balances := core.NewAddressBalances(1 * time.Minute)
			node.Balances = balances
			orch := core.NewOrchestrator(node, nil)

			paymentReceiver := livePaymentReceiver{orchestrator: orch}
			if tt.credit != nil {
				node.Balances.Credit(sender, core.ManifestID(sessionID), tt.credit)
			}

			// when
			err := paymentReceiver.AccountPayment(context.TODO(), segmentInfo)

			// then
			if tt.expErr {
				require.Error(err, "insufficient balance")
			} else {
				require.Nil(err)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRemotePaymentSender_RequestPayment_ValidateReq(t *testing.T) {
	require := require.New(t)

	t.Run("remote signer not configured - nil receiver", func(t *testing.T) {
		var r *remotePaymentSender
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{})
		require.Contains(err.Error(), "remote signer not configured")
	})

	t.Run("remote signer not configured - nil node", func(t *testing.T) {
		r := &remotePaymentSender{}
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{})
		require.Contains(err.Error(), "remote signer not configured")
	})

	t.Run("remote signer not configured - nil RemoteSignerAddr", func(t *testing.T) {
		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = nil
		r := NewRemotePaymentSender(node)
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{})
		require.Contains(err.Error(), "remote signer not configured")
	})

	t.Run("segment info missing", func(t *testing.T) {
		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = &url.URL{Scheme: "http", Host: "example.com"}
		r := NewRemotePaymentSender(node)
		_, err := r.RequestPayment(context.Background(), nil)
		require.Contains(err.Error(), "segment info missing")
	})

	t.Run("missing session", func(t *testing.T) {
		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = &url.URL{Scheme: "http", Host: "example.com"}
		r := NewRemotePaymentSender(node)
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{sess: nil})
		require.Contains(err.Error(), "missing session or OrchestratorInfo")
	})

	t.Run("missing OrchestratorInfo", func(t *testing.T) {
		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = &url.URL{Scheme: "http", Host: "example.com"}
		r := NewRemotePaymentSender(node)
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{sess: &BroadcastSession{}})
		require.Contains(err.Error(), "missing session or OrchestratorInfo")
	})

	t.Run("missing PriceInfo", func(t *testing.T) {
		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = &url.URL{Scheme: "http", Host: "example.com"}
		r := NewRemotePaymentSender(node)
		_, err := r.RequestPayment(context.Background(), &SegmentInfoSender{
			sess: &BroadcastSession{OrchestratorInfo: &net.OrchestratorInfo{}},
		})
		require.Contains(err.Error(), "missing session or OrchestratorInfo")
	})
}

func TestRemotePaymentSender_RequestPayment_Success_CachesStateAndSendsExpectedPayload(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// Stub remote signer
	var gotReq RemotePaymentRequest
	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal("/generate-live-payment", r.URL.Path)
		require.Equal("application/json", r.Header.Get("Content-Type"))

		require.NoError(json.NewDecoder(r.Body).Decode(&gotReq))

		// Decode orchestrator blob back into a proto and sanity-check
		var pr net.PaymentResult
		require.NoError(proto.Unmarshal(gotReq.Orchestrator, &pr))
		require.NotNil(pr.Info)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(RemotePaymentResponse{
			Payment:  "dummy-payment",
			SegCreds: "dummy-segcreds",
			State:    RemotePaymentStateSig{State: []byte{0xAA}, Sig: []byte{0xBB}},
		})
	}))
	defer remoteTS.Close()
	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	// Session + node configured
	sess := StubBroadcastSession("http://orch.example")
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL

	r := NewRemotePaymentSender(node)
	r.state = RemotePaymentStateSig{State: []byte{0x01}, Sig: []byte{0x02}}

	segmentInfo := &SegmentInfoSender{
		sess: sess,
		mid:  "mid1",
	}

	// when
	resp, err := r.RequestPayment(context.Background(), segmentInfo)

	// then
	require.NoError(err)
	require.NotNil(resp)
	assert.Equal("dummy-payment", resp.Payment)
	assert.Equal("dummy-segcreds", resp.SegCreds)
	assert.Equal(RemotePaymentStateSig{State: []byte{0xAA}, Sig: []byte{0xBB}}, resp.State)

	// Request payload checks
	assert.Equal("mid1", gotReq.ManifestID)
	assert.Equal(RemoteType_LiveVideoToVideo, gotReq.Type)
	assert.Equal(RemotePaymentStateSig{State: []byte{0x01}, Sig: []byte{0x02}}, gotReq.State)

	var pr net.PaymentResult
	require.NoError(proto.Unmarshal(gotReq.Orchestrator, &pr))
	require.NotNil(pr.Info)
	assert.Equal(sess.OrchestratorInfo.Transcoder, pr.Info.Transcoder)
	assert.Equal(sess.OrchestratorInfo.PriceInfo.PricePerUnit, pr.Info.PriceInfo.PricePerUnit)
	assert.Equal(sess.OrchestratorInfo.PriceInfo.PixelsPerUnit, pr.Info.PriceInfo.PixelsPerUnit)

	// Check cached state
	assert.Equal(RemotePaymentStateSig{State: []byte{0xAA}, Sig: []byte{0xBB}}, r.state)
}

func TestRemotePaymentSender_RequestPayment_UsesCapabilityPrice(t *testing.T) {
	require := require.New(t)

	var gotReq RemotePaymentRequest
	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(json.NewDecoder(r.Body).Decode(&gotReq))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RemotePaymentResponse{Payment: "p", SegCreds: "s"})
	}))
	defer remoteTS.Close()
	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	caps := core.NewCapabilities([]core.Capability{core.Capability_LiveVideoToVideo}, nil)
	caps.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
		core.Capability_LiveVideoToVideo: {
			Models: map[string]*core.ModelConstraint{
				"model-x": {},
			},
		},
	})

	sess := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID:   core.RandomManifestID(),
			Capabilities: caps,
		},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: "http://orch.example",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  1,
				PixelsPerUnit: 1,
			},
			CapabilitiesPrices: []*net.PriceInfo{
				{
					Capability:    uint32(core.Capability_LiveVideoToVideo),
					Constraint:    "model-x",
					PricePerUnit:  999,
					PixelsPerUnit: 3,
				},
			},
			TicketParams: &net.TicketParams{Recipient: pm.RandAddress().Bytes()},
			AuthToken:    stubAuthToken,
		},
		lock: &sync.RWMutex{},
	}

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL

	sender := NewRemotePaymentSender(node)
	_, err = sender.RequestPayment(context.Background(), &SegmentInfoSender{sess: sess})
	require.NoError(err)

	var pr net.PaymentResult
	require.NoError(proto.Unmarshal(gotReq.Orchestrator, &pr))
	require.NotNil(pr.Info)
	require.Equal(int64(999), pr.Info.PriceInfo.PricePerUnit)
	require.Equal(int64(3), pr.Info.PriceInfo.PixelsPerUnit)
	require.Equal("model-x", pr.Info.PriceInfo.Constraint)
}

func TestRemotePaymentSender_RequestPayment_RefreshFallbacksToCachedPrice(t *testing.T) {
	require := require.New(t)

	var gotReq RemotePaymentRequest
	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(json.NewDecoder(r.Body).Decode(&gotReq))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RemotePaymentResponse{Payment: "p", SegCreds: "s"})
	}))
	defer remoteTS.Close()
	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	caps := core.NewCapabilities([]core.Capability{core.Capability_LiveVideoToVideo}, nil)
	caps.SetPerCapabilityConstraints(core.PerCapabilityConstraints{
		core.Capability_LiveVideoToVideo: {
			Models: map[string]*core.ModelConstraint{
				"model-y": {},
			},
		},
	})

	sess := &BroadcastSession{
		Broadcaster: stubBroadcaster2(),
		Params: &core.StreamParameters{
			ManifestID:   core.RandomManifestID(),
			Capabilities: caps,
		},
		OrchestratorInfo: &net.OrchestratorInfo{
			Transcoder: "http://orch.example",
			PriceInfo: &net.PriceInfo{
				PricePerUnit:  7,
				PixelsPerUnit: 5,
			},
			TicketParams: &net.TicketParams{Recipient: pm.RandAddress().Bytes()},
			AuthToken:    stubAuthToken,
		},
		lock: &sync.RWMutex{},
	}

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL

	sender := NewRemotePaymentSender(node)

	origGetter := sender.getOrchInfo
	sender.getOrchInfo = func(ctx context.Context, _ common.Broadcaster, _ *url.URL, _ GetOrchestratorInfoParams) (*net.OrchestratorInfo, error) {
		// refreshed info is missing price info; should fall back to cached price
		return &net.OrchestratorInfo{
			Transcoder: sess.OrchestratorInfo.Transcoder,
			TicketParams: &net.TicketParams{
				Recipient: pm.RandAddress().Bytes(),
			},
			AuthToken: stubAuthToken,
		}, nil
	}
	defer func() { sender.getOrchInfo = origGetter }()

	_, err = sender.RequestPayment(context.Background(), &SegmentInfoSender{sess: sess, mid: "mid1"})
	require.NoError(err)

	var pr net.PaymentResult
	require.NoError(proto.Unmarshal(gotReq.Orchestrator, &pr))
	require.NotNil(pr.Info)
	require.Equal(int64(7), pr.Info.PriceInfo.PricePerUnit)
	require.Equal(int64(5), pr.Info.PriceInfo.PixelsPerUnit)
	require.Equal("", pr.Info.PriceInfo.Constraint)
}

func TestRemotePaymentSender_RequestPayment_RemoteSignerCallError(t *testing.T) {
	require := require.New(t)

	remoteURL, err := url.Parse("gopher://example")
	require.NoError(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL

	r := NewRemotePaymentSender(node)
	r.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}),
	}

	sess := StubBroadcastSession("http://orch.example")
	_, err = r.RequestPayment(context.Background(), &SegmentInfoSender{sess: sess, mid: "mid1"})
	require.Error(err)
	require.Contains(err.Error(), "failed to call remote signer")
}

func TestRemotePaymentSender_RequestPayment_RefreshSession(t *testing.T) {
	require := require.New(t)

	t.Run("guard triggers when callCount > 3", func(t *testing.T) {
		remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(HTTPStatusRefreshSession)
		}))
		defer remoteTS.Close()
		remoteURL, err := url.Parse(remoteTS.URL)
		require.NoError(err)

		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = remoteURL
		r := NewRemotePaymentSender(node)
		r.refreshSession = func(context.Context, *BroadcastSession, bool) error {
			t.Fatal("refreshSession should not be called when callCount > 3")
			return nil
		}

		sess := StubBroadcastSession("http://orch.example")
		segmentInfo := &SegmentInfoSender{sess: sess, mid: "mid1", callCount: 4}
		_, err = r.RequestPayment(context.Background(), segmentInfo)
		require.Equal("too many consecutive session refreshes", err.Error())
	})

	t.Run("refresh fails", func(t *testing.T) {
		remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(HTTPStatusRefreshSession)
		}))
		defer remoteTS.Close()
		remoteURL, err := url.Parse(remoteTS.URL)
		require.NoError(err)

		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = remoteURL
		r := NewRemotePaymentSender(node)
		r.refreshSession = func(context.Context, *BroadcastSession, bool) error {
			return errors.New("refresh failed")
		}

		sess := StubBroadcastSession("http://orch.example")
		segmentInfo := &SegmentInfoSender{sess: sess, mid: "mid1"}
		_, err = r.RequestPayment(context.Background(), segmentInfo)
		require.Contains(err.Error(), "could not refresh session for remote signer")
	})

	t.Run("refresh succeeds then retry succeeds", func(t *testing.T) {
		var callCount int
		var transcoderSeen []string

		remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++

			var req RemotePaymentRequest
			require.NoError(json.NewDecoder(r.Body).Decode(&req))

			var pr net.PaymentResult
			require.NoError(proto.Unmarshal(req.Orchestrator, &pr))
			require.NotNil(pr.Info)
			transcoderSeen = append(transcoderSeen, pr.Info.Transcoder)

			if callCount == 1 {
				w.WriteHeader(HTTPStatusRefreshSession)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(RemotePaymentResponse{
				Payment:  "dummy-payment",
				SegCreds: "dummy-segcreds",
				State:    RemotePaymentStateSig{State: []byte{0x03}, Sig: []byte{0x04}},
			})
		}))
		defer remoteTS.Close()
		remoteURL, err := url.Parse(remoteTS.URL)
		require.NoError(err)

		node, _ := core.NewLivepeerNode(nil, "", nil)
		node.RemoteSignerAddr = remoteURL
		r := NewRemotePaymentSender(node)

		refreshCalls := 0
		r.refreshSession = func(_ context.Context, sess *BroadcastSession, _ bool) error {
			refreshCalls++
			// Mutate sess so we can observe the new info in the second request payload
			sess.OrchestratorInfo.Transcoder = "http://orch.refreshed"
			return nil
		}

		sess := StubBroadcastSession("http://orch.initial")
		segmentInfo := &SegmentInfoSender{sess: sess, mid: "mid1"}

		resp, err := r.RequestPayment(context.Background(), segmentInfo)
		require.NoError(err)
		require.NotNil(resp)
		require.Equal("dummy-payment", resp.Payment)
		require.Equal(1, refreshCalls)
		require.Equal(1, segmentInfo.callCount)
		require.Equal(2, callCount)
		require.Len(transcoderSeen, 2)
		require.Equal("http://orch.initial", transcoderSeen[0])
		require.Equal("http://orch.refreshed", transcoderSeen[1])
	})
}

func TestRemotePaymentSender_RequestPayment_RemoteSignerNon200(t *testing.T) {
	require := require.New(t)

	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer remoteTS.Close()
	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL
	r := NewRemotePaymentSender(node)

	sess := StubBroadcastSession("http://orch.example")
	_, err = r.RequestPayment(context.Background(), &SegmentInfoSender{sess: sess, mid: "mid1"})
	require.Error(err)
	require.Contains(err.Error(), "remote signer returned status 500: boom")
}

func TestRemotePaymentSender_RequestPayment_RemoteSignerDecodeError(t *testing.T) {
	require := require.New(t)

	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer remoteTS.Close()
	remoteURL, err := url.Parse(remoteTS.URL)
	require.NoError(err)

	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL
	r := NewRemotePaymentSender(node)

	sess := StubBroadcastSession("http://orch.example")
	_, err = r.RequestPayment(context.Background(), &SegmentInfoSender{sess: sess, mid: "mid1"})
	require.Error(err)
	require.Contains(err.Error(), "failed to decode remote signer response")
}
