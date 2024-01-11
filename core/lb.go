package core

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/livepeer/go-livepeer/clog"
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
	transcoders   []string // Slice of device IDs
	newT          newTranscoderFn
	detectorModel string

	// The following fields need to be protected by the mutex `mu`
	mu       *sync.RWMutex
	load     map[string]int
	sessions map[string]*transcoderSession
	idx      int // Ensures a non-tapered work distribution
}

func (lb *LoadBalancingTranscoder) EndTranscodingSession(sessionId string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if session, exists := lb.sessions[sessionId]; exists {
		// delete session id here to avoid the race
		delete(lb.sessions, sessionId)
		// signal transcode loop finish for this session
		close(session.stop)
		clog.V(common.DEBUG).Infof(context.TODO(), "LB: Transcode session id=%s teared down", session.key)
	} else {
		clog.V(common.DEBUG).Infof(context.TODO(), "LB: Transcode session id=%s already finished", sessionId)
	}
}

func NewLoadBalancingTranscoder(devices []string, newTranscoderFn newTranscoderFn) Transcoder {
	return &LoadBalancingTranscoder{
		transcoders: devices,
		newT:        newTranscoderFn,
		mu:          &sync.RWMutex{},
		load:        make(map[string]int),
		sessions:    make(map[string]*transcoderSession),
	}
}

func (lb *LoadBalancingTranscoder) Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error) {
	lb.mu.RLock()
	session, exists := lb.sessions[string(md.AuthToken.SessionId)]
	lb.mu.RUnlock()
	if exists {
		clog.V(common.DEBUG).Infof(ctx, "LB: Using existing transcode session for key=%s", session.key)
		if md != nil && md.SegmentParameters != nil && md.SegmentParameters.ForceSessionReinit {
			// Broadcaster requested HW session reinitialization
			lb.mu.Lock()
			session.transcoder.Stop()
			session.transcoder = lb.newT(lb.leastLoaded())
			lb.mu.Unlock()
		}
	} else {
		var err error
		session, err = lb.createSession(clog.Clone(context.Background(), ctx), md)
		if err != nil {
			return nil, err
		}
	}
	return session.Transcode(ctx, md)
}

func (lb *LoadBalancingTranscoder) createSession(ctx context.Context, md *SegTranscodingMetadata) (*transcoderSession, error) {

	lb.mu.Lock()
	defer lb.mu.Unlock()

	job := string(md.AuthToken.SessionId)
	if session, exists := lb.sessions[job]; exists {
		clog.V(common.DEBUG).Infof(ctx, "Attempted to create session but already exists key=%s", session.key)
		return session, nil
	}

	clog.V(common.DEBUG).Infof(ctx, "LB: Creating transcode session for job=%s", job)
	transcoder := lb.leastLoaded()

	// Acquire transcode session. Map to job id + assigned transcoder
	key := job + "_" + transcoder
	costEstimate := calculateCost(md.Profiles)

	// create the transcoder
	session := &transcoderSession{
		transcoder:  lb.newT(transcoder),
		key:         key,
		done:        make(chan struct{}),
		stop:        make(chan struct{}),
		sender:      make(chan *transcoderParams, maxSegmentChannels),
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
		if exists {
			delete(lb.sessions, job)
		}
		lb.load[transcoder] -= costEstimate
		clog.V(common.DEBUG).Infof(ctx, "LB: Deleted transcode session for key=%s", session.key)
	}

	go func() {
		session.loop(ctx)
		cleanupSession()
	}()

	clog.V(common.DEBUG).Infof(ctx, "LB: Created transcode session for key=%s", session.key)
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
	ctx context.Context
	md  *SegTranscodingMetadata
	res chan struct {
		*TranscodeData
		error
	}
}

type transcoderSession struct {
	transcoder TranscoderSession
	key        string

	sender chan *transcoderParams
	// channel to handle Orchestrator error or shutdown during transcoding
	done chan struct{}
	// channel to signal transcoding loop stop, done channel is not used when not transcoding
	stop        chan struct{}
	makeContext func() (context.Context, context.CancelFunc)
}

func (sess *transcoderSession) loop(logCtx context.Context) {
	defer func() {
		sess.transcoder.Stop()
		// Close the done channel to signal the sender(s) that the
		// transcode loop has stopped
		close(sess.done)
	}()

	// Run everything on a single loop to mitigate threading issues,
	//   especially around transcoder cleanup
	for {
		ctx, cancel := sess.makeContext()
		select {
		case <-sess.stop:
			// Terminate the session after a period of inactivity
			clog.V(common.DEBUG).Infof(logCtx, "LB: Transcode loop stopped for key=%s", sess.key)
			return
		case <-ctx.Done():
			// Terminate the session after a period of inactivity
			clog.V(common.DEBUG).Infof(logCtx, "LB: Transcode loop timed out for key=%s", sess.key)
			return
		case params := <-sess.sender:
			cancel()
			res, err :=
				sess.transcoder.Transcode(params.ctx, params.md)
			params.res <- struct {
				*TranscodeData
				error
			}{res, err}
			if err != nil {
				clog.V(common.DEBUG).Infof(logCtx, "LB: Stopping transcoder due to error for key=%s", sess.key)
				return
			}
		}
	}
}

func (sess *transcoderSession) Transcode(ctx context.Context, md *SegTranscodingMetadata) (*TranscodeData, error) {
	params := &transcoderParams{
		md:  md,
		ctx: ctx,
		res: make(chan struct {
			*TranscodeData
			error
		})}
	select {
	case sess.sender <- params:
		clog.V(common.DEBUG).Infof(ctx, "LB: Transcode submitted for key=%s", sess.key)
	default:
		clog.V(common.DEBUG).Infof(ctx, "LB: Transcoder was busy; exiting key=%s", sess.key)
		return nil, ErrTranscoderBusy
	}
	select {
	case res := <-params.res:
		return res.TranscodeData, res.error
	case <-sess.done:
		return nil, ErrTranscoderStopped
	}
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
