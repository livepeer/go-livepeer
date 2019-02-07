package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

func Over1Pct(val int, cmp int) bool {
	return float32(val) > float32(cmp)*1.01 || float32(val) < float32(cmp)*0.99
}

func StubSegment() *stream.HLSSegment {
	d, _ := ioutil.ReadFile("./test.ts")
	return &stream.HLSSegment{SeqNo: 100, Name: "test.ts", Data: d[0:402696], Duration: 1}
}

func StubJobId() int64 {
	return int64(1234)
}

var videoProfiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}

func TestTranscode(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, err := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	if err != nil {
		t.Error("Error initializing DB: ", err)
	}
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()

	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}

	// Check nil transcoder.
	tr, err := n.sendToTranscodeLoop(md, ss)
	if err != ErrTranscoderAvail {
		t.Error("Error transcoding ", err)
	}

	// Sanity check full flow.
	n.Transcoder = NewLocalTranscoder(tmp)
	tr, err = n.sendToTranscodeLoop(md, ss)
	if err != nil {
		t.Error("Error transcoding ", err)
	}

	if len(tr.Data) != len(videoProfiles) && len(videoProfiles) != 2 {
		t.Error("Job profile count did not match broadcasters")
	}

	// 	Check transcode result
	if Over1Pct(len(tr.Data[0]), 65424) { // 144p
		t.Error("Unexpected transcode result ", len(tr.Data[0]))
	}
	if Over1Pct(len(tr.Data[1]), 81968) { // 240p
		t.Error("Unexpected transcode result ", len(tr.Data[1]))
	}

	// TODO check transcode loop expiry, storage, sig construction, etc
}

func TestTranscodeLoop_GivenNoSegmentsPastTimeout_CleansSegmentChan(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)

	transcodeLoopTimeout = 100 * time.Millisecond
	assert := assert.New(t)
	require := require.New(t)

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	segChan := getSegChan(n, md.ManifestID)
	require.NotNil(segChan)

	waitForTranscoderLoopTimeout(n, md.ManifestID)

	segChan = getSegChan(n, md.ManifestID)
	assert.Nil(segChan)
}

func TestTranscodeLoop_GivenOnePMSessionAtVideoSessionTimeout_RedeemsOneSession(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)

	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)

	sessionID := "some session ID"
	n.pmSessionsMutex.Lock()
	n.pmSessions[md.ManifestID] = make(map[string]bool)
	n.pmSessions[md.ManifestID][sessionID] = true
	n.pmSessionsMutex.Unlock()

	recipient.On("RedeemWinningTickets", []string{sessionID}[:]).Return(nil)

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)

	recipient.AssertExpectations(t)
}

func TestTranscodeLoop_GivenMultiplePMSessionAtVideoSessionTimeout_RedeemsAllSessions(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)

	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)

	sessionIDs := []string{"first session ID", "second session ID"}
	n.pmSessionsMutex.Lock()
	n.pmSessions[md.ManifestID] = make(map[string]bool)
	n.pmSessions[md.ManifestID][sessionIDs[0]] = true
	n.pmSessions[md.ManifestID][sessionIDs[1]] = true
	n.pmSessionsMutex.Unlock()

	// Need to use this because the order of the string slice used to call this function
	// later cannot be guaranteed, since they are coming from a map's keys.
	argsMatcher := mock.MatchedBy(func(IDs []string) bool {
		sort.Strings(sessionIDs)
		sort.Strings(IDs)

		return reflect.DeepEqual(sessionIDs, IDs)
	})

	recipient.On("RedeemWinningTickets", argsMatcher).Return(nil)

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)

	recipient.AssertExpectations(t)
}

func TestTranscodeLoop_GivenMultiplePMSessionsAtVideoSessionTimeout_CleansSessionIDMemoryCache(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)

	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)

	sessionIDs := []string{"first session ID", "second session ID"}
	n.pmSessionsMutex.Lock()
	n.pmSessions[md.ManifestID] = make(map[string]bool)
	n.pmSessions[md.ManifestID][sessionIDs[0]] = true
	n.pmSessions[md.ManifestID][sessionIDs[1]] = true
	n.pmSessionsMutex.Unlock()
	recipient.On("RedeemWinningTickets", mock.Anything).Return(nil)

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)

	n.pmSessionsMutex.Lock()
	assert.NotContains(t, n.pmSessions, md.ManifestID)
	n.pmSessionsMutex.Unlock()
}

func TestTranscodeLoop_GivenNoPMSessionAtVideoSessionTimeout_DoesntTryToRedeem(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)
	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)
	recipient.On("RedeemWinningTickets", mock.Anything).Return(nil)

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)

	recipient.AssertNotCalled(t, "RedeemWinningTickets", mock.Anything)
}

func TestTranscodeLoop_GivenRedeemErrorAtVideoSessionTimeout_ErrorLogIsWritten(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)
	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)

	sessionID := "some session ID"
	n.pmSessionsMutex.Lock()
	n.pmSessions[md.ManifestID] = make(map[string]bool)
	n.pmSessions[md.ManifestID][sessionID] = true
	n.pmSessionsMutex.Unlock()
	recipient.On("RedeemWinningTickets", mock.Anything).Return(fmt.Errorf("some error"))

	errorLogsBefore := glog.Stats.Error.Lines()
	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)
	errorLogsAfter := glog.Stats.Error.Lines()

	assert.Equal(t, int64(1), errorLogsAfter-errorLogsBefore)
}

func TestTranscodeLoop_GivenRedeemErrorAtVideoSessionTimeout_StillCleanspmSessionsCache(t *testing.T) {
	recipient := new(pm.MockRecipient)
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	n.Recipient = recipient
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles}
	n.Transcoder = NewLocalTranscoder(tmp)
	transcodeLoopTimeout = 100 * time.Millisecond
	require := require.New(t)

	sessionID := "some session ID"
	n.pmSessionsMutex.Lock()
	n.pmSessions[md.ManifestID] = make(map[string]bool)
	n.pmSessions[md.ManifestID][sessionID] = true
	n.pmSessionsMutex.Unlock()
	recipient.On("RedeemWinningTickets", mock.Anything).Return(fmt.Errorf("some error"))

	_, err := n.sendToTranscodeLoop(md, ss)
	require.Nil(err)
	waitForTranscoderLoopTimeout(n, md.ManifestID)

	n.pmSessionsMutex.Lock()
	assert.NotContains(t, n.pmSessions, md.ManifestID)
	n.pmSessionsMutex.Unlock()
}

func waitForTranscoderLoopTimeout(n *LivepeerNode, m ManifestID) {
	for i := 0; i < 3; i++ {
		time.Sleep(transcodeLoopTimeout * 2)
		segChan := getSegChan(n, m)
		if segChan == nil {
			return
		}
	}
}

func getSegChan(n *LivepeerNode, m ManifestID) SegmentChan {
	n.segmentMutex.Lock()
	defer n.segmentMutex.Unlock()

	return n.SegmentChans[m]
}

// XXX unclear what the tests below check
type Vint interface {
	Call(nums ...int)
}

type Vimp struct{}

func (*Vimp) Call(nums ...int) {
	fmt.Println(nums[0])
}

func TestInterface(t *testing.T) {
	var obj Vint
	obj = &Vimp{}
	obj.Call(4, 5, 6)
}

func TestSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	boolChan := make(chan bool)
	intChan := chanRoutine(ctx, boolChan)
	go insertBool(boolChan)
	go monitorChan(intChan)
	time.Sleep(time.Second)
	cancel()

	// time.Sleep(time.Second * 5)
}

func insertBool(boolChan chan bool) {
	for {
		boolChan <- true
		time.Sleep(500 * time.Millisecond)
	}
}

func chanRoutine(ctx context.Context, boolChan chan bool) chan int {
	intChan := make(chan int)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-boolChan:
				intChan <- i
			case <-ctx.Done():
				fmt.Println("Done")
				return
			}
		}
	}()
	return intChan
}

func monitorChan(intChan chan int) {
	for {
		select {
		case i := <-intChan:
			fmt.Printf("i:%v\n", i)
		}
	}
}

func TestCrypto(t *testing.T) {
	blkNumB := make([]byte, 8)
	binary.BigEndian.PutUint64(blkNumB, uint64(9994353847340985734))
	fmt.Printf("%x\n\n", blkNumB)

	newb := make([]byte, 32)
	copy(newb[24:], blkNumB[:])
	fmt.Printf("%x\n\n", newb)

	i, _ := binary.Uvarint(ethCrypto.Keccak256(newb, ethCrypto.Keccak256([]byte("abc"))))
	fmt.Printf("%x\n\n", i%1)
}
