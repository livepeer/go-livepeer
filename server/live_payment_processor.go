package server

import (
	"bytes"
	"context"
	"fmt"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/lpms/ffmpeg"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

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
	defaultSegInfo := &ffmpeg.MediaFormatInfo{Height: 480, Width: 640, FPS: 30.0}
	pp := &LivePaymentProcessor{
		interval: processInterval,

		processCh:          make(chan time.Time, 1),
		processSegmentFunc: processSegmentFunc,
		lastProcessedAt:    time.Now(),

		lastProbedAt:      time.Now(),
		lastProbedSegInfo: defaultSegInfo,
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
				p.processOne(timestamp)
			case <-ctx.Done():
				slog.Info("Done processing payments for session")
				return
			}

		}
	}()
	go func() {
		for {
			select {
			case seg := <-p.probeSegCh:
				p.probeOne(seg)
			case <-ctx.Done():
				slog.Info("Done probing segments for session")
				return
			}

		}
	}()
}

func (p *LivePaymentProcessor) processOne(timestamp time.Time) {
	if p.shouldSkip(timestamp) {
		return
	}

	p.lastProbedSegInfoMu.RLock()
	info := p.lastProbedSegInfo
	p.lastProbedSegInfoMu.RUnlock()

	pixelsPerSec := float64(info.Height) * float64(info.Width) * float64(info.FPS)
	secSinceLastProcessed := timestamp.Sub(p.lastProcessedAt).Seconds()
	pixelsSinceLastProcessed := pixelsPerSec * secSinceLastProcessed
	clog.V(6).Infof(context.Background(), "Processing live payment: secSinceLastProcessed=%v, pixelsSinceLastProcessed=%v, height=%d, width=%d, FPS=%v", secSinceLastProcessed, pixelsSinceLastProcessed, info.Height, info.Width, info.FPS)

	err := p.processSegmentFunc(int64(pixelsSinceLastProcessed))
	if err != nil {
		slog.Error("Error processing payment", "err", err)
		// Temporarily ignore failing payments, because they are not critical while we're using our own Os
		// return
	}

	p.lastProcessedMu.Lock()
	defer p.lastProcessedMu.Unlock()
	p.lastProcessedAt = timestamp
}

func (p *LivePaymentProcessor) process(reader io.Reader) io.Reader {
	timestamp := time.Now()
	if p.shouldSkip(timestamp) {
		// We don't process every segment, because it's too compute-expensive
		return reader
	}

	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		slog.Error("Error creating pipe", "err", err)
		return reader
	}

	resReader := io.TeeReader(reader, pipeWriter)
	go func() {
		select {
		case p.processCh <- timestamp:
		default:
			// We process one segment at the time, no need to buffer them
		}

		// read the segment into the buffer, because the direct use of the reader causes Broken pipe
		// it's probably related to different pace of reading by trickle and ffmpeg.GetCodecInfo()
		defer pipeReader.Close()
		segData, err := io.ReadAll(pipeReader)
		if err != nil {
			slog.Error("Error reading segment data", "err", err)
			return
		}

		select {
		case p.probeSegCh <- &segment{timestamp: timestamp, segData: segData}:
		default:
			// We process one segment at the time, no need to buffer them
		}
	}()

	return resReader
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

func (p *LivePaymentProcessor) probeOne(seg *segment) {
	if p.lastProbedAt.Add(p.interval).After(seg.timestamp) {
		// We don't probe every segment, because it's too compute-expensive
		return
	}

	info, err := probeSegment(seg)
	if err != nil {
		clog.Warningf(context.Background(), "Error probing segment, err=%v", err)
		return
	}
	clog.V(6).Infof(context.Background(), "Probed segment: height=%d, width=%d, FPS=%v", info.Height, info.Width, info.FPS)

	p.lastProbedSegInfoMu.Lock()
	defer p.lastProbedSegInfoMu.Unlock()
	p.lastProbedSegInfo = &info
	p.lastProbedAt = seg.timestamp
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

	// For WebRTC the probing sometimes returns FPS=90000, which is incorrect and causes issues with payment,
	// so as a hack let's hardcode FPS to 30
	info.FPS = 30.0
	return info, nil
}
