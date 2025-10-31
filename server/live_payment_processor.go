package server

import (
	"context"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/lpms/ffmpeg"
)

var defaultSegInfo = ffmpeg.MediaFormatInfo{
	Height: 720,
	Width:  1280,
	FPS:    30.0,
}

type LivePaymentProcessor struct {
	interval time.Duration

	lastProcessedAt time.Time
	lastProcessedMu sync.RWMutex
	processCh       chan time.Time

	processSegmentFunc func(inPixels int64) error
}

type segment struct {
	timestamp time.Time
	segData   []byte
}

func NewLivePaymentProcessor(ctx context.Context, processInterval time.Duration, processSegmentFunc func(inPixels int64) error) *LivePaymentProcessor {
	pp := &LivePaymentProcessor{
		interval: processInterval,

		processCh:          make(chan time.Time, 1),
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
			case timestamp := <-p.processCh:
				p.processOne(ctx, timestamp)
			case <-ctx.Done():
				clog.Info(ctx, "Done processing payments for session")
				return
			}
		}
	}()
}

func (p *LivePaymentProcessor) processOne(ctx context.Context, timestamp time.Time) {
	if p.shouldSkip(timestamp) {
		return
	}

	info := defaultSegInfo

	pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
	secSinceLastProcessed := timestamp.Sub(p.lastProcessedAt).Seconds()
	pixelsSinceLastProcessed := pixelsPerSec * secSinceLastProcessed
	clog.Info(ctx, "Processing live payment", "secsSinceLastProcessed", secSinceLastProcessed, "pixelsSinceLastProcessed", pixelsSinceLastProcessed)

	err := p.processSegmentFunc(int64(pixelsSinceLastProcessed))
	if err != nil {
		clog.InfofErr(ctx, "Error processing payment", err)
		// Temporarily ignore failing payments, because they are not critical while we're using our own Os
		// return
	}

	p.lastProcessedMu.Lock()
	defer p.lastProcessedMu.Unlock()
	p.lastProcessedAt = timestamp
}

func (p *LivePaymentProcessor) process(ctx context.Context) {
	timestamp := time.Now()
	if p.shouldSkip(timestamp) {
		// Only need to process segments periodically
		return
	}

	go func() {
		select {
		case p.processCh <- timestamp:
		default:
			// We process one segment at the time, no need to buffer them
		}
	}()

	return
}

func (p *LivePaymentProcessor) shouldSkip(timestamp time.Time) bool {
	p.lastProcessedMu.RLock()
	lastProcessedAt := p.lastProcessedAt
	p.lastProcessedMu.RUnlock()
	if lastProcessedAt.Add(p.interval).After(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		return true
	}
	return false
}
