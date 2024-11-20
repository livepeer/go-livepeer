package server

import (
	"bytes"
	"context"
	"fmt"
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
	reader    *os.File
	timestamp time.Time
	segData   []byte
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
	//defer seg.reader.Close()
	//defer io.Copy(io.Discard, seg.reader)

	if p.shouldSkip(seg.timestamp) {
		//io.Copy(io.Discard, seg.reader)
		return
	}

	lastProcessedAt := p.lastProcessedAt
	if lastProcessedAt.IsZero() {
		lastProcessedAt = seg.timestamp.Add(-p.processInterval)
	}

	info, err := probeSegment(seg)
	if err != nil {
		slog.Error("Error probing segment", "err", err)
		return
	}

	pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
	slog.Info("###### PUBLISH 1", "pixelsPerSec", pixelsPerSec)
	secSinceLastProcessed := seg.timestamp.Sub(lastProcessedAt).Seconds()
	slog.Info("###### PUBLISH 2", "secSinceLastProcessed", secSinceLastProcessed, "lastProcessedAt", lastProcessedAt, "timestamp", seg.timestamp)
	pixelsSinceLastProcessed := pixelsPerSec * secSinceLastProcessed
	slog.Info("###### PUBLISH 3", "pixelsSinceLastProcessed", int64(pixelsSinceLastProcessed))

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
	p.lastProcessedAt = seg.timestamp
	slog.Info("###### PUBLISH 4", "lastProcessedAt", p.lastProcessedAt)
}

func probeSegment(seg *segment) (ffmpeg.MediaFormatInfo, error) {
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		return ffmpeg.MediaFormatInfo{}, err
	}

	go func() {
		defer pipeWriter.Close()
		if _, err := io.Copy(pipeWriter, bytes.NewReader(seg.segData)); err != nil {
			slog.Error("failed to copy data to pipe", "err", err)
		}
	}()

	fname := fmt.Sprintf("pipe:%d", pipeReader.Fd())
	status, info, err := ffmpeg.GetCodecInfo(fname)
	if err != nil {
		return ffmpeg.MediaFormatInfo{}, err
	}
	if status != ffmpeg.CodecStatusOk {
		slog.Error("Invalid CodecStatus while probing segment", "status", status)
		return ffmpeg.MediaFormatInfo{}, fmt.Errorf("invalid CodecStatus while probing segment, status=%d", status)
	}
	slog.Info("Probed segment", "info", info)
	return info, nil
}

func (p *LivePaymentProcessor) shouldSkip(timestamp time.Time) bool {
	p.lastProcessedMu.RLock()
	defer p.lastProcessedMu.RUnlock()
	if !p.lastProcessedAt.IsZero() && p.lastProcessedAt.Add(p.processInterval).After(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		slog.Info("##### Skipping payment processing", "lastProcessedAt", p.lastProcessedAt, "timestamp", timestamp)
		return true
	}
	return false
}

func (p *LivePaymentProcessor) process(reader io.Reader) io.Reader {
	timestamp := time.Now()
	if p.shouldSkip(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		return reader
	}

	slog.Info("##### Processing payment", "lastProcessedAt", p.lastProcessedAt, "timestamp", timestamp)

	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		slog.Error("Error creating pipe", "err", err)
		return reader
	}

	resReader := io.TeeReader(reader, pipeWriter)
	go func() {
		defer pipeReader.Close()
		segData, err := io.ReadAll(pipeReader)
		if err != nil {
			slog.Error("Error reading segment data", "err", err)
			return
		}
		slog.Info("##### Read segData", "segData", len(segData))

		select {
		case p.segCh <- &segment{reader: pipeReader, timestamp: timestamp, segData: segData}:
		default:
			// We process one segment at the time, no need to buffer them
		}
	}()

	return resReader
}
