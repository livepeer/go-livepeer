package server

import (
	"context"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"math/big"
	"net/http"
	"testing"
	"time"
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
	paymentSender := realtimePaymentSender{
		segmentsToPayUpfront: 10,
	}
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
	// Paid upfront for 10 segments => 10000000
	// Spent cost for 1 segment => 1000000
	// The balance should be 9000000
	balance := sess.Balances.Balance(ethcommon.BytesToAddress(sess.OrchestratorInfo.Address), core.ManifestID(sess.OrchestratorInfo.AuthToken.SessionId))
	require.Equal(new(big.Rat).SetInt64(9000000), balance)
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
