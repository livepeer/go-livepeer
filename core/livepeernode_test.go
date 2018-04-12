package core

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

type StubClaimManager struct {
	verifyCalled         bool
	distributeFeesCalled bool
}

func (cm *StubClaimManager) BroadcasterAddr() common.Address { return common.Address{} }
func (cm *StubClaimManager) AddReceipt(seqNo int64, data []byte, bSig []byte, tData map[ffmpeg.VideoProfile][]byte, tStart time.Time, tEnd time.Time) error {
	return nil
}
func (cm *StubClaimManager) SufficientBroadcasterDeposit() (bool, error) { return true, nil }
func (cm *StubClaimManager) ClaimVerifyAndDistributeFees() error         { return nil }
func (cm *StubClaimManager) DidFirstClaim() bool                         { return false }
func (cm *StubClaimManager) CanClaim() (bool, error)                     { return true, nil }

type StubTranscoder struct {
	Profiles      []ffmpeg.VideoProfile
	InputData     [][]byte
	FailTranscode bool
}

func (t *StubTranscoder) Transcode(fname string) ([][]byte, error) {
	d, err := ioutil.ReadFile(fname)
	if err != nil || t.FailTranscode {
		return nil, ErrTranscode
	}

	t.InputData = append(t.InputData, d)

	result := make([][]byte, 0)
	for _, p := range t.Profiles {
		result = append(result, []byte(fmt.Sprintf("Transcoded_%v", p.Name)))
	}
	return result, nil
}

func TestTranscodeAndBroadcast(t *testing.T) {
	nid := NodeID("12201c23641663bf06187a8c154a6c97266d138cb8379c1bc0828122dcc51c83698d")
	strmID, _ := MakeStreamID(nid, RandomVideoID(), ffmpeg.P720p30fps4x3.Name)
	jid := big.NewInt(0)
	ffmpeg.InitFFmpeg()
	defer ffmpeg.DeinitFFmpeg()
	p := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	config := net.TranscodeConfig{StrmID: strmID.String(), Profiles: p, PerformOnchainClaim: false, JobID: jid}

	stubnet := &StubVideoNetwork{subscribers: make(map[string]*StubSubscriber)}
	stubnet.subscribers[strmID.String()] = &StubSubscriber{}
	n, err := NewLivepeerNode(&eth.StubClient{}, stubnet, nid, ".", nil) // TODO fix empty work dir
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	tr := &StubTranscoder{InputData: make([][]byte, 0), Profiles: p}
	ids, err := n.TranscodeAndBroadcast(config, &StubClaimManager{}, tr)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("Expecting 2 profiles, got %v", ids)
	}

	start := time.Now()
	for time.Since(start) < time.Second {
		if len(tr.InputData) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	//Should have transcoded the segments into 2 different profiles (right now StubSubscriber emits 1 segment)
	if len(tr.InputData) != 1 {
		t.Errorf("Expecting 1 segment to be transcoded, got %v", tr.InputData)
	}

	// Should have broadcasted the transcoded segments into new streams
	if len(stubnet.broadcasters) != 2 {
		t.Errorf("Expecting 2 streams to be created, but got %v", stubnet.broadcasters)
	}
	if len(n.VideoNetwork.(*StubVideoNetwork).broadcasters) != 2 {
		t.Errorf("Expecting 2 broadcasters to be created, but got %v", n.VideoNetwork.(*StubVideoNetwork).broadcasters)
	}

	// Test when transcoder fails
	tr.FailTranscode = true
	ids, err = n.TranscodeAndBroadcast(config, &StubClaimManager{}, tr)

	//TODO: Should have done the claiming
}
