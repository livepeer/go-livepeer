package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/livepeer/lpms/ffmpeg"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
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
	segmentsToPayUpfront int64
}

type livePaymentReceiver struct {
	orchestrator Orchestrator
}

type LivePaymentProcessor struct {
	sender LivePaymentSender
	sess   *AISession

	processInterval       time.Duration
	setupFee              *big.Rat
	intervalsToPayUpfront int64

	lastProcessedAt time.Time
	lastProcessedMu sync.RWMutex
	segCh           chan *segment
}

type segment struct {
	reader *os.File
	now    time.Time
}

func NewLivePaymentProcessor(ctx context.Context, session *AISession, processInterval time.Duration, intervalsToPayUpfront int64) *LivePaymentProcessor {
	pp := &LivePaymentProcessor{
		sender:                &livePaymentSender{segmentsToPayUpfront: 1},
		sess:                  session,
		processInterval:       processInterval,
		intervalsToPayUpfront: intervalsToPayUpfront,
		segCh:                 make(chan *segment, 1),
	}
	pp.start(ctx)
	return pp
}

func (p *LivePaymentProcessor) start(ctx context.Context) {
	go func() {
		for {
			select {
			case seg := <-p.segCh:
				p.processSegment(seg)
			case <-ctx.Done():
				slog.Info("Done processing payments for session", "sessionID", p.sess.OrchestratorInfo.AuthToken.SessionId)
				return
			}

		}
	}()
}

func (p *LivePaymentProcessor) processSegment(seg *segment) {
	uuid := core.RandomManifestID()
	defer seg.reader.Close()

	if !p.worthProcessing(seg.now) {
		slog.Info("### NOT WORTH, CLosing")
		io.Copy(io.Discard, seg.reader)
		slog.Info("### Closed")
		return
	}

	lastProcessedAt := p.lastProcessedAt
	if lastProcessedAt.IsZero() {
		lastProcessedAt = seg.now.Add(-p.processInterval)
	}

	slog.Info("#### Publish 0", "uuid", uuid)
	fname := fmt.Sprintf("pipe:%d", seg.reader.Fd())
	status, info, err := ffmpeg.GetCodecInfo(fname)
	if err != nil {
		slog.Error("Error probing segment", "err", err)
		return
	}
	if status != ffmpeg.CodecStatusOk {
		slog.Error("Invalid CodecStatus while probing segment", "status", status)
		return
	}
	io.Copy(io.Discard, seg.reader)
	slog.Info("Probed segment", "info", info, "uuid", uuid)
	pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
	slog.Info("###### PUBLISH 1", "pixelsPerSec", pixelsPerSec, "uuid", uuid)
	secSinceLastProcessed := seg.now.Sub(lastProcessedAt).Seconds()
	slog.Info("###### PUBLISH 2", "secSinceLastProcessed", secSinceLastProcessed, "lastProcessedAt", lastProcessedAt, "now", seg.now, "uuid", uuid)
	pixelsSinceLastProcessed := pixelsPerSec * secSinceLastProcessed
	slog.Info("###### PUBLISH 3", "pixelsSinceLastProcessed", int64(pixelsSinceLastProcessed), "uuid", uuid)

	err = p.sender.SendPayment(context.Background(), &SegmentInfoSender{
		sess:      p.sess.BroadcastSession,
		inPixels:  int64(pixelsSinceLastProcessed),
		priceInfo: p.sess.OrchestratorInfo.PriceInfo,
	})
	if err != nil {
		slog.Error("Error sending payment", "err", err)
		return
	}
	p.lastProcessedMu.Lock()
	defer p.lastProcessedMu.Unlock()
	p.lastProcessedAt = seg.now
	slog.Info("###### PUBLISH 4", "lastProcessedAt", p.lastProcessedAt, "uuid", uuid)
}

func (p *LivePaymentProcessor) worthProcessing(timestamp time.Time) bool {
	p.lastProcessedMu.RLock()
	defer p.lastProcessedMu.RUnlock()
	if !p.lastProcessedAt.IsZero() && p.lastProcessedAt.Add(p.processInterval).After(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		slog.Info("##### Skipping payment processing", "lastProcessedAt", p.lastProcessedAt, "now", timestamp)
		return false
	}
	return true
}

func (p *LivePaymentProcessor) process(reader io.Reader) io.Reader {
	now := time.Now()
	if !p.worthProcessing(now) {
		return reader
	}

	slog.Info("##### Processing payment", "lastProcessedAt", p.lastProcessedAt, "now", now)

	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		slog.Error("Error creating pipe", "err", err)
		return reader
	}

	resReader := io.TeeReader(reader, pipeWriter)
	select {
	case p.segCh <- &segment{reader: pipeReader, now: now}:
	default:
		defer pipeReader.Close()
		io.Copy(io.Discard, pipeReader)
		slog.Error("Segment buffer full")
	}

	return resReader
}

func (r *livePaymentSender) SendPayment(ctx context.Context, segmentInfo *SegmentInfoSender) error {
	sess := segmentInfo.sess

	if err := refreshSessionIfNeeded(ctx, sess); err != nil {
		return err
	}

	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	// We pay a few segments upfront to avoid race condition between payment and segment processing
	minCredit := new(big.Rat).Mul(fee, new(big.Rat).SetInt64(r.segmentsToPayUpfront))
	balUpdate, err := newBalanceUpdate(sess, minCredit)
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
	fee := calculateFee(segmentInfo.inPixels, segmentInfo.priceInfo)

	balance := r.orchestrator.Balance(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID))
	if balance == nil || balance.Cmp(fee) < 0 {
		return errors.New("insufficient balance")
	}
	r.orchestrator.DebitFees(segmentInfo.sender, core.ManifestID(segmentInfo.sessionID), segmentInfo.priceInfo, segmentInfo.inPixels)
	clog.V(common.DEBUG).Infof(ctx, "Accounted payment for sessionID=%s, fee=%s", segmentInfo.sessionID, fee.FloatString(0))
	return nil
}

func calculateFee(inPixels int64, price *net.PriceInfo) *big.Rat {
	priceRat := big.NewRat(price.GetPricePerUnit(), price.GetPixelsPerUnit())
	return priceRat.Mul(priceRat, big.NewRat(inPixels, 1))
}
