package server

import (
	"context"
	"fmt"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/lpms/ffmpeg"
	"io"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"time"
)

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
