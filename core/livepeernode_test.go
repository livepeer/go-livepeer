package core

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ericxtang/m3u8"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/types"
	"github.com/livepeer/lpms/stream"
)

type StubClaimManager struct{}

func (cm *StubClaimManager) AddReceipt(seqNo int64, data []byte, tDataHash string, bSig []byte, profile types.VideoProfile) {
}
func (cm *StubClaimManager) SufficientBroadcasterDeposit() (bool, error) { return true, nil }
func (cm *StubClaimManager) Claim(p types.VideoProfile) error            { return nil }

type StubTranscoder struct {
	Profiles  []types.VideoProfile
	InputData [][]byte
}

func (t *StubTranscoder) Transcode(d []byte) ([][]byte, error) {
	if d == nil {
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
	strmID := "strmID"
	jid := big.NewInt(0)
	p := []types.VideoProfile{types.P720p60fps16x9, types.P144p30fps16x9}
	config := net.TranscodeConfig{StrmID: strmID, Profiles: p, PerformOnchainClaim: false, JobID: jid}

	n, err := NewLivepeerNode(&eth.StubClient{}, &StubVideoNetwork{}, nid, []string{""}, "")
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
	if len(n.VideoDB.streams) != 2 {
		t.Errorf("Expecting 2 streams to be created, but got %v", n.VideoDB.streams)
	}
	if len(n.VideoNetwork.(*StubVideoNetwork).broadcasters) != 2 {
		t.Errorf("Expecting 2 broadcasters to be created, but got %v", n.VideoNetwork.(*StubVideoNetwork).broadcasters)
	}

	//TODO: Should have done the claiming
}

func TestBroadcastToNetwork(t *testing.T) {
	nid := NodeID("12201c23641663bf06187a8c154a6c97266d138cb8379c1bc0828122dcc51c83698d")
	n, err := NewLivepeerNode(&eth.StubClient{}, &StubVideoNetwork{}, nid, []string{""}, "")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	strmID, err := MakeStreamID(nid, RandomVideoID(), types.P144p30fps16x9.Name)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	pl, _ := m3u8.NewMediaPlaylist(3, 10)
	testStrm, err := n.VideoDB.AddNewHLSStream(strmID)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	//Broadcast the stream
	if err := n.BroadcastStreamToNetwork(testStrm); err != nil {
		t.Errorf("Error: %v", err)
	}

	//Insert a segment into the stream
	seg := &stream.HLSSegment{SeqNo: 0, Name: fmt.Sprintf("%v_00.ts", strmID), Data: []byte("hello"), Duration: 1}
	if err := testStrm.AddHLSSegment(seg); err != nil {
		t.Errorf("Error: %v", err)
	}

	b, ok := n.VideoNetwork.(*StubVideoNetwork).broadcasters[strmID.String()]
	if !ok {
		t.Errorf("Shoudl have created a broadcaster")
	}

	if string(b.Data) != string(seg.Data) {
		t.Errorf("Expecting %v, got %v", seg.Data, b.Data)
	}

	//Broadcast the manifest
	mid, err := MakeManifestID(nid, RandomVideoID())
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	manifest, err := n.VideoDB.AddNewHLSManifest(mid)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	variant := &m3u8.Variant{URI: "test.m3u8", Chunklist: pl, VariantParams: m3u8.VariantParams{Bandwidth: 100}}
	if err := manifest.AddVideoStream(testStrm, variant); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := n.BroadcastManifestToNetwork(manifest); err != nil {
		t.Errorf("Error :%v", err)
	}
	//We should have created a playlist and inserted into the network broadcaster
	_, ok = n.VideoNetwork.(*StubVideoNetwork).mplMap[mid.String()]
	if !ok {
		t.Errorf("Should have created a playlist")
	}

	//Broadcast Finish
	if n.VideoNetwork.(*StubVideoNetwork).broadcasters[strmID.String()].FinishMsg != false {
		t.Errorf("Expecting finish to have not been called yet")
	}
	if err := n.BroadcastFinishMsg(strmID.String()); err != nil {
		t.Errorf("Error: %v", err)
	}
	if n.VideoNetwork.(*StubVideoNetwork).broadcasters[strmID.String()].FinishMsg != true {
		t.Errorf("Expecting finish to have been called")
	}
}
