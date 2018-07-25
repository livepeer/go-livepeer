package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	//"strings"
	"math/big"
	"testing"
	"time"

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	//"github.com/livepeer/lpms/transcoder"
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
	broadcasters map[string]*StubBroadcaster
	subscribers  map[string]*StubSubscriber
	tResult      map[string]string
	strmID       string
	nodeID       string
	mplMap       map[string]*m3u8.MasterPlaylist
	connectInfo  []StubConnInfo
}

func (n *StubVideoNetwork) String() string { return "" }
func (n *StubVideoNetwork) GetNodeID() string {
	return "122011e494a06b20bf7a80f40e80d538675cc0b168c21912d33e0179617d5d4fe4e0"
}

func (n *StubVideoNetwork) GetBroadcaster(strmID string) (stream.Broadcaster, error) {
	if n.broadcasters == nil {
		n.broadcasters = make(map[string]*StubBroadcaster)
	}
	b, ok := n.broadcasters[strmID]
	if !ok {
		b = &StubBroadcaster{T: n.T, StrmID: strmID}
		n.broadcasters[strmID] = b
	}
	return b, nil
}
func (n *StubVideoNetwork) GetSubscriber(strmID string) (stream.Subscriber, error) {
	if s, ok := n.subscribers[strmID]; ok {
		return s, nil
	}
	return nil, ErrNotFound
	// return &StubSubscriber{T: n.T}, nil
}
func (n *StubVideoNetwork) Connect(nodeID string, nodeAddr []string) error {
	if n.connectInfo == nil {
		n.connectInfo = make([]StubConnInfo, 0)
	}
	n.connectInfo = append(n.connectInfo, StubConnInfo{NodeID: nodeID, NodeAddr: nodeAddr})
	return nil
}
func (n *StubVideoNetwork) SetupProtocol() error { return nil }
func (n *StubVideoNetwork) SendTranscodeResponse(nid string, sid string, tr map[string]string) error {
	n.nodeID = nid
	n.strmID = sid
	n.tResult = tr
	return nil
}
func (n *StubVideoNetwork) ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string)) {
}
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

func (n *StubVideoNetwork) TranscodeSub(ctx context.Context, strmID string, gotData func(seqNo uint64, data []byte, eof bool)) error {
	sub, _ := n.GetSubscriber(strmID)
	return sub.Subscribe(ctx, gotData)
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
	n, _ := NewLivepeerNode(nil, stubnet, "12209433a695c8bf34ef6a40863cfe7ed64266d876176aee13732293b63ba1637fd2", ".tmp", nil)
	job := StubJob(n)
	stubnet.subscribers[job.StreamId] = &StubSubscriber{}
	ffmpeg.InitFFmpeg()
	defer ffmpeg.DeinitFFmpeg()

	//Call transcode
	ss := StubSegment()
	err := n.TranscodeSegment(job, ss)
	if err != nil {
		t.Error("Error transcoding ", err)
	}

	/*tProfiles := make([]ffmpeg.VideoProfile, len(p), len(p))
	for i, vp := range p {
		tProfiles[i] = ffmpeg.VideoProfileLookup[vp.Name]
	}
	tr := transcoder.NewFFMpegSegmentTranscoder(tProfiles, n.WorkDir)
	ids, err := n.TranscodeAndBroadcast(net.TranscodeConfig{StrmID: strmID.String(), Profiles: p}, &StubClaimManager{}, tr)
	if err != nil {
		t.Errorf("Error transcoding: %v", err)
	}

	if !strings.HasSuffix(ids[0].String(), "P144p30fps16x9") {
		t.Errorf("Bad id0: %v", ids[0])
	}

	if !strings.HasSuffix(ids[1].String(), "P240p30fps16x9") {
		t.Errorf("Bad id1: %v", ids[1])
	}

	b1tmp, err := n.VideoNetwork.GetBroadcaster(ids[0].String())
	if err != nil {
		t.Errorf("Error getting broadcaster: %v", err)
	}
	b2tmp, err := n.VideoNetwork.GetBroadcaster(ids[1].String())
	if err != nil {
		t.Errorf("Error getting broadcaster: %v", err)
	}
	b1, ok := b1tmp.(*StubBroadcaster)
	if !ok {
		t.Errorf("Error converting broadcaster")
	}
	b2, ok := b2tmp.(*StubBroadcaster)
	if !ok {
		t.Errorf("Error converting broadcaster")
	}
	start := time.Now()
	for time.Since(start) < time.Second*5 {
		if b1.SeqNo == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if b1.SeqNo != 100 || b2.SeqNo != 100 {
		t.Errorf("Wrong SeqNo assigned to broadcaster: %v", b1.SeqNo)
	}

	if Over1Pct(len(b1.Data), 65424) || Over1Pct(len(b2.Data), 81968) {
		t.Errorf("Wrong data assigned to broadcaster: %v, %v", len(b1.Data), len(b2.Data))
	}
	*/
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

// TODO add tests for resulting streamIDs from TranscodeSegmentLoop

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
