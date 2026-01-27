package server

import (
	"context"
	"math/big"
	"net/http"
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
