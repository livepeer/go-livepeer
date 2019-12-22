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

type TranscoderSession interface {
	Transcoder
	Stop()
}

type newTranscoderFn func(device string, workDir string) TranscoderSession

type LoadBalancingTranscoder struct {
	transcoders []string // Slice of device IDs
	workDir     string
	newT        newTranscoderFn

	// The following fields need to be protected by the mutex `mu`
	mu       *sync.RWMutex
	load     map[string]int
	sessions map[string]*transcoderSession
	idx      int // Ensures a non-tapered work distribution
}

func NewLoadBalancingTranscoder(devices string, workDir string, newTranscoderFn newTranscoderFn) Transcoder {
	d := strings.Split(devices, ",")
	return &LoadBalancingTranscoder{
		transcoders: d,
		workDir:     workDir,
		newT:        newTranscoderFn,
		mu:          &sync.RWMutex{},
		load:        make(map[string]int),
		sessions:    make(map[string]*transcoderSession),
	}
}

func (lb *LoadBalancingTranscoder) Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*TranscodeData, error) {

	lb.mu.RLock()
	session, exists := lb.sessions[job]
	lb.mu.RUnlock()
	if exists {
		glog.V(common.DEBUG).Info("LB: Using existing transcode session for ", session.key)
	} else {
		var err error
		session, err = lb.createSession(job, fname, profiles)
		if err != nil {
			return nil, err
		}
	}
	return session.Transcode(job, fname, profiles)
}

func (lb *LoadBalancingTranscoder) createSession(job string, fname string, profiles []ffmpeg.VideoProfile) (*transcoderSession, error) {

	lb.mu.Lock()
	defer lb.mu.Unlock()

	if session, exists := lb.sessions[job]; exists {
		glog.V(common.DEBUG).Info("Attempted to create session but already exists ", session.key)
		return session, nil
	}

	glog.V(common.DEBUG).Info("LB: Creating transcode session for ", job)
	transcoder := lb.leastLoaded()

	// Acquire transcode session. Map to job id + assigned transcoder
	key := job + "_" + transcoder
	costEstimate := calculateCost(profiles)
	session := &transcoderSession{
		transcoder:  lb.newT(transcoder, lb.workDir),
		key:         key,
		mu:          &sync.Mutex{},
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
	job      string
	fname    string
	profiles []ffmpeg.VideoProfile
	res      chan struct {
		*TranscodeData
		error
	}
}

type transcoderSession struct {
	transcoder TranscoderSession
	key        string
	mu         *sync.Mutex

	sender      chan *transcoderParams
	makeContext func() (context.Context, context.CancelFunc)
}

func (sess *transcoderSession) loop() {
	defer func() {
		sess.transcoder.Stop()
		sess.mu.Lock()
		defer sess.mu.Unlock()
		sess.sender = nil // unblocks any sender + select{ ... }
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
				sess.transcoder.Transcode(params.job, params.fname, params.profiles)
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

func (sess *transcoderSession) Transcode(job string, fname string, profiles []ffmpeg.VideoProfile) (*TranscodeData, error) {
	params := &transcoderParams{job: job, fname: fname, profiles: profiles,
		res: make(chan struct {
			*TranscodeData
			error
		})}
	sess.mu.Lock()
	defer sess.mu.Unlock()
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
		cost += w * h * int(v.Framerate) // TODO incorporate duration
	}
	return cost
}
