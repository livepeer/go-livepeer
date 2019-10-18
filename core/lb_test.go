package core

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flyingmutant/rapid"
	"github.com/stretchr/testify/assert"

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

	shuffleProfiles := func(t *rapid.T) []ffmpeg.VideoProfile {
		// fisher-yates shuffle. or an approximation thereof. ( should test this)
		for i := len(profiles) - 1; i >= 1; i-- {
			j := rapid.IntsRange(0, i).Draw(t, "j").(int)
			profiles[i], profiles[j] = profiles[j], profiles[i]
		}
		nbProfs := rapid.IntsRange(1, len(profiles)-1).Draw(t, "nbProfs").(int)
		return profiles[:nbProfs]
	}

	// TODO state machine checks would probably be better here?
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

func TestLB_Transcode(t *testing.T) {
	// Property: Transcoder should be called with each call to Transcode.
	assert := assert.New(t)
	lb := NewLoadBalancingTranscoder("0,1", "", newStubTranscoder).(*LoadBalancingTranscoder)
	sessions := []string{"a", "b", "c", "d", "e"}

	rapid.Check(t, func(t *rapid.T) {
		sessIdx := rapid.IntsRange(0, len(sessions)-1).Draw(t, "sess").(int)
		sessName := sessions[sessIdx]
		sess, exists := lb.sessions[sessName]
		count := 0
		if exists {
			transcoder := sess.transcoder.(*StubTranscoder)
			count = transcoder.SegCount
		}
		lb.Transcode(sessName, "", []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9})
		if !exists {
			sess, exists = lb.sessions[sessName]
			assert.True(exists)
			assert.NotNil(sess)
		}
		transcoder := sess.transcoder.(*StubTranscoder)
		assert.Equal(count+1, transcoder.SegCount)
	})

	// Property: An error transcoding should propagate.
	// Test:     Inject an error into an existing transcode session.

	// Ensure session exists prior to transcoding so we can set the flag
	sessName := sessions[0]
	_, err := lb.Transcode(sessName, "", []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9})
	assert.Nil(err)
	sess, exists := lb.sessions[sessName]
	assert.True(exists)
	assert.NotNil(sess)
	transcoder := sess.transcoder.(*StubTranscoder)
	// Now set the fail flag in and check for propagation
	transcoder.FailTranscode = true
	_, err = lb.Transcode(sessName, "", []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9})
	assert.Equal(err, ErrTranscode)

	// Property: Cancelling a transcoder should remove session.
	oldContext := transcodeLoopContext
	defer func() { transcodeLoopContext = oldContext }()
	//var wg sync.WaitGroup
	stubCtx, stubCancel := context.WithCancel(context.Background())
	transcodeLoopContext = func() (context.Context, context.CancelFunc) { return stubCtx, stubCancel }
	profs := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	sess, err = lb.createSession("abc", "", profs)
	transcoder = sess.transcoder.(*StubTranscoder)
	assert.Nil(err, "Could not create session")
	assert.Equal(sess, lb.sessions["abc"])
	totalLoad := accumLoad(lb)
	stubCancel()
	time.Sleep(10 * time.Millisecond) // allow the cleanup routine to run
	assert.Nil(lb.sessions["abc"], "Session was not cleaned up")
	assert.Equal(1, transcoder.StoppedCount, "Session was not stopped")
	// Check load decreases
	assert.Equal(totalLoad-calculateCost(profs), accumLoad(lb), "Estimated load did not decrease")
}

func accumLoad(lb *LoadBalancingTranscoder) int {
	totalLoad := 0
	for _, v := range lb.load {
		totalLoad += v
	}
	return totalLoad
}

// Description of a rapid state machine for testing the load balancer
type lbMachine struct {
	lb *LoadBalancingTranscoder

	// Our model: various bits of internal state we want to synchronize with
	gpus map[string]string
}

func (m *lbMachine) Init(t *rapid.T) {
	var devices []string
	nbDevices := rapid.IntsRange(1, 100).Draw(t, "nbDevices").(int)
	for i := 0; i < nbDevices; i++ {
		devices = append(devices, strconv.Itoa(i))
	}

	m.lb = NewLoadBalancingTranscoder(strings.Join(devices, "."), "", newStubTranscoder).(*LoadBalancingTranscoder)
	m.gpus = make(map[string]string)
}

func (m *lbMachine) Transcode(t *rapid.T) {
	sessName := strconv.Itoa(rapid.IntsRange(0, 1000).Draw(t, "sess").(int))
	_, exists := m.gpus[sessName]
	if exists {
	}
}

func TestLB_Machine(t *testing.T) {
	rapid.Check(t, rapid.StateMachine(&lbMachine{}))
}
