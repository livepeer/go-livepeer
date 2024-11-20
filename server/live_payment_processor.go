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

	processSegmentFunc func(inPixels int64) error
}

type segment struct {
	timestamp time.Time
	segData   []byte
}

func NewLivePaymentProcessor(ctx context.Context, processInterval time.Duration, processSegmentFunc func(inPixels int64) error) *LivePaymentProcessor {
	pp := &LivePaymentProcessor{
		sender:             &livePaymentSender{segmentsToPayUpfront: 1},
		processInterval:    processInterval,
		segCh:              make(chan *segment, 1),
		processSegmentFunc: processSegmentFunc,
		lastProcessedAt:    time.Now(),
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
				slog.Info("Done processing payments for session")
				return
			}

		}
	}()
}

func (p *LivePaymentProcessor) processSegment(seg *segment) {
	if p.shouldSkip(seg.timestamp) {
		return
	}

	lastProcessedAt := p.lastProcessedAt

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

	err = p.processSegmentFunc(int64(pixelsSinceLastProcessed))
	if err != nil {
		slog.Error("Error processing payment", "err", err)
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
		io.Copy(pipeWriter, bytes.NewReader(seg.segData))
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
	if p.lastProcessedAt.Add(p.processInterval).After(timestamp) {
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
		case p.segCh <- &segment{timestamp: timestamp, segData: segData}:
		default:
			// We process one segment at the time, no need to buffer them
		}
	}()

	return resReader
}
