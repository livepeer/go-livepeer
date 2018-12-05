package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/common"
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

var videoProfiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}

func TestTranscode(t *testing.T) {
	//Set up the node
	drivers.NodeStorage = drivers.NewMemoryDriver("")
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	seth := &eth.StubClient{}
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, tmp, db)
	defer os.RemoveAll(tmp)
	ffmpeg.InitFFmpeg()

	ss := StubSegment()
	md := &SegmentMetadata{Profiles: videoProfiles}

	// Check nil transcoder.
	tr, err := n.TranscodeSegment(md, ss)
	if err != ErrTranscoderAvail {
		t.Error("Error transcoding ", err)
	}

	// Sanity check full flow.
	n.Transcoder = NewLocalTranscoder(tmp)
	tr, err = n.TranscodeSegment(md, ss)
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
