package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sync"
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
	callCount int
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

	if err := refreshSessionIfNeeded(ctx, sess, true); err != nil {
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
	defer logPaymentSent(ctx, sess)
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

	return nil
}

func logPaymentSent(ctx context.Context, sess *BroadcastSession) {
	balance := sess.Balance.Balance()
	if balance == nil {
		balance = big.NewRat(0, 1)
	}
	clog.V(common.DEBUG).Infof(ctx, "Payment sent to orchestrator=%s, balance=%s", sess.OrchestratorInfo.Transcoder, balance.FloatString(0))

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
	balance = r.orchestrator.Balance(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID))
	clog.V(common.DEBUG).Infof(ctx, "Accounted payment for sessionID=%s, fee=%s balance=%s", segmentInfo.sessionID, fee.FloatString(0), balance.FloatString(0))
	return nil
}

// Delegate ticket generation to a remote signer service and then forward the
// payment on to the orchestrator. Return intermediate payment state as a blob
type remotePaymentSender struct {
	node   *core.LivepeerNode
	client *http.Client

	// access to all fields below  must be protected by the mutex mu
	mu    sync.Mutex
	state RemotePaymentStateSig
}

func NewRemotePaymentSender(node *core.LivepeerNode) *remotePaymentSender {
	return &remotePaymentSender{
		node: node,
		client: &http.Client{
			Timeout: paymentRequestTimeout,
		},
	}
}

func (r *remotePaymentSender) RequestPayment(ctx context.Context, segmentInfo *SegmentInfoSender) (*RemotePaymentResponse, error) {
	if r == nil || r.node == nil || r.node.RemoteSignerAddr == nil {
		return nil, fmt.Errorf("remote signer not configured")
	}

	sess := segmentInfo.sess
	if sess == nil || sess.OrchestratorInfo == nil || sess.OrchestratorInfo.PriceInfo == nil {
		return nil, fmt.Errorf("missing session or OrchestratorInfo")
	}

	// Marshal OrchestratorInfo
	oInfoBytes, err := proto.Marshal(&net.PaymentResult{Info: sess.OrchestratorInfo})
	if err != nil {
		return nil, fmt.Errorf("error marshaling OrchestratorInfo for remote signer: %w", err)
	}

	r.mu.Lock()
	state := r.state
	r.mu.Unlock()

	priceInfo := segmentInfo.priceInfo

	// Build remote payment request
	reqPayload := RemotePaymentRequest{
		ManifestID:   segmentInfo.mid,
		Orchestrator: oInfoBytes,
		State:        state,
		PriceInfo:    priceInfo,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request payload for remote signer: %w", err)
	}

	remoteURL := r.node.RemoteSignerAddr.ResolveReference(&url.URL{Path: "/generate-live-payment"})
	httpReq, err := http.NewRequestWithContext(ctx, "POST", remoteURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call remote signer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == HTTPStatusRefreshSession {
		if segmentInfo.callCount > 3 {
			return nil, errors.New("too many consecutive session refreshes")
		}
		if err := refreshSession(ctx, sess, true); err != nil {
			return nil, fmt.Errorf("could not refresh session for remote signer: %w", err)
		}
		segmentInfo.callCount += 1
		return r.RequestPayment(ctx, segmentInfo)
	}

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote signer returned status %d: %s", resp.StatusCode, string(data))
	}

	var rp RemotePaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&rp); err != nil {
		return nil, fmt.Errorf("failed to decode remote signer response: %w", err)
	}

	// Cache updated state blob and signature
	r.mu.Lock()
	r.state = rp.State
	r.mu.Unlock()

	return &rp, nil
}

// SendPayment via remote signer: request tickets + seg creds from remote signer
// and then forward them to the orchestrator.
func (r *remotePaymentSender) SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error {
	rp, err := r.RequestPayment(ctx, segmentInfo)
	if err != nil {
		return err
	}

	// Forward payment + segment credentials to orchestrator
	url := segmentInfo.sess.OrchestratorInfo.Transcoder
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/payment", nil)
	if err != nil {
		clog.Errorf(ctx, "Could not generate payment request to orch=%s", url)
		return err
	}
	req.Header.Set(paymentHeader, rp.Payment)
	req.Header.Set(segmentHeader, rp.SegCreds)

	resp, err := sendReqWithTimeout(req, paymentRequestTimeout)
	if err != nil {
		clog.Errorf(ctx, "Could not send payment to orch=%s err=%q", url, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		clog.Errorf(ctx, "Orchestrator did not accept payment status=%d", resp.StatusCode)
		return fmt.Errorf("orchestrator did not accept payment, status=%d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		clog.Errorf(ctx, "Could not read response from orchestrator=%s err=%q", url, err)
		return err
	}

	// Update session to refresh ticket params from the response
	var pr net.PaymentResult
	err = proto.Unmarshal(data, &pr)
	if err != nil {
		clog.Errorf(ctx, "Could not unmarshal response from orchestrator=%s err=%q", url, err)
		return err
	}
	updateSession(segmentInfo.sess, &ReceivedTranscodeResult{Info: pr.Info})

	return nil
}

func calculateFee(inPixels int64, price *net.PriceInfo) *big.Rat {
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	return priceRat.Mul(priceRat, big.NewRat(inPixels, 1))
}
