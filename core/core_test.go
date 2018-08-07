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

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

func Over1Pct(val int, cmp int) bool {
	return float32(val) > float32(cmp)*1.01 || float32(val) < float32(cmp)*0.99
}

type StubConnInfo struct {
	NodeID   string
	NodeAddr []string
}
type StubVideoNetwork struct {
	T            *testing.T
	broadcasters map[StreamID]stream.Broadcaster
	subscribers  map[string]*StubSubscriber
	tResult      map[string]string
	strmID       string
	nodeID       string
	mplMap       map[string]*m3u8.MasterPlaylist
	connectInfo  []StubConnInfo
}

func (n *StubVideoNetwork) String() string { return "" }

func (n *StubVideoNetwork) GetMasterPlaylist(nodeID string, strmID string) (chan *m3u8.MasterPlaylist, error) {
	mplc := make(chan *m3u8.MasterPlaylist)
	mpl, ok := n.mplMap[strmID]
	if !ok {
		mpl = m3u8.NewMasterPlaylist()
		pl, _ := m3u8.NewMediaPlaylist(100, 100)
		mpl.Append("stub.m3u8", pl, m3u8.VariantParams{Bandwidth: 100})
	}
	// glog.Infof("StubNetwork GetMasterPlaylist. mpl: %v", mpl)

	go func() {
		mplc <- mpl
		close(mplc)
	}()

	return mplc, nil
}
func (n *StubVideoNetwork) UpdateMasterPlaylist(strmID string, mpl *m3u8.MasterPlaylist) error {
	if n.mplMap == nil {
		n.mplMap = make(map[string]*m3u8.MasterPlaylist)
	}
	n.mplMap[strmID] = mpl
	return nil
}

func (n *StubVideoNetwork) GetNodeStatus(nodeID string) (chan *net.NodeStatus, error) {
	return nil, nil
}

type StubBroadcaster struct {
	T         *testing.T
	StrmID    string
	SeqNo     uint64
	Data      []byte
	FinishMsg bool
}

func (n *StubBroadcaster) IsLive() bool   { return true }
func (n *StubBroadcaster) String() string { return "" }
func (n *StubBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	ss, err := BytesToSignedSegment(data)
	if err != nil {
		n.T.Errorf("Error Converting bytes to SignedSegment: %v", err)
	}

	if n.SeqNo == 0 {
		n.SeqNo = ss.Seg.SeqNo
	} else {
		n.T.Errorf("Already assigned")
	}

	if n.Data == nil {
		n.Data = ss.Seg.Data
	} else {
		n.T.Errorf("Already assigned")
	}
	return nil
}
func (n *StubBroadcaster) Finish() error {
	n.FinishMsg = true
	return nil
}

func StubSegment() *SignedSegment {
	d, _ := ioutil.ReadFile("./test.ts")
	return &SignedSegment{Seg: stream.HLSSegment{SeqNo: 100, Name: "test.ts", Data: d[0:402696], Duration: 1}, Sig: []byte("test sig")}
}

type StubSubscriber struct {
	T *testing.T
}

func (s *StubSubscriber) IsLive() bool   { return true }
func (s *StubSubscriber) String() string { return "" }
func (s *StubSubscriber) Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error {
	newSeg := StubSegment()
	b, err := SignedSegmentToBytes(*newSeg)
	if err != nil {
		s.T.Errorf("Error Converting SignedSegment to Bytes: %v", err)
	}

	// glog.Infof("Returning seg 100: %v", len(b))
	gotData(100, b, false)
	return nil
}
func (s *StubSubscriber) Unsubscribe() error { return nil }

func StubJob(n *LivepeerNode) *lpTypes.Job {
	streamId, _ := MakeStreamID(n.Identity, RandomVideoID(), ffmpeg.P720p30fps4x3.Name)
	return &lpTypes.Job{
		JobId:              big.NewInt(0),
		StreamId:           string(streamId),
		BroadcasterAddress: ethcommon.Address{},
		TranscoderAddress:  ethcommon.Address{},
		CreationBlock:      big.NewInt(0),
		EndBlock:           big.NewInt(10),
		MaxPricePerSegment: big.NewInt(1),
		TotalClaims:        big.NewInt(0),
		Profiles:           []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9},
	}
}

func TestTranscode(t *testing.T) {
	//Set up the node
	stubnet := &StubVideoNetwork{T: t, subscribers: make(map[string]*StubSubscriber)}
	seth := &eth.StubClient{}
	db, _ := common.InitDB("file:TestTranscode?mode=memory&cache=shared")
	defer db.Close()
	tmp, _ := ioutil.TempDir("", "")
	n, _ := NewLivepeerNode(seth, stubnet, "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2", tmp, db)
	defer os.RemoveAll(tmp)
	job := StubJob(n)
	ffmpeg.InitFFmpeg()
	defer ffmpeg.DeinitFFmpeg()

	// Sanity check full flow.
	ss := StubSegment()
	err := n.TranscodeSegment(job, ss)
	if err != nil {
		t.Error("Error transcoding ", err)
	}

	if len(stubnet.broadcasters) != len(job.Profiles) && len(job.Profiles) != 2 {
		t.Error("Job profile count did not match broadcasters")
	}

	// Check transcode result
	has_144p, has_240p := false, false
	for k, v := range stubnet.broadcasters {
		b, ok := v.(*StubBroadcaster)
		if !ok {
			t.Error("Error converting broadcaster")
		}
		if b.SeqNo != 100 {
			t.Error("Wrong SeqNo assigned to broadcaser ", b.SeqNo)
		}
		r := k.GetRendition()
		if r == "P144p30fps16x9" {
			if Over1Pct(len(b.Data), 65424) {
				t.Errorf("Wrong data assigned to broadcaster: %v", len(b.Data))
			} else {
				has_144p = true
			}
		} else if r == "P240p30fps16x9" {
			if Over1Pct(len(b.Data), 81968) {
				t.Errorf("Wrong data assigned to broadcaster: %v", len(b.Data))
			} else {
				has_240p = true
			}
		}
	}
	if !has_144p || !has_240p {
		t.Error("Missing some expected tests")
	}

	// check duplicate sequence in DB
	err = n.TranscodeSegment(job, ss)
	if err.Error() != "DuplicateSequence" {
		t.Error("Unexpected error when checking duplicate seqs ", err)
	}

	// Check segment too long
	d, _ := ioutil.ReadFile("test.ts")
	ssd := ss.Seg.Data
	ss.Seg.Data = d
	ss.Seg.SeqNo += 1
	err = n.TranscodeSegment(job, ss)
	if err.Error() != "MediaStats Failure" {
		t.Error("Unexpected error when checking mediastats ", err)
	}
	ss.Seg.Data = ssd

	// Check insufficient deposit
	job.JobId = big.NewInt(10) // force a new job with a new price
	job.MaxPricePerSegment = big.NewInt(1000)
	err = n.TranscodeSegment(job, ss)
	if err.Error() != "Insufficient deposit" {
		t.Error("Unexpected error when checking deposit ", err)
	}

	// TODO check transcode loop expiry, claim manager submission, etc

}

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

func TestCreateTranscodeJob(t *testing.T) {
	nw := &StubVideoNetwork{}
	seth := &eth.StubClient{JobsMap: make(map[string]*lpTypes.Job)}
	n, _ := NewLivepeerNode(seth, nw, "", "./tmp", nil)
	j := StubJob(n)
	seth.JobsMap[j.StreamId] = j

	cjt := func(n *LivepeerNode) (*lpTypes.Job, error) {
		return n.CreateTranscodeJob(StreamID(j.StreamId), j.Profiles, j.MaxPricePerSegment)
	}
	seth.TranscoderAddress = ethcommon.BytesToAddress([]byte("Job Transcoder Addr"))

	// test success
	if _, err := cjt(n); err != nil {
		t.Error("Error creating transcode job ", err)
	}
	if j.TranscoderAddress != seth.TranscoderAddress {
		t.Error("Did not have expected transcoder assigned ", j.TranscoderAddress)
	}

	// test missing eth client
	n1, _ := NewLivepeerNode(nil, nw, "", "./tmp", nil)
	if _, err := cjt(n1); err != ErrNotFound {
		t.Error("Did not receive expected error; got ", err)
	}

	// test various error conditions from ethclient

	seth.LatestBlockError = fmt.Errorf("LatestBlockError")
	if _, err := cjt(n); err != seth.LatestBlockError {
		t.Error("Did not receive expected error; got ", err)
	}
	seth.LatestBlockError = nil

	seth.JobError = fmt.Errorf("JobError")
	if _, err := cjt(n); err != seth.JobError {
		t.Error("Did not receive expeced error; got ", err)
	}
	seth.JobError = nil

	seth.WatchJobError = fmt.Errorf("WatchJobError")
	if _, err := cjt(n); err != seth.WatchJobError {
		t.Error("Did not receive expected error; got ", err)
	}
	seth.WatchJobError = nil
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
