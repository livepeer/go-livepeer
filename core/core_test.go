package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math/big"
	"testing"
	"time"

	"strings"

	"io/ioutil"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/livepeer/golp/net"
	"github.com/livepeer/lpms/stream"
)

// Should Create Node
func TestNewLivepeerNode(t *testing.T) {
	// n := NewLivepeerNode()
	// if n == nil {
	// 	t.Errorf("Cannot set up new livepeer node")
	// }
}

type StubVideoNetwork struct {
	T            *testing.T
	broadcasters map[string]*StubBroadcaster
	tResult      map[string]string
	strmID       string
	nodeID       string
}

func (n *StubVideoNetwork) GetBroadcaster(strmID string) (net.Broadcaster, error) {
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
func (n *StubVideoNetwork) GetSubscriber(strmID string) (net.Subscriber, error) {
	return &StubSubscriber{}, nil
}
func (n *StubVideoNetwork) Connect(nodeID, nodeAddr string) error { return nil }
func (n *StubVideoNetwork) SetupProtocol() error                  { return nil }
func (n *StubVideoNetwork) SendTranscodResult(nid string, sid string, tr map[string]string) error {
	n.nodeID = nid
	n.strmID = sid
	n.tResult = tr
	return nil
}

type StubBroadcaster struct {
	T      *testing.T
	StrmID string
	SeqNo  uint64
	Data   []byte
}

func (n *StubBroadcaster) Broadcast(seqNo uint64, data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	var seg stream.HLSSegment
	dec.Decode(&seg)

	if n.SeqNo == 0 {
		n.SeqNo = seg.SeqNo
	} else {
		n.T.Errorf("Already assigned")
	}

	if n.Data == nil {
		n.Data = seg.Data
	} else {
		n.T.Errorf("Already assigned")
	}
	return nil
}
func (n *StubBroadcaster) Finish() error { return nil }

type StubSubscriber struct{}

func (s *StubSubscriber) Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error {
	d, _ := ioutil.ReadFile("./test.ts")
	newSeg := stream.HLSSegment{SeqNo: 100, Name: "test name", Data: d, Duration: 1}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(newSeg)
	gotData(100, buf.Bytes(), false)
	return nil
}
func (s *StubSubscriber) Unsubscribe() error { return nil }

func TestTranscode(t *testing.T) {
	//Set up the node
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	n, _ := NewLivepeerNode(15000, priv, pub, nil)
	n.VideoNetwork = &StubVideoNetwork{T: t}

	//Call transcode
	ids, err := n.Transcode(net.TranscodeConfig{StrmID: "strmID", Profiles: []net.VideoProfile{net.P_144P_30FPS_16_9, net.P_240P_30FPS_16_9}})

	if err != nil {
		t.Errorf("Error transcoding: %v", err)
	}

	if !strings.HasSuffix(ids[0].String(), "P144P30FPS169") {
		t.Errorf("Bad id0: %v", ids[0])
	}

	if !strings.HasSuffix(ids[1].String(), "P240P30FPS169") {
		t.Errorf("Bad id1: %v", ids[1])
	}

	// b1ID := strings.Replace(ids[0], "P_144P_30")
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
	for time.Since(start) < time.Second*3 {
		if b1.SeqNo == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	if b1.SeqNo != 100 || b2.SeqNo != 100 {
		t.Errorf("Wrong SeqNo assigned to broadcaster: %v", b1.SeqNo)
	}

	if len(b1.Data) != 277300 || len(b2.Data) != 378068 {
		t.Errorf("Wrong data assigned to broadcaster: %v, %v", len(b1.Data), len(b2.Data))
	}
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
	// intChan := make(chan int)
	// go forroutine(ctx, intChan)
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

func forroutine(ctx context.Context, intChan chan int) {
	go func() {
		for i := 0; ; i++ {
			intChan <- i
			time.Sleep(500 * time.Millisecond)
		}
	}()
	select {
	case <-ctx.Done():
		fmt.Println("Done")
	}
}

func monitorChan(intChan chan int) {
	for {
		select {
		case i := <-intChan:
			fmt.Printf("i:%v\n", i)
		}
	}
}

type StubEth struct {
	strmID   string
	tOpts    [32]byte
	maxPrice *big.Int
}

func (e *StubEth) Backend() *ethclient.Client { return nil }
func (e *StubEth) Account() accounts.Account  { return accounts.Account{} }
func (e *StubEth) SubscribeToJobEvent(callback func(types.Log) error) (ethereum.Subscription, chan types.Log, error) {
	return nil, nil, nil
}
func (e *StubEth) WatchEvent(logsCh <-chan types.Log) (types.Log, error) { return types.Log{}, nil }
func (e *StubEth) RoundInfo() (*big.Int, *big.Int, *big.Int, *big.Int, error) {
	return nil, nil, nil, nil, nil
}
func (e *StubEth) InitializeRound() (*types.Transaction, error) { return nil, nil }
func (e *StubEth) CurrentRoundInitialized() (bool, error)       { return false, nil }
func (e *StubEth) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubEth) IsActiveTranscoder() (bool, error)  { return false, nil }
func (e *StubEth) TranscoderStake() (*big.Int, error) { return nil, nil }
func (e *StubEth) Bond(amount *big.Int, toAddr common.Address) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubEth) ValidRewardTimeWindow() (bool, error) { return false, nil }
func (e *StubEth) Reward() (*types.Transaction, error)  { return nil, nil }
func (e *StubEth) Job(streamId string, transcodingOptions [32]byte, maxPricePerSegment *big.Int) (*types.Transaction, error) {
	e.strmID = streamId
	e.tOpts = transcodingOptions
	e.maxPrice = maxPricePerSegment
	return nil, nil
}
func (e *StubEth) SignSegmentHash(passphrase string, hash []byte) ([]byte, error) { return nil, nil }
func (e *StubEth) ClaimWork(jobId *big.Int, startSegmentSequenceNumber *big.Int, endSegmentSequenceNumber *big.Int, transcodeClaimsRoot [32]byte) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubEth) Verify(jobId *big.Int, segmentSequenceNumber *big.Int, dataHash [32]byte, transcodedDataHash [32]byte, broadcasterSig []byte, proof []byte) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubEth) Transfer(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return nil, nil
}
func (e *StubEth) TokenBalance() (*big.Int, error)   { return nil, nil }
func (e *StubEth) WaitUntilNextRound(*big.Int) error { return nil }

func TestCreateTranscodeJob(t *testing.T) {
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	seth := &StubEth{}
	n, _ := NewLivepeerNode(15000, priv, pub, seth)
	strmID, _ := MakeStreamID(n.Identity, RandomVideoID(), "")
	err := n.CreateTranscodeJob(strmID, []net.VideoProfile{net.P_720P_60FPS_16_9}, 999999999999)
	if err == nil {
		t.Errorf("Expecting error since no broadcast stream in streamDB")
	}

	n.StreamDB.AddNewHLSBuffer(strmID)
	err = n.CreateTranscodeJob(strmID, []net.VideoProfile{net.P_720P_60FPS_16_9}, 999999999999)
	if err != nil {
		t.Errorf("Error creating transcoding job")
	}

	if seth.strmID != strmID.String() {
		t.Errorf("Expecting strmID to be: %v", strmID)
	}

	if strings.Trim(string(seth.tOpts[:]), "\x00") != net.P_720P_60FPS_16_9.Name {
		t.Errorf("Expecting transcode options to be %v, but got %v", net.P_720P_60FPS_16_9.Name, string(seth.tOpts[:]))
	}

	if big.NewInt(999999999999).Cmp(seth.maxPrice) != 0 {
		t.Errorf("Expecting price to be 999999999999, got but %v", seth.maxPrice)
	}

}

func TestNotifyBroadcaster(t *testing.T) {
	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	seth := &StubEth{}
	n, _ := NewLivepeerNode(15000, priv, pub, seth)
	sn := &StubVideoNetwork{}
	n.VideoNetwork = sn

	err := n.NotifyBroadcaster(n.Identity, "strmid", map[StreamID]net.VideoProfile{"strmid1": net.P_240P_30FPS_16_9})
	if err != nil {
		t.Errorf("Error notifying broadcaster: %v", err)
	}

	if sn.nodeID != string(n.Identity) {
		t.Errorf("Expecting %v, got %v", n.Identity, sn.nodeID)
	}

	if sn.strmID != "strmid" {
		t.Errorf("Expecting strmid, got %v", sn.strmID)
	}

	if sn.tResult["strmid1"] != net.P_240P_30FPS_16_9.Name {
		t.Errorf("Expecting %v, got %v", net.P_240P_30FPS_16_9.Name, sn.tResult["strmid1"])
	}
}
