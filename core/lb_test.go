package core

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flyingmutant/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/livepeer/lpms/ffmpeg"
)

func TestLB_LeastLoaded(t *testing.T) {
	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1,2,3,4", "", newStubTranscoder).(*LoadBalancingTranscoder)
	rapid.Check(t, func(t *rapid.T) {
		cost := rapid.IntsRange(1, 10).Draw(t, "cost").(int)
		transcoder := lb.leastLoaded()
		// ensure we selected the minimum cost
		lb.load[transcoder] += cost
		currentLoad := lb.load[transcoder]
		for k, v := range lb.load {
			if k == transcoder {
				continue
			}
			assert.LessOrEqual(currentLoad, v+cost, "Would have been less loaded")
		}
	})
}

func TestLB_Ratchet(t *testing.T) {
	// Property: After assigning a new session to a transcoder,
	//           increment the starting index for the next search
	//           Also ensure wraparound.

	// Test:     Two transcoders, several sessions with the same set of profiles
	//           Run multiple transcodes.
	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1", "", newStubTranscoder).(*LoadBalancingTranscoder)
	sessions := []string{"a", "b", "c", "d", "e"}

	rapid.Check(t, func(t *rapid.T) {
		sessIdx := rapid.IntsRange(0, len(sessions)-1).Draw(t, "sess").(int)
		sess := sessions[sessIdx]
		_, exists := lb.sessions[sess]
		idx := lb.idx
		lb.Transcode(sess, "", []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9})
		if exists {
			assert.Equal(idx, lb.idx)
		} else {
			assert.Equal((idx+1)%len(lb.transcoders), lb.idx)
		}
	})
}

func TestLB_LoadAssignment(t *testing.T) {

	// Property: Overall load only increases after first segment

	// Test :    Randomize profiles to randomize "load" **per segment**;
	//           Subsequent segments should ignore subsequent load costs.

	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1,2,3,4", "", newStubTranscoder).(*LoadBalancingTranscoder)
	sessions := []string{"a", "b", "c", "d", "e"}
	profiles := []ffmpeg.VideoProfile{}
	for _, v := range ffmpeg.VideoProfileLookup {
		profiles = append(profiles, v)
	}

	rapid.Check(t, func(t *rapid.T) {
		sessIdx := rapid.IntsRange(0, len(sessions)-1).Draw(t, "sess").(int)
		sessName := sessions[sessIdx]
		profs := shuffleProfiles(t)
		_, exists := lb.sessions[sessName]
		totalLoad := accumLoad(lb)
		lb.Transcode(sessName, "", profs)
		if exists {
			assert.Equal(totalLoad, accumLoad(lb))
		} else {
			assert.Contains(lb.sessions, sessName, "Transcoder did not establish session")
			assert.Equal(totalLoad+calculateCost(profs), accumLoad(lb))
		}
	})
}

func TestLB_SessionCancel(t *testing.T) {
	// One-off test for session cancellation to work around thread safety issues
	stubCtx, stubCancel := context.WithCancel(context.Background())
	ctxFunc := func() (context.Context, context.CancelFunc) { return stubCtx, stubCancel }

	sess := &transcoderSession{
		transcoder:  newStubTranscoder("", ""),
		mu:          &sync.Mutex{},
		sender:      make(chan *transcoderParams, 1),
		makeContext: ctxFunc,
	}

	wg := newWg(1)
	go func() {
		sess.loop()
		wg.Done()
	}()
	stubCancel()
	wgWait(wg)
	sess.mu.Lock()
	assert.Nil(t, sess.sender)
	sess.mu.Unlock()
}

func TestLB_SessionConcurrency(t *testing.T) {

	stubCtx, stubCancel := context.WithCancel(context.Background())
	ctxFunc := func() (context.Context, context.CancelFunc) { return stubCtx, stubCancel }

	sess := &transcoderSession{
		transcoder:  newStubTranscoder("", ""),
		mu:          &sync.Mutex{},
		sender:      make(chan *transcoderParams, 1),
		makeContext: ctxFunc,
	}

	wg := newWg(1)
	go func() {
		sess.loop()
		wg.Done()
	}()

	iters := 100
	for i := 0; i < iters; i++ {
		if i == iters/2 {
			stubCancel()
		}
		wg.Add(1)
		go func() {
			sess.Transcode("", "", []ffmpeg.VideoProfile{})
			wg.Done()
		}()
	}
	wgWait(wg)
}

func accumLoad(lb *LoadBalancingTranscoder) int {
	totalLoad := 0
	for _, v := range lb.load {
		totalLoad += v
	}
	return totalLoad
}

func shuffleProfiles(t *rapid.T) []ffmpeg.VideoProfile {
	// fisher-yates shuffle. or an approximation thereof. (should test this)
	profiles := []ffmpeg.VideoProfile{}
	for _, v := range ffmpeg.VideoProfileLookup {
		profiles = append(profiles, v)
	}
	for i := len(profiles) - 1; i >= 1; i-- {
		j := rapid.IntsRange(0, i).Draw(t, "j").(int)
		profiles[i], profiles[j] = profiles[j], profiles[i]
	}
	nbProfs := rapid.IntsRange(1, len(profiles)-1).Draw(t, "nbProfs").(int)
	return profiles[:nbProfs]
}

type machineState struct {
	segs     int
	load     int
	cancel   *context.CancelFunc
	profiles []ffmpeg.VideoProfile
}

// Description of a rapid state machine for testing the load balancer
type lbMachine struct {
	lb *LoadBalancingTranscoder

	// Our model: various bits of internal state we want to synchronize with
	states    map[string]*machineState
	totalLoad int
}

func (m *lbMachine) randomSession(t *rapid.T) (string, *machineState) {
	// Create an internal session
	// Doesn't actually create it on the transcoder - should we?
	sessName := strconv.Itoa(rapid.IntsRange(0, 25).Draw(t, "sess").(int))

	// Create internal state if necessary
	state, exists := m.states[sessName]
	if exists {
		return sessName, state
	}

	profs := shuffleProfiles(t)
	state = &machineState{profiles: profs}
	m.states[sessName] = state
	m.totalLoad += calculateCost(profs)

	return sessName, state
}

func (m *lbMachine) Init(t *rapid.T) {
	var devices []string
	nbDevices := rapid.IntsRange(1, 10).Draw(t, "nbDevices").(int)
	for i := 0; i < nbDevices; i++ {
		devices = append(devices, strconv.Itoa(i))
	}

	m.lb = NewLoadBalancingTranscoder(strings.Join(devices, ","), "", newStubTranscoder).(*LoadBalancingTranscoder)
	m.states = make(map[string]*machineState)

	assert.Equal(t, devices, m.lb.transcoders) // sanity check
}

func (m *lbMachine) TranscodeOK(t *rapid.T) {
	// Run a successful segment transcode

	sessName, state := m.randomSession(t)
	_, err := m.lb.Transcode(sessName, "", state.profiles)

	assert.Nil(t, err)

	// Update internal state
	state.segs++
}

func (m *lbMachine) TranscodeError(t *rapid.T) {
	// Run a failed segment transcode

	sessName, state := m.randomSession(t)

	// If session doesn't already exist, create it by forcing a transcode
	_, ok := m.lb.sessions[sessName]
	if !ok {
		_, err := m.lb.Transcode(sessName, "", state.profiles)
		assert.Nil(t, err)
		require.Contains(t, m.lb.sessions, sessName)
	}
	transcoder, ok := m.lb.sessions[sessName].transcoder.(*StubTranscoder)
	require.True(t, ok, "Transcoder was not a StubTranscoder")
	require.Equal(t, 0, transcoder.StoppedCount) // Sanity check

	transcoder.FailTranscode = true
	_, err := m.lb.Transcode(sessName, "", state.profiles)
	assert.Equal(t, ErrTranscode, err)

	m.totalLoad -= calculateCost(state.profiles)
	delete(m.states, sessName)

	// Give time for the transcode and session to stop
	for retryCount := 0; retryCount < 100; retryCount++ {
		m.lb.mu.Lock()
		_, exists := m.lb.sessions[sessName]
		m.lb.mu.Unlock()
		if !exists {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}

	// Confirm things stopped
	assert.NotContains(t, m.lb.sessions, sessName)
	assert.Equal(t, 1, transcoder.StoppedCount, "Transcoder did not stop")
}

func (m *lbMachine) Check(t *rapid.T) {
	assert := assert.New(t)

	m.lb.mu.RLock()
	defer m.lb.mu.RUnlock()
	assert.Equal(len(m.states), len(m.lb.sessions), "Mismatch in number of sessions")
	assert.Equal(m.totalLoad, accumLoad(m.lb), "Mismatch in load calculation")

	for name, sess := range m.lb.sessions {
		transcoder, ok := sess.transcoder.(*StubTranscoder)
		require.True(t, ok, "Not a stub transcoder!")
		require.Contains(t, m.states, name)
		assert.Equal(m.states[name].segs, transcoder.SegCount)
	}
}

func TestLB_Machine(t *testing.T) {
	rapid.Check(t, rapid.StateMachine(&lbMachine{}))
}

func TestLB_Busy(t *testing.T) {
	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1,2,3,4", "", newStubTranscoder).(*LoadBalancingTranscoder)
	sess, err := lb.createSession("jo1", "f1", nil)
	assert.Nil(err)
	assert.NotNil(sess)
	stubTrans, ok := sess.transcoder.(*StubTranscoder)
	require.True(t, ok, "Transcoder was not a StubTranscoder")
	stubTrans.TranscodeWait = 500 * time.Millisecond
	stubTrans.Started = make(chan interface{}, 10)
	var wg sync.WaitGroup
	wg.Add(3)
	seq := make(chan interface{})
	var f3e, f4e error
	go func() {
		_, err = sess.Transcode("jo1", "f2", nil)
		assert.Nil(err)
		wg.Done()
	}()
	go func() {
		<-stubTrans.Started // need to make sure that transcoding loop took first job from the channel
		seq <- nil
		_, f3e = sess.Transcode("jo1", "f3", nil)
		wg.Done()
	}()
	go func() {
		<-seq
		_, f4e = sess.Transcode("jo1", "f4", nil)
		wg.Done()
	}()
	wg.Wait()
	assert.Equal(2, stubTrans.SegCount)
	busyErrors := 0
	if f3e == ErrTranscoderBusy {
		busyErrors++
	}
	if f4e == ErrTranscoderBusy {
		busyErrors++
	}
	assert.Equal(1, busyErrors, "Should be exactly one TranscoderBusy error, instead got %d", busyErrors)
}

func TestLB_Race(t *testing.T) {
	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1,2,3,4", "", newStubTranscoder).(*LoadBalancingTranscoder)
	sess, err := lb.createSession("jo1", "f1", nil)
	stubTrans, ok := sess.transcoder.(*StubTranscoder)
	require.True(t, ok, "Transcoder was not a StubTranscoder")
	stubTrans.FailWait = 200 * time.Millisecond
	stubTrans.Started = make(chan interface{}, 10)
	assert.NoError(err)
	assert.NotNil(sess)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		<-stubTrans.Started // need to make sure that transcoding loop took first job from the channel
		_, err := sess.Transcode("jo1", "f2", nil)
		assert.Equal(ErrTranscoderStopped, err)
		wg.Done()
	}()
	go func() {
		_, err := sess.Transcode("jo1", "f3", nil)
		assert.Equal(ErrTranscode, err)
		wg.Done()
	}()
	wg.Wait()
	assert.Equal(1, stubTrans.SegCount)
}

func TestLB_SessionErrors(t *testing.T) {
	for j := 0; j < 100; j++ {
		stubCtx, stubCancel := context.WithCancel(context.Background())
		ctxFunc := func() (context.Context, context.CancelFunc) { return stubCtx, stubCancel }

		tcCtx, tcCancel := context.WithCancel(context.Background())
		transcoder := stubTranscoderWithProfiles(nil)
		transcoder.TranscodeFn = func() { <-tcCtx.Done() }
		sess := &transcoderSession{
			transcoder:  transcoder,
			mu:          &sync.Mutex{},
			sender:      make(chan *transcoderParams, 1),
			makeContext: ctxFunc,
		}

		go sess.loop()

		iters := 100
		errCh := make(chan int)
		for i := 0; i < iters; i++ {
			if i == iters/2 {
				stubCancel()
			}
			if i == 2*iters/3 {
				tcCancel()
			}
			go func(ch chan int) {
				_, err := sess.Transcode("", "", []ffmpeg.VideoProfile{})
				if err == nil {
					ch <- 0
				} else {
					ch <- 1
					if err != ErrTranscoderBusy && err != ErrTranscoderStopped {
						t.Error("Unexpected error from transcoder ", err)
					}
				}
			}(errCh)
		}

		errCount := 0
		timeout := time.After(100 * time.Second)
		for i := 0; i < iters; i++ {
			select {
			case k := <-errCh:
				errCount += k
			case <-timeout:
				t.Error("Stopped because of timeout")
				break
			}
		}
		assert.Equal(t, iters, errCount+transcoder.SegCount)
		assert.Greater(t, errCount, int(float64(iters)*0.9))
	}
}
