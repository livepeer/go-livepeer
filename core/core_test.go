package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/eth"
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

var videoProfiles = func() []ffmpeg.VideoProfile {
	p := []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	p[0].Format = ffmpeg.FormatMPEGTS
	p[1].Format = ffmpeg.FormatMPEGTS
	return p
}()

func TestTranscode(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	n, _ := NewLivepeerNode(seth, tmp, nil)
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

	if len(tr.TranscodeData.Segments) != len(videoProfiles) && len(videoProfiles) != 2 {
		t.Error("Job profile count did not match broadcasters")
	}

	// 	Check transcode result
	if Over1Pct(len(tr.TranscodeData.Segments[0].Data), 74636) { // 144p
		t.Error("Unexpected transcode result ", len(tr.TranscodeData.Segments[0].Data))
	}
	if Over1Pct(len(tr.TranscodeData.Segments[1].Data), 100204) { // 240p
		t.Error("Unexpected transcode result ", len(tr.TranscodeData.Segments[1].Data))
	}

	// TODO check transcode loop expiry, storage, sig construction, etc
}

func TestTranscodeSeg(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	profiles := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	n, _ := NewLivepeerNode(nil, tmp, nil)
	n.Transcoder = stubTranscoderWithProfiles(profiles)

	conf := transcodeConfig{LocalOS: (drivers.NewMemoryDriver(nil)).NewSession("")}
	md := &SegTranscodingMetadata{Profiles: profiles}
	seg := StubSegment()

	assert := assert.New(t)
	require := require.New(t)

	// Test offchain mode
	require.Nil(n.Eth) // sanity check the offchain precondition of a nil eth
	res := n.transcodeSeg(conf, seg, md)
	assert.Nil(res.Err)
	assert.Nil(res.Sig)
	// sanity check results
	resBytes, _ := n.Transcoder.Transcode(md)
	for i, trData := range res.TranscodeData.Segments {
		assert.Equal(resBytes.Segments[i].Data, trData.Data)
	}

	// Test onchain mode
	n.Eth = &eth.StubClient{}
	res = n.transcodeSeg(conf, seg, md)
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
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)
	n, _ := NewLivepeerNode(seth, tmp, nil)
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

func TestEstimateFee(t *testing.T) {
	assert := assert.New(t)

	// Test nil priceInfo
	fee, err := EstimateFee(&stream.HLSSegment{}, []ffmpeg.VideoProfile{}, nil)
	assert.Nil(err)
	assert.Nil(fee)

	// Test first profile is invalid
	profiles := []ffmpeg.VideoProfile{ffmpeg.VideoProfile{Resolution: "foo"}}
	_, err = EstimateFee(&stream.HLSSegment{}, profiles, big.NewRat(1, 1))
	assert.Error(err)

	// Test non-first profile is invalid
	profiles = []ffmpeg.VideoProfile{
		ffmpeg.P144p30fps16x9,
		ffmpeg.VideoProfile{Resolution: "foo"},
	}
	_, err = EstimateFee(&stream.HLSSegment{}, profiles, big.NewRat(1, 1))
	assert.Error(err)

	// Test no profiles
	fee, err = EstimateFee(&stream.HLSSegment{Duration: 2.0}, []ffmpeg.VideoProfile{}, big.NewRat(1, 1))
	assert.Nil(err)
	assert.Zero(fee.Cmp(big.NewRat(0, 1)))

	// Test estimation with 1 profile
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9}
	priceInfo := big.NewRat(3, 1)
	// pixels = 256 * 144 * 30 * 2
	expFee := new(big.Rat).SetInt64(2211840)
	expFee.Mul(expFee, new(big.Rat).SetFloat64(pixelEstimateMultiplier))
	expFee.Mul(expFee, priceInfo)
	fee, err = EstimateFee(&stream.HLSSegment{Duration: 2.0}, profiles, priceInfo)
	assert.Nil(err)
	assert.Zero(fee.Cmp(expFee))

	// Test estimation with 2 profiles
	profiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}
	// pixels = (256 * 144 * 30 * 2) + (426 * 240 * 30 * 2)
	expFee = new(big.Rat).SetInt64(8346240)
	expFee.Mul(expFee, new(big.Rat).SetFloat64(pixelEstimateMultiplier))
	expFee.Mul(expFee, priceInfo)
	fee, err = EstimateFee(&stream.HLSSegment{Duration: 2.0}, profiles, priceInfo)
	assert.Nil(err)
	assert.Zero(fee.Cmp(expFee))

	// Test estimation with non-integer duration
	// pixels = (256 * 144 * 30 * 3) + (426 * 240 * 30 * 3)
	expFee = new(big.Rat).SetInt64(12519360)
	expFee.Mul(expFee, new(big.Rat).SetFloat64(pixelEstimateMultiplier))
	expFee.Mul(expFee, priceInfo)
	// Calculations should take ceiling of duration i.e. 2.2 -> 3
	fee, err = EstimateFee(&stream.HLSSegment{Duration: 2.2}, profiles, priceInfo)
	assert.Nil(err)
	assert.Zero(fee.Cmp(expFee))

	// Test estimation with fps pass-through
	// pixels = (256 * 144 * 120 * 3) + (426 * 240 * 30 * 3)
	profiles[0].Framerate = 0
	expFee = new(big.Rat).SetInt64(22472640)
	expFee.Mul(expFee, new(big.Rat).SetFloat64(pixelEstimateMultiplier))
	expFee.Mul(expFee, priceInfo)
	fee, err = EstimateFee(&stream.HLSSegment{Duration: 3.0}, profiles, priceInfo)
	assert.Nil(err)
	assert.Zero(fee.Cmp(expFee))
	assert.Equal(uint(0), profiles[0].Framerate, "Profile framerate was reset")
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
