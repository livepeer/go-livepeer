package server

import (
	"context"
	"errors"
	"io"
	"math/big"
	"net/http"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

const paymentRequestTimeout = 1 * time.Minute

type SegmentInfoSender struct {
	sess      *BroadcastSession
	inPixels  int64
	priceInfo *net.PriceInfo
}

type SegmentInfoReceiver struct {
	sender    ethcommon.Address
	sessionID string
	inPixels  int64
	priceInfo *net.PriceInfo
}

// RealtimePaymentSender is used in Gateway to send payment to Orchestrator
type RealtimePaymentSender interface {
	// SendPayment process the streamInfo and sends a payment to Orchestrator if needed
	SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error
}

// RealtimePaymentReceiver is used in Orchestrator to account for each processed segment
type RealtimePaymentReceiver interface {
	// AccountSegment checks if the stream is paid and if not it returns error, so that stream can be stopped
	AccountSegment(ctx context.Context, segmentInfo *SegmentInfoReceiver) error
}

type realtimePaymentSender struct {
	segmentsToPayUpfront int64
}

type realtimePaymentReceiver struct {
	orchestrator Orchestrator
}

func (r *realtimePaymentSender) SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error {
	sess := segmentInfo.sess

	if err := refreshSessionIfNeeded(ctx, sess); err != nil {
		return err
	}

	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	// We pay a few segments upfront to avoid race condition between payment and segment processing
	safeMinCredit := new(big.Rat).Mul(fee, new(big.Rat).SetInt64(r.segmentsToPayUpfront))
	balUpdate, err := newBalanceUpdate(sess, safeMinCredit)
	if err != nil {
		return err
	}
	balUpdate.Debit = fee
	balUpdate.Status = ReceivedChange

	defer completeBalanceUpdate(sess, balUpdate)

	// Generate payment tickets
	payment, err := genPayment(ctx, sess, balUpdate.NumTickets)
	if err != nil {
		clog.Errorf(ctx, "Could not create payment err=%q", err)
		if monitor.Enabled {
			monitor.PaymentCreateError(ctx)
		}
		return err
	}

	url := sess.OrchestratorInfo.Transcoder
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/payment", nil)
	if err != nil {
		clog.Errorf(ctx, "Could not generate payment request to orch=%s", url)
		// TODO: Monitor metrics for payments
		return err
	}

	// genSegCreds expects a stream.HLSSegment so in order to reuse it here we pass a dummy object
	clog.Infof(ctx, "Session ID=%v", sess.Params.SessionID)
	segCreds, err := genSegCreds(sess, &stream.HLSSegment{}, nil, false)
	if err != nil {
		return err
	}

	req.Header.Set(paymentHeader, payment)
	// TODO: Check if we can get rid of this for AI
	req.Header.Set(segmentHeader, segCreds)

	// Send payment to Orchestrator
	// TODO
	resp, err := sendReqWithTimeout(req, paymentRequestTimeout)
	if err != nil {
		clog.Errorf(ctx, "Could not send payment to orch=%s err=%q", url, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		clog.Errorf(ctx, "Orchestrator did not accept payment status=%d", resp.StatusCode)
		return err
	}

	if monitor.Enabled {
		monitor.TicketValueSent(ctx, balUpdate.NewCredit)
		monitor.TicketsSent(ctx, balUpdate.NumTickets)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Could not read response from orchestrator=%s err=%q", url, err)
		return err
	}

	var pr net.PaymentResult
	err = proto.Unmarshal(data, &pr)
	if err != nil {
		clog.Errorf(ctx, "Could not unmarshal response from orchestrator=%s err=%q", url)
		return err
	}

	updateSession(sess, &ReceivedTranscodeResult{Info: pr.Info})
	clog.Infof(ctx, "Payment sent to orchestrator=%s", url)

	return nil
}

func refreshSessionIfNeeded(ctx context.Context, sess *BroadcastSession) error {
	shouldRefresh, err := shouldRefreshSession(ctx, sess)
	if err != nil {
		return err
	}
	if shouldRefresh {
		if err := refreshSession(ctx, sess); err != nil {
			return err
		}
	}
	return nil
}

func (r *realtimePaymentReceiver) AccountPayment(
	ctx context.Context, segmentInfo SegmentInfoReceiver) error {
	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	balance := r.orchestrator.Balance(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID))
	if balance.Cmp(fee) < 0 {
		return errors.New("insufficient balance")
	}
	r.orchestrator.DebitFees(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID), segmentInfo.priceInfo, segmentInfo.inPixels)
	clog.V(common.DEBUG).Infof(ctx, "Accounted for payment for sessionID=%s, fee=%s", segmentInfo.sessionID, fee.FloatString(0))
	return nil
}

func calculateFee(inPixels int64, price *net.PriceInfo) *big.Rat {
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	return priceRat.Mul(priceRat, big.NewRat(inPixels, 1))
}
