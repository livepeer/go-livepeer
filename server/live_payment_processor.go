package server

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/media"
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

	lastProbedAt        time.Time
	lastProbedSegInfoMu sync.RWMutex
	lastProbedSegInfo   *ffmpeg.MediaFormatInfo
	probeSegCh          chan *segment

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

		lastProbedAt:      time.Now(),
		lastProbedSegInfo: &defaultSegInfo,
		probeSegCh:        make(chan *segment, 1),
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
	go func() {
		for {
			select {
			case seg := <-p.probeSegCh:
				p.probeOne(ctx, seg)
			case <-ctx.Done():
				clog.Info(ctx, "Done probing segments for session")
				return
			}

		}
	}()
}

func (p *LivePaymentProcessor) processOne(ctx context.Context, timestamp time.Time) {
	if p.shouldSkip(timestamp) {
		return
	}

	p.lastProbedSegInfoMu.RLock()
	info := p.lastProbedSegInfo
	p.lastProbedSegInfoMu.RUnlock()

	pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
	secSinceLastProcessed := timestamp.Sub(p.lastProcessedAt).Seconds()
	pixelsSinceLastProcessed := pixelsPerSec * secSinceLastProcessed
	clog.V(6).Infof(ctx, "Processing live payment: secSinceLastProcessed=%v, pixelsSinceLastProcessed=%v, height=%d, width=%d, FPS=%v", secSinceLastProcessed, pixelsSinceLastProcessed, info.Height, info.Width, info.FPS)

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

func (p *LivePaymentProcessor) process(ctx context.Context, reader media.CloneableReader) {
	timestamp := time.Now()
	if p.shouldSkip(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		return
	}

	go func() {
		select {
		case p.processCh <- timestamp:
		default:
			// We process one segment at the time, no need to buffer them
		}

		// read the segment into the buffer, because the direct use of the reader causes Broken pipe
		// it's probably related to different pace of reading by trickle and ffmpeg.GetCodecInfo()
		segData, err := io.ReadAll(reader.Clone())
		if err != nil {
			clog.InfofErr(ctx, "Error reading segment data", err)
			return
		}

		select {
		case p.probeSegCh <- &segment{timestamp: timestamp, segData: segData}:
		default:
			// We process one segment at the time, no need to buffer them
		}
	}()
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

func (p *LivePaymentProcessor) probeOne(ctx context.Context, seg *segment) {
	if p.lastProbedAt.Add(p.interval).After(seg.timestamp) {
		// We don't probe every segment, because it's too compute-expensive
		return
	}

	info, err := probeSegment(ctx, seg)
	if err != nil {
		clog.Warningf(ctx, "Error probing segment, err=%v", err)
		return
	}
	clog.V(6).Infof(ctx, "Probed segment: height=%d, width=%d, FPS=%v", info.Height, info.Width, info.FPS)

	p.lastProbedSegInfoMu.Lock()
	defer p.lastProbedSegInfoMu.Unlock()
	p.lastProbedSegInfo = &info
	p.lastProbedAt = seg.timestamp
}

func probeSegment(ctx context.Context, seg *segment) (ffmpeg.MediaFormatInfo, error) {
	// Return a constant value to calculate payments based on time intervals rather than input segment pixel data
	return defaultSegInfo, nil
}
