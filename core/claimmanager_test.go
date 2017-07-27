package core

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/types"
)

func TestClaimAndVerify(t *testing.T) {
	//Prep the data
	jid := big.NewInt(0)
	vidLen := 10
	strmID := "strmID"
	profiles := []types.VideoProfile{types.P_240P_30FPS_16_9}
	s1 := &eth.StubClient{}
	cm1 := NewClaimManager(strmID, jid, profiles, s1)

	//Add claim info
	for i := 1; i < vidLen-1; i++ {
		dh := common.StringToHash(fmt.Sprintf("dh%v", i))
		th := common.StringToHash(fmt.Sprintf("th%v", i))
		sig := (&ethTypes.Segment{strmID, big.NewInt(int64(i)), dh}).Hash().Bytes()
		cm1.AddClaim(int64(i), dh, th, sig, types.P_240P_30FPS_16_9)
	}

	VerifyRate = 10 //Set the verify rate so it gets called once
	cm1.Claim(types.P_240P_30FPS_16_9)

	if s1.ClaimCounter != 1 {
		t.Errorf("Claim should be called once, but got %v", s1.ClaimCounter)
	}

	//Test calling claim twice when claiming non-consecutive elements
	s2 := &eth.StubClient{ClaimEnd: make([]*big.Int, 0, 10), ClaimJid: make([]*big.Int, 0, 10), ClaimRoot: make([][32]byte, 0, 10), ClaimStart: make([]*big.Int, 0, 10)}
	cm2 := NewClaimManager(strmID, jid, profiles, s2)
	tcHashes := make([]common.Hash, 0)

	//Take #4 out
	for i := 1; i < vidLen-1; i++ {
		if i == 4 {
			continue
		}
		dh := common.StringToHash(fmt.Sprintf("dh%v", i))
		th := common.StringToHash(fmt.Sprintf("th%v", i))
		sig := (&ethTypes.Segment{strmID, big.NewInt(int64(i)), dh}).Hash().Bytes()
		seqNo := int64(i + 10)
		claim := &ethTypes.TranscodeClaim{
			StreamID:              strmID,
			SegmentSequenceNumber: big.NewInt(seqNo),
			DataHash:              dh,
			TranscodedDataHash:    th,
			BroadcasterSig:        sig,
		}
		tcHashes = append(tcHashes, claim.Hash())
		cm2.AddClaim(seqNo, dh, th, sig, types.P_240P_30FPS_16_9)
	}
	cm2.Claim(profiles[0])
	if s2.ClaimCounter != 2 {
		t.Errorf("Claim should be called twice, but got %v", s2.ClaimCounter)
	}

	if s2.ClaimStart[0].Cmp(big.NewInt(11)) != 0 || s2.ClaimEnd[0].Cmp(big.NewInt(13)) != 0 {
		t.Errorf("First claim should be from 11 to 13, but got %v to %v", s2.ClaimStart[0], s2.ClaimEnd[0])
	}

	if s2.ClaimStart[1].Cmp(big.NewInt(15)) != 0 || s2.ClaimEnd[1].Cmp(big.NewInt(18)) != 0 {
		t.Errorf("Second claim should be from 15 to 18, but got %v to %v", s2.ClaimStart[1], s2.ClaimEnd[1])
	}

	root1, _, _ := ethTypes.NewMerkleTree(tcHashes[0:3])
	root2, _, _ := ethTypes.NewMerkleTree(tcHashes[3:7])
	if bytes.Compare(s2.ClaimRoot[0][:], root1.Hash[:]) != 0 {
		t.Errorf("Expecting claim root %v, got %v", root1.Hash, s2.ClaimRoot[0])
	}
	if bytes.Compare(s2.ClaimRoot[1][:], root2.Hash[:]) != 0 {
		t.Errorf("Expecting claim root %v, got %v", root2.Hash, s2.ClaimRoot[1])
	}
}

func TestVerify(t *testing.T) {
	dataHashes := []common.Hash{common.StringToHash("dh1"), common.StringToHash("dh2"), common.StringToHash("dh3")}
	tHashes := []common.Hash{common.StringToHash("th1"), common.StringToHash("th2"), common.StringToHash("th3")}
	sigs := [][]byte{[]byte("sig1"), []byte("sig2"), []byte("sig3")}
	proofs := []*ethTypes.MerkleProof{&ethTypes.MerkleProof{}, &ethTypes.MerkleProof{}, &ethTypes.MerkleProof{}}
	s := &eth.StubClient{}
	verify(big.NewInt(0), dataHashes, tHashes, sigs, proofs, 0, 2, s, common.Hash{})

	if s.Jid.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("Expecting 0, got %v", s.Jid)
	}
	if fmt.Sprintf("%x", s.DHash) != fmt.Sprintf("%x", common.StringToHash("dh3")) {
		t.Errorf("Expecting %v, got %v", common.StringToHash("dh3"), s.DHash)
	}
	if fmt.Sprintf("%x", s.TDHash) != fmt.Sprintf("%x", common.StringToHash("th3")) {
		t.Errorf("Expecting %v, got %v", common.StringToHash("th3"), s.TDHash)
	}
	if s.VerifyCounter != 3 {
		t.Errorf("Verify should be called 3 times, but got %v", s.VerifyCounter)
	}
}
