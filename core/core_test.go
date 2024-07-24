package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

func Over1Pct(val int, cmp int) bool {
	return float32(val) > float32(cmp)*1.02 || float32(val) < float32(cmp)*0.98
}

func StubSegment() *stream.HLSSegment {
	d, _ := ioutil.ReadFile("./test.ts")
	return &stream.HLSSegment{SeqNo: 100, Name: "test.ts", Data: d[0:402696], Duration: 1}
}

func StubJobId() int64 {
	return int64(1234)
}

var videoProfiles = func() []ffmpeg.VideoProfile {
	p := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	p[0].Format = ffmpeg.FormatMPEGTS
	p[1].Format = ffmpeg.FormatMPEGTS
	return p
}()

func stubAuthToken() *net.AuthToken {
	return &net.AuthToken{Token: []byte("foo"), SessionId: "bar", Expiration: time.Now().Add(1 * time.Hour).Unix()}
}

func TestTranscode(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	seth := &eth.StubClient{}
	tmp := t.TempDir()
	n, _ := NewLivepeerNode(seth, tmp, nil)
	ffmpeg.InitFFmpeg()

	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles, AuthToken: stubAuthToken()}

	// Check nil transcoder.
	tr, err := n.sendToTranscodeLoop(context.TODO(), md, ss)
	if err != ErrTranscoderAvail {
		t.Error("Error transcoding ", err)
	}

	// Sanity check full flow.
	n.Transcoder = NewLocalTranscoder(tmp)
	tr, err = n.sendToTranscodeLoop(context.TODO(), md, ss)
	if err != nil {
		t.Error("Error transcoding ", err)
	}

	// Check that local OS is initialize auth token sessionID and not the manifestID supplied by B
	memOS := drivers.NodeStorage.(*drivers.MemoryOS)
	assert.NotNil(t, memOS.GetSession(md.AuthToken.SessionId))
	assert.Nil(t, memOS.GetSession(string(md.ManifestID)))

	if len(tr.TranscodeData.Segments) != len(videoProfiles) && len(videoProfiles) != 2 {
		t.Error("Job profile count did not match broadcasters")
	}

	// 	Check transcode result
	if Over1Pct(len(tr.TranscodeData.Segments[0].Data), 273352) { // 144p
		t.Error("Unexpected transcode result ", len(tr.TranscodeData.Segments[0].Data))
	}
	if Over1Pct(len(tr.TranscodeData.Segments[1].Data), 378068) { // 240p
		t.Error("Unexpected transcode result ", len(tr.TranscodeData.Segments[1].Data))
	}

	// TODO check transcode loop expiry, storage, sig construction, etc
}

func TestTranscodeSeg(t *testing.T) {
	tmp := t.TempDir()

	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	n, _ := NewLivepeerNode(nil, tmp, nil)
	n.Transcoder = stubTranscoderWithProfiles(profiles)

	conf := transcodeConfig{LocalOS: (drivers.NewMemoryDriver(nil)).NewSession("")}
	md := &SegTranscodingMetadata{Profiles: profiles, AuthToken: stubAuthToken()}
	seg := StubSegment()

	assert := assert.New(t)
	require := require.New(t)

	// Test offchain mode
	require.Nil(n.Eth) // sanity check the offchain precondition of a nil eth
	res := n.transcodeSeg(context.TODO(), conf, seg, md)
	assert.Nil(res.Err)
	assert.Nil(res.Sig)
	// sanity check results
	resBytes, _ := n.Transcoder.Transcode(context.TODO(), md)
	for i, trData := range res.TranscodeData.Segments {
		assert.Equal(resBytes.Segments[i].Data, trData.Data)
	}

	// Test onchain mode
	n.Eth = &eth.StubClient{}
	res = n.transcodeSeg(context.TODO(), conf, seg, md)
	assert.Nil(res.Err)
	assert.NotNil(res.Sig)
	// check sig
	resHashes := make([][]byte, len(profiles))
	for i, v := range resBytes.Segments {
		resHashes[i] = ethCrypto.Keccak256(v.Data)
	}
	resHash := ethCrypto.Keccak256(resHashes...)
	assert.Equal(resHash, res.Sig)
}

func TestTranscodeLoop_GivenNoSegmentsPastTimeout_CleansSegmentChan(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	seth := &eth.StubClient{}
	tmp := t.TempDir()
	n, _ := NewLivepeerNode(seth, tmp, nil)
	ffmpeg.InitFFmpeg()
	ss := StubSegment()
	md := &SegTranscodingMetadata{Profiles: videoProfiles, AuthToken: stubAuthToken()}
	n.Transcoder = NewLocalTranscoder(tmp)

	transcodeLoopTimeout = 100 * time.Millisecond
	assert := assert.New(t)
	require := require.New(t)

	mid := ManifestID(md.AuthToken.SessionId)

	_, err := n.sendToTranscodeLoop(context.TODO(), md, ss)
	require.Nil(err)
	segChan := getSegChan(n, mid)
	require.NotNil(segChan)

	waitForTranscoderLoopTimeout(n, mid)

	segChan = getSegChan(n, mid)
	assert.Nil(segChan)
}

func TestTranscodeLoop_CleanupForBroadcasterEndTranscodingSession(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	oldTranscodeLoopTimeout := transcodeLoopTimeout
	defer func() { transcodeLoopTimeout = oldTranscodeLoopTimeout }()
	transcodeLoopTimeout = 100 * time.Millisecond

	tmp := t.TempDir()

	ffmpeg.InitFFmpeg()
	n, _ := NewLivepeerNode(&eth.StubClient{}, tmp, nil)
	n.Transcoder = NewLocalTranscoder(tmp)

	md := &SegTranscodingMetadata{Profiles: videoProfiles, AuthToken: stubAuthToken()}
	mid := ManifestID(md.AuthToken.SessionId)

	ss := StubSegment()
	_, err := n.sendToTranscodeLoop(context.TODO(), md, ss)
	require.Nil(err)
	require.NotNil(getSegChan(n, mid))

	startRoutines := runtime.NumGoroutine()

	n.endTranscodingSession(md.AuthToken.SessionId, context.TODO())
	waitForTranscoderLoopTimeout(n, mid)

	endRoutines := runtime.NumGoroutine()

	assert.Equal(endRoutines, startRoutines-1)
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
