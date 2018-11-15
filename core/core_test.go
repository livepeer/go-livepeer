package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

func Over1Pct(val int, cmp int) bool {
	return float32(val) > float32(cmp)*1.01 || float32(val) < float32(cmp)*0.99
}

func StubSegment() *SignedSegment {
	d, _ := ioutil.ReadFile("./test.ts")
	return &SignedSegment{Seg: stream.HLSSegment{SeqNo: 100, Name: "test.ts", Data: d[0:402696], Duration: 1}, Sig: []byte("test sig")}
}

func StubJobId() int64 {
	return int64(1234)
}

var videoProfiles = []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9}

// func TestTranscode(t *testing.T) {
// 	//Set up the node
// 	drivers.NodeStorage = drivers.NewMemoryDriver("")
// 	seth := &eth.StubClient{}
// 	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
// 	defer db.Close()
// 	tmp, _ := ioutil.TempDir("", "")
// 	n, _ := NewLivepeerNode(seth, tmp, db)
// 	defer os.RemoveAll(tmp)
// 	jobId := StubJobId()
// 	ffmpeg.InitFFmpeg()

// 	// Sanity check full flow.
// 	ss := StubSegment()
// 	tr, err := n.TranscodeSegment(jobId, ss)
// 	if err != nil {
// 		t.Error("Error transcoding ", err)
// 	}

// 	if len(tr.Data) != len(videoProfiles) && len(videoProfiles) != 2 {
// 		t.Error("Job profile count did not match broadcasters")
// 	}

// 	// Check transcode result
// 	// XXX fix - we don't have this data here now
// 	/*
// 		has_144p, has_240p := false, false
// 		for _, v := range tr.Data {
// 			rgx, _ := regexp.Compile("[[:alnum:]]+")
// 			sid := StreamID(rgx.FindString(v)) // trim off the training "_100.ts"
// 			b := n.VideoSource.GetHLSSegment(sid, v)
// 			if b == nil {
// 				t.Error("Error converting broadcaster ", sid.GetRendition())
// 			}
// 			if b.SeqNo != 100 {
// 				t.Error("Wrong SeqNo assigned to broadcaser ", b.SeqNo)
// 			}
// 			r := sid.GetRendition()
// 			if r == "P144p30fps16x9" {
// 				if Over1Pct(len(b.Data), 65424) {
// 					t.Errorf("Wrong data assigned to broadcaster: %v", len(b.Data))
// 				} else {
// 					has_144p = true
// 				}
// 			} else if r == "P240p30fps16x9" {
// 				if Over1Pct(len(b.Data), 81968) {
// 					t.Errorf("Wrong data assigned to broadcaster: %v", len(b.Data))
// 				} else {
// 					has_240p = true
// 				}
// 			}
// 		}
// 		if !has_144p || !has_240p {
// 			t.Error("Missing some expected tests")
// 		}
// 	*/

// 	// check duplicate sequence in DB
// 	_, err = n.TranscodeSegment(jobId, ss)
// 	if err.Error() != "DuplicateSequence" {
// 		t.Error("Unexpected error when checking duplicate seqs ", err)
// 	}

// 	// Check segment too long
// 	d, _ := ioutil.ReadFile("test.ts")
// 	ssd := ss.Seg.Data
// 	ss.Seg.Data = d
// 	ss.Seg.SeqNo += 1
// 	_, err = n.TranscodeSegment(jobId, ss)
// 	if err.Error() != "MediaStats Failure" {
// 		t.Error("Unexpected error when checking mediastats ", err)
// 	}
// 	ss.Seg.Data = ssd

// 	// TODO check transcode loop expiry, claim manager submission, etc
// }

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
