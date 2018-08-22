package core

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

type StubClaimManager struct {
	verifyCalled         bool
	distributeFeesCalled bool
	receiptAdded         bool
}

func (cm *StubClaimManager) BroadcasterAddr() common.Address { return common.Address{} }
func (cm *StubClaimManager) AddReceipt(seqNo int64, fname string, data []byte, bSig []byte, tData map[ffmpeg.VideoProfile][]byte, tStart time.Time, tEnd time.Time) ([]byte, error) {
	cm.receiptAdded = true
	return []byte{}, nil
}
func (cm *StubClaimManager) SufficientBroadcasterDeposit() (bool, error)   { return true, nil }
func (cm *StubClaimManager) ClaimVerifyAndDistributeFees() error           { return nil }
func (cm *StubClaimManager) CanClaim(*big.Int, *lpTypes.Job) (bool, error) { return true, nil }

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
	strmID, _ := MakeStreamID(RandomVideoID(), ffmpeg.P720p30fps4x3.Name)
	jid := big.NewInt(0)
	ffmpeg.InitFFmpeg()
	p := []ffmpeg.VideoProfile{ffmpeg.P720p60fps16x9, ffmpeg.P144p30fps16x9}
	tr := &StubTranscoder{Profiles: p}
	mkid := func(p ffmpeg.VideoProfile) StreamID {
		s, _ := MakeStreamID(strmID.GetVideoID(), p.Name)
		return s
	}
	strmIds := []StreamID{mkid(p[0]), mkid(p[1])}
	cm := StubClaimManager{}
	config := transcodeConfig{StrmID: strmID.String(), Profiles: p, ResultStrmIDs: strmIds, ClaimManager: &cm, JobID: jid, Transcoder: tr}

	n, err := NewLivepeerNode(&eth.StubClient{}, ".", nil) // TODO fix empty work dir
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	ss := StubSegment()
	res := n.transcodeAndCacheSeg(config, ss)
	if res.Err != nil {
		t.Errorf("Error: %v", res.Err)
	}

	if len(res.Urls) != len(p) {
		t.Errorf("Expecting %v profiles, got %v", len(p), len(res.Urls))
	}

	//Should have transcoded the segments into 2 different profiles (right now StubSubscriber emits 1 segment)
	if len(tr.InputData) != 1 {
		t.Errorf("Expecting 1 segment to be transcoded, got %v", tr.InputData)
	}

	// Check playlist was updated
	for _, v := range strmIds {
		pl := n.VideoSource.GetHLSMediaPlaylist(v)
		if pl == nil {
			t.Error("Expected media playlist; got none")
		}
		if len(pl.Segments) != 1 && pl.SeqNo != 100 {
			t.Error("Mismatched segments (expected 1) or seq (expected 100), got ", pl.Segments, pl.SeqNo)
		}
	}

	if !cm.receiptAdded {
		t.Error("Receipt was not added ", cm.receiptAdded)
	}
	// TODO check sig?

	// Test when transcoder fails
	tr.FailTranscode = true
	res = n.transcodeAndCacheSeg(config, ss)
	if res.Err == nil {
		t.Error("Expecting a transcode error")
	}
	tr.FailTranscode = false

	// Test when the number of results mismatchches expectations
	tr.Profiles = []ffmpeg.VideoProfile{p[0]}
	res = n.transcodeAndCacheSeg(config, ss)
	if res.Err == nil || res.Err.Error() != "MismatchedSegments" {
		t.Error("Did not get mismatched segments as expected")
	}
	tr.Profiles = p

	//TODO: Should have done the claiming

}

func TestNodeClaimManager(t *testing.T) {
	n, err := NewLivepeerNode(nil, ".", nil)

	job := &lpTypes.Job{
		JobId:              big.NewInt(15),
		MaxPricePerSegment: big.NewInt(1),
		Profiles:           []ffmpeg.VideoProfile{},
		TotalClaims:        big.NewInt(0),
	}

	// test claimmanager existence via manual insertion
	n.ClaimManagers[15] = &StubClaimManager{}
	cm, err := n.GetClaimManager(job)
	if err != nil || cm == nil {
		t.Errorf("Did not retrieve claimmanager %v %v", cm, err)
		return
	}

	job.JobId = big.NewInt(10)

	// test with a nil eth client
	cm, err = n.GetClaimManager(job)
	if err != nil || cm != nil {
		t.Errorf("Claimmanager unexpected result %v %v", cm, err)
		return
	}

	// test with a nonexisting job
	stubClient := eth.StubClient{}
	n.Eth = &stubClient
	cm, err = n.GetClaimManager(nil)
	if err == nil || err.Error() != "Nil job" {
		t.Error("Expected nil job ", err)
		return
	}

	// test creating a new claimmanager
	cm, err = n.GetClaimManager(job)
	if err != nil || cm == nil {
		t.Error("Expected claimmanager from job ", cm, err)
		return
	}
}
