package server

import (
	"context"
	"fmt"
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
	mid       string
}

type SegmentInfoReceiver struct {
	sender    ethcommon.Address
	sessionID string
	inPixels  int64
	priceInfo *net.PriceInfo
}

// LivePaymentSender is used in Gateway to send payment to Orchestrator
type LivePaymentSender interface {
	// SendPayment process the streamInfo and sends a payment to Orchestrator if needed
	SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error
}

// LivePaymentReceiver is used in Orchestrator to account for each processed segment
type LivePaymentReceiver interface {
	// AccountSegment checks if the stream is paid and if not it returns error, so that stream can be stopped
	AccountSegment(ctx context.Context, segmentInfo *SegmentInfoReceiver) error
}

type livePaymentSender struct {
}

type livePaymentReceiver struct {
	orchestrator Orchestrator
}

func (r *livePaymentSender) SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error {
	if segmentInfo.priceInfo == nil || segmentInfo.priceInfo.PricePerUnit == 0 {
		clog.V(common.DEBUG).Infof(ctx, "Skipping sending payment, priceInfo not set for requestID=%s, ", segmentInfo.mid)
		return nil
	}
	sess := segmentInfo.sess

	if err := refreshSessionIfNeeded(ctx, sess); err != nil {
		return err
	}
	sess.lock.Lock()
	sess.Params.ManifestID = core.ManifestID(segmentInfo.mid)
	sess.lock.Unlock()

	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	balUpdate, err := newBalanceUpdate(sess, fee)
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

	// Generate segment credentials with an empty segment
	segCreds, err := genSegCreds(sess, &stream.HLSSegment{}, nil, false)
	if err != nil {
		return err
	}

	// Send payment to Orchestrator
	url := sess.OrchestratorInfo.Transcoder
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/payment", nil)
	if err != nil {
		clog.Errorf(ctx, "Could not generate payment request to orch=%s", url)
		return err
	}
	req.Header.Set(paymentHeader, payment)
	req.Header.Set(segmentHeader, segCreds)
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

	// Update session to preserve the same AuthToken.SessionID between payments
	updateSession(sess, &ReceivedTranscodeResult{Info: pr.Info})

	clog.V(common.DEBUG).Infof(ctx, "Payment sent to orchestrator=%s", url)
	return nil
}

func (r *livePaymentReceiver) AccountPayment(
	ctx context.Context, segmentInfo *SegmentInfoReceiver) error {
	if segmentInfo.priceInfo == nil || segmentInfo.priceInfo.PricePerUnit == 0 {
		clog.V(common.DEBUG).Infof(ctx, "Skipping accounting, priceInfo not set for sessionID=%s, ", segmentInfo.sessionID)
		return nil
	}
	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	balance := r.orchestrator.Balance(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID))
	if balance == nil || balance.Cmp(fee) < 0 {
		balanceStr := "nil"
		if balance != nil {
			balanceStr = balance.FloatString(0)
		}
		return fmt.Errorf("insufficient balance, mid=%s, fee=%s, balance=%s", segmentInfo.sessionID, fee.FloatString(0), balanceStr)
	}
	r.orchestrator.DebitFees(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID), segmentInfo.priceInfo, segmentInfo.inPixels)
	clog.V(common.DEBUG).Infof(ctx, "Accounted payment for sessionID=%s, fee=%s", segmentInfo.sessionID, fee.FloatString(0))
	return nil
}

func calculateFee(inPixels int64, price *net.PriceInfo) *big.Rat {
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	return priceRat.Mul(priceRat, big.NewRat(inPixels, 1))
}
