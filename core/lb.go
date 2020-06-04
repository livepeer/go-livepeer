package core

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/lpms/ffmpeg"
)

var ErrTranscoderBusy = errors.New("TranscoderBusy")
var ErrTranscoderStopped = errors.New("TranscoderStopped")

type TranscoderSession interface {
	Transcoder
	Stop()
}

type newTranscoderFn func(device string) TranscoderSession

type LoadBalancingTranscoder struct {
	transcoders []string // Slice of device IDs
	newT        newTranscoderFn

	// The following fields need to be protected by the mutex `mu`
	mu       *sync.RWMutex
	load     map[string]int
	sessions map[string]*transcoderSession
	idx      int // Ensures a non-tapered work distribution
}

func NewLoadBalancingTranscoder(devices string, newTranscoderFn newTranscoderFn) Transcoder {
	d := strings.Split(devices, ",")
	return &LoadBalancingTranscoder{
		transcoders: d,
		newT:        newTranscoderFn,
		mu:          &sync.RWMutex{},
		load:        make(map[string]int),
		sessions:    make(map[string]*transcoderSession),
	}
}

func (lb *LoadBalancingTranscoder) Transcode(md *SegTranscodingMetadata) (*TranscodeData, error) {

	lb.mu.RLock()
	session, exists := lb.sessions[string(md.ManifestID)]
	lb.mu.RUnlock()
	if exists {
		glog.V(common.DEBUG).Info("LB: Using existing transcode session for ", session.key)
	} else {
		var err error
		session, err = lb.createSession(md)
		if err != nil {
			return nil, err
		}
	}
	return session.Transcode(md)
}

func (lb *LoadBalancingTranscoder) createSession(md *SegTranscodingMetadata) (*transcoderSession, error) {

	lb.mu.Lock()
	defer lb.mu.Unlock()

	job := string(md.ManifestID)
	if session, exists := lb.sessions[job]; exists {
		glog.V(common.DEBUG).Info("Attempted to create session but already exists ", session.key)
		return session, nil
	}

	glog.V(common.DEBUG).Info("LB: Creating transcode session for ", job)
	transcoder := lb.leastLoaded()

	// Acquire transcode session. Map to job id + assigned transcoder
	key := job + "_" + transcoder
	costEstimate := calculateCost(md.Profiles)
	session := &transcoderSession{
		transcoder:  lb.newT(transcoder),
		key:         key,
		sender:      make(chan *transcoderParams, 1),
		makeContext: transcodeLoopContext,
	}
	lb.sessions[job] = session
	lb.load[transcoder] += costEstimate
	lb.idx = (lb.idx + 1) % len(lb.transcoders)

	// Local cleanup function
	cleanupSession := func() {
		lb.mu.Lock()
		defer lb.mu.Unlock()
		_, exists := lb.sessions[job]
		if !exists {
			return
		}
		delete(lb.sessions, job)
		lb.load[transcoder] -= costEstimate
		glog.V(common.DEBUG).Info("LB: Deleted transcode session for ", session.key)
	}

	go func() {
		session.loop()
		cleanupSession()
	}()

	glog.V(common.DEBUG).Info("LB: Created transcode session for ", session.key)
	return session, nil
}

// Find the lowest loaded transcoder.
// Expects the mutex `lb.mu` to be locked by the caller.
func (lb *LoadBalancingTranscoder) leastLoaded() string {
	min, idx := math.MaxInt64, 0
	for i := 0; i < len(lb.transcoders); i++ {
		k := (i + lb.idx) % len(lb.transcoders)
		if lb.load[lb.transcoders[k]] < min {
			min = lb.load[lb.transcoders[k]]
			idx = k
		}
	}
	return lb.transcoders[idx]
}

type transcoderParams struct {
	md  *SegTranscodingMetadata
	res chan struct {
		*TranscodeData
		error
	}
}

type transcoderSession struct {
	transcoder TranscoderSession
	key        string

	sender      chan *transcoderParams
	makeContext func() (context.Context, context.CancelFunc)
}

func (sess *transcoderSession) loop() {
	defer func() {
		sess.transcoder.Stop()
		// Attempt to drain any pending messages in the channel.
		// Otherwise, write a message into the channel to fill it up.
		// Since we know the channel is buffered with size 1, any
		// successful writes here mean the channel is full
		// and we can safely exit immediately knowing that subsequent writes
		// will be rejected
		for {
			select {
			case params := <-sess.sender:
				params.res <- struct {
					*TranscodeData
					error
				}{nil, ErrTranscoderStopped}
			case sess.sender <- &transcoderParams{}:
				return
			default:
				continue
			}
		}
	}()

	// Run everything on a single loop to mitigate threading issues,
	//   especially around transcoder cleanup
	for {
		ctx, cancel := sess.makeContext()
		select {
		case <-ctx.Done():
			// Terminate the session after a period of inactivity
			glog.V(common.DEBUG).Info("LB: Transcode loop timed out for ", sess.key)
			return
		case params := <-sess.sender:
			cancel()
			res, err :=
				sess.transcoder.Transcode(params.md)
			params.res <- struct {
				*TranscodeData
				error
			}{res, err}
			if err != nil {
				glog.V(common.DEBUG).Info("LB: Stopping transcoder due to error for ", sess.key)
				return
			}
		}
	}
}

func (sess *transcoderSession) Transcode(md *SegTranscodingMetadata) (*TranscodeData, error) {
	params := &transcoderParams{md: md,
		res: make(chan struct {
			*TranscodeData
			error
		})}
	select {
	case sess.sender <- params:
		glog.V(common.DEBUG).Info("LB: Transcode submitted for ", sess.key)
	default:
		glog.V(common.DEBUG).Info("LB: Transcoder was busy; exiting ", sess.key)
		return nil, ErrTranscoderBusy
	}
	res := <-params.res
	return res.TranscodeData, res.error
}

func calculateCost(profiles []ffmpeg.VideoProfile) int {
	cost := 0
	for _, v := range profiles {
		w, h, err := ffmpeg.VideoProfileResolution(v)
		if err != nil {
			continue
		}
		framerate := int(v.Framerate)
		if 0 == framerate {
			// passthrough; estimate 30fps for load balancing purposes
			framerate = 30
		}
		framerateDen := int(v.FramerateDen)
		if 0 == framerateDen {
			// denominator unset; treat as 1
			framerateDen = 1
		}
		cost += w * h * (framerate / framerateDen) // TODO incorporate duration
	}
	return cost
}
