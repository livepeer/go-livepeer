package server

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
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
			Transcoder: ts.URL,
			PriceInfo:  &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{
				Recipient:         ethcommon.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
				FaceValue:         big.NewInt(1_000_000_000_000_000_000).Bytes(), // 1 ETH
				WinProb:           big.NewInt(1).Bytes(),
				RecipientRandHash: ethcommon.HexToHash("0x2222222222222222222222222222222222222222").Bytes(),
				Seed:              big.NewInt(1234).Bytes(),
				ExpirationBlock:   big.NewInt(100).Bytes(),
			},
			AuthToken: stubAuthToken,
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

// TestRemoteLivePaymentSender_BasicFlow verifies that remoteLivePaymentSender forwards
// remote-generated payments to the orchestrator.
func TestRemoteLivePaymentSender_BasicFlow(t *testing.T) {
	require := require.New(t)

	// Stub Orchestrator
	ts, mux := stubTLSServer()
	defer ts.Close()
	tr := &net.PaymentResult{
		Info: &net.OrchestratorInfo{
			Transcoder: ts.URL,
			PriceInfo:  &net.PriceInfo{PricePerUnit: 7, PixelsPerUnit: 7},
			TicketParams: &net.TicketParams{
				Recipient:         ethcommon.HexToAddress("0x3333333333333333333333333333333333333333").Bytes(),
				FaceValue:         big.NewInt(1_000_000_000_000_000_000).Bytes(), // 1 ETH
				WinProb:           big.NewInt(1).Bytes(),
				RecipientRandHash: ethcommon.HexToHash("0x4444444444444444444444444444444444444444").Bytes(),
				Seed:              big.NewInt(5678).Bytes(),
				ExpirationBlock:   big.NewInt(100).Bytes(),
			},
			AuthToken: stubAuthToken,
		},
	}
	buf, err := proto.Marshal(tr)
	require.Nil(err)

	mux.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		// Expect payment + segCreds headers from remote signer
		if r.Header.Get(paymentHeader) == "" {
			t.Errorf("missing payment header")
		}
		if r.Header.Get(segmentHeader) == "" {
			t.Errorf("missing segment header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write(buf)
	})

	// Stub remote signer
	remoteResp := RemotePaymentResponse{
		Payment:  "dummy-payment",
		SegCreds: "dummy-segcreds",
		State:    HexBytes([]byte{0x01}),
		// Signature value is not validated by the sender
		StateSignature: HexBytes([]byte{0x02}),
	}
	remoteBody, err := json.Marshal(remoteResp)
	require.Nil(err)

	remoteTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(remoteBody)
	}))
	defer remoteTS.Close()

	remoteURL, err := url.Parse(remoteTS.URL)
	require.Nil(err)

	// Stub session
	sess := StubBroadcastSession(ts.URL)
	sess.Sender = nil
	sess.Balances = nil

	// Livepeer node with RemoteSignerAddr configured
	node, _ := core.NewLivepeerNode(nil, "", nil)
	node.RemoteSignerAddr = remoteURL

	// Create remote payment sender and segment info
	paymentSender := NewRemotePaymentProcessor(node)
	segmentInfo := &SegmentInfoSender{
		sess:     sess,
		inPixels: 1000000,
		priceInfo: &net.PriceInfo{
			PricePerUnit:  1,
			PixelsPerUnit: 1,
		},
		mid: "mid1",
	}

	// when
	err = paymentSender.SendPayment(context.TODO(), segmentInfo)

	// then
	require.Nil(err)
}
