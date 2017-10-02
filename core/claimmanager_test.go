package core

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	lpCommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	lpmscore "github.com/livepeer/lpms/core"
)

func TestProfileOrder(t *testing.T) {
	ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps4x3, lpmscore.P720p30fps4x3}
	cm := NewBasicClaimManager("strmID", big.NewInt(5), common.Address{}, big.NewInt(1), ps, &eth.StubClient{}, &StubShell{})

	if cm.profiles[0] != lpmscore.P720p30fps4x3 || cm.profiles[1] != lpmscore.P360p30fps4x3 || cm.profiles[2] != lpmscore.P240p30fps16x9 {
		t.Errorf("wrong ordering: %v", cm.profiles)
	}
}

func TestAddReceipt(t *testing.T) {
	ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps4x3, lpmscore.P720p30fps4x3}
	cm := NewBasicClaimManager("strmID", big.NewInt(5), common.Address{}, big.NewInt(1), ps, &eth.StubClient{}, &StubShell{})

	//Should get error for adding to a non-existing profile
	if err := cm.AddReceipt(0, []byte("data"), []byte("tdatahash"), []byte("sig"), lpmscore.P144p30fps16x9); err == nil {
		t.Errorf("Expecting an error for adding to a non-existing profile.")
	}

	if err := cm.AddReceipt(0, []byte("data"), []byte("tdatahash"), []byte("sig"), lpmscore.P240p30fps16x9); err != nil {
		t.Errorf("Error: %v", err)
	}

	if string(cm.segClaimMap[0].segData) != "data" {
		t.Errorf("Expecting %v, got %v", "data", string(cm.segClaimMap[0].segData))
	}

	if string(cm.segClaimMap[0].dataHash) != string(crypto.Keccak256([]byte("data"))) { //appended by StubShell
		t.Errorf("Expecting %v, got %v", string(crypto.Keccak256([]byte("data"))), string(cm.segClaimMap[0].dataHash))
	}

	if string(cm.segClaimMap[0].tDataHashes[lpmscore.P240p30fps16x9]) != "tdatahash" {
		t.Errorf("Expecting %v, got %v", "datahash", cm.segClaimMap[0].tDataHashes[lpmscore.P240p30fps16x9])
	}

	if string(cm.segClaimMap[0].bSig) != "sig" {
		t.Errorf("Expecting %v, got %v", "sig", string(cm.segClaimMap[0].bSig))
	}
}

//We add 6 ranges (0, 3-13, 15-18, 20-25, 27, 29)
func setupRanges(t *testing.T) *BasicClaimManager {
	ethClient := &eth.StubClient{ClaimStart: make([]*big.Int, 0), ClaimEnd: make([]*big.Int, 0), ClaimJid: make([]*big.Int, 0), ClaimRoot: make(map[[32]byte]bool)}
	ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps4x3, lpmscore.P720p30fps4x3}
	cm := NewBasicClaimManager("strmID", big.NewInt(5), common.Address{}, big.NewInt(1), ps, ethClient, &StubShell{})

	for _, segRange := range [][2]int64{[2]int64{0, 0}, [2]int64{3, 13}, [2]int64{15, 18}, [2]int64{21, 25}, [2]int64{27, 27}, [2]int64{29, 29}} {
		for i := segRange[0]; i <= segRange[1]; i++ {
			for _, p := range ps {
				data := []byte(fmt.Sprintf("data%v%v", p.Name, i))
				hash := []byte(fmt.Sprintf("hash%v%v", p.Name, i))
				sig := []byte(fmt.Sprintf("sig%v%v", p.Name, i))
				if err := cm.AddReceipt(int64(i), []byte(data), hash, []byte(sig), p); err != nil {
					t.Errorf("Error: %v", err)
				}
				if i == 16 {
					glog.Infof("data: %v", data)
				}
			}
		}
	}

	//Add invalid 19 out of order (invalid because it's only a single video profile)
	i := 19
	p := lpmscore.P360p30fps4x3
	data := []byte(fmt.Sprintf("data%v%v", p.Name, i))
	hash := []byte(fmt.Sprintf("hash%v%v", p.Name, i))
	sig := []byte(fmt.Sprintf("sig%v%v", p.Name, i))
	if err := cm.AddReceipt(int64(i), []byte(data), hash, []byte(sig), p); err != nil {
		t.Errorf("Error: %v", err)
	}

	return cm
}

func TestRanges(t *testing.T) {
	//We added 6 ranges (0, 3-13, 15-18, 20-25, 27, 29)
	cm := setupRanges(t)
	ranges := cm.makeRanges()
	glog.Infof("ranges: %v", ranges)
	if len(ranges) != 6 {
		t.Errorf("Expecting 6 ranges, got %v", ranges)
	}
}

func TestClaim(t *testing.T) {
	ethClient := &eth.StubClient{ClaimStart: make([]*big.Int, 0), ClaimEnd: make([]*big.Int, 0), ClaimJid: make([]*big.Int, 0), ClaimRoot: make(map[[32]byte]bool)}
	ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps4x3, lpmscore.P720p30fps4x3}
	cm := NewBasicClaimManager("strmID", big.NewInt(5), common.Address{}, big.NewInt(1), ps, ethClient, &StubShell{})

	//Add some receipts(0-9)
	receiptHashes1 := make([]common.Hash, 10)
	for i := 0; i < 10; i++ {
		segTDataHashes := make([][]byte, len(ps))
		data := []byte(fmt.Sprintf("data%v", i))
		sig := []byte(fmt.Sprintf("sig%v", i))
		for pi, p := range ps {
			tHash := []byte(fmt.Sprintf("tHash%v%v", lpmscore.P240p30fps16x9.Name, i))
			if err := cm.AddReceipt(int64(i), data, tHash, []byte(sig), p); err != nil {
				t.Errorf("Error: %v", err)
			}
			segTDataHashes[pi] = tHash
		}
		receipt := &ethTypes.TranscodeReceipt{
			StreamID:                 "strmID",
			SegmentSequenceNumber:    big.NewInt(int64(i)),
			DataHash:                 crypto.Keccak256(data),
			ConcatTranscodedDataHash: crypto.Keccak256(segTDataHashes...),
			BroadcasterSig:           []byte(sig),
		}
		receiptHashes1[i] = receipt.Hash()
	}

	//Add some receipts(15-24)
	receiptHashes2 := make([]common.Hash, 10)
	for i := 15; i < 25; i++ {
		segTDataHashes := make([][]byte, len(ps))
		data := []byte(fmt.Sprintf("data%v", i))
		sig := []byte(fmt.Sprintf("sig%v", i))
		for pi, p := range ps {
			tHash := []byte(fmt.Sprintf("tHash%v%v", lpmscore.P240p30fps16x9.Name, i))
			if err := cm.AddReceipt(int64(i), []byte(data), tHash, []byte(sig), p); err != nil {
				t.Errorf("Error: %v", err)
			}

			segTDataHashes[pi] = tHash
		}
		receipt := &ethTypes.TranscodeReceipt{
			StreamID:                 "strmID",
			SegmentSequenceNumber:    big.NewInt(int64(i)),
			DataHash:                 crypto.Keccak256(data),
			ConcatTranscodedDataHash: crypto.Keccak256(segTDataHashes...),
			BroadcasterSig:           []byte(sig),
		}
		receiptHashes2[i-15] = receipt.Hash()
	}

	//call Claim
	count, rc, ec := cm.Claim()
	timer := time.NewTimer(500 * time.Millisecond)
	select {
	case <-rc:
		count = count - 1
		if count == 0 {
			break
		}
	case err := <-ec:
		t.Errorf("Error: %v", err)
	case <-timer.C:
		t.Errorf("Timed out")
	}

	//Make sure the roots are used for calling Claim
	root1, _, err := ethTypes.NewMerkleTree(receiptHashes1)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, ok := ethClient.ClaimRoot[[32]byte(root1.Hash)]; !ok {
		t.Errorf("Expecting claim to have root %v, but got %v", [32]byte(root1.Hash), ethClient.ClaimRoot)
	}

	root2, _, err := ethTypes.NewMerkleTree(receiptHashes2)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, ok := ethClient.ClaimRoot[[32]byte(root2.Hash)]; !ok {
		t.Errorf("Expecting claim to have root %v, but got %v", [32]byte(root2.Hash), ethClient.ClaimRoot)
	}
}

func TestVerify(t *testing.T) {
	// ethClient := &eth.StubClient{ClaimStart: make([]*big.Int, 0), ClaimEnd: make([]*big.Int, 0), ClaimJid: make([]*big.Int, 0), ClaimRoot: make(map[[32]byte]bool)}
	ethClient := &eth.StubClient{VeriRate: 10}
	ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps4x3, lpmscore.P720p30fps4x3}
	cm := NewBasicClaimManager("strmID", big.NewInt(5), common.Address{}, big.NewInt(1), ps, ethClient, &StubShell{})
	start := int64(0)
	end := int64(100)
	blkNum := int64(100)
	blkHash := common.HexToHash("0xDEADBEEF")
	seqNum := int64(10)
	//Find a seqNum that will trigger verification
	for i := 0; i < 100; i++ {
		blkHash = common.HexToHash(fmt.Sprintf("0xDEADBEEF%v", i))
		if shouldVerifySegment(seqNum, start, end, blkNum, blkHash, 10) {
			break
		}
	}

	//Add a seg that shouldn't trigger
	cm.AddReceipt(seqNum, []byte("data240"), []byte("tDataHash240"), []byte("bSig"), lpmscore.P240p30fps16x9)
	cm.AddReceipt(seqNum, []byte("data360"), []byte("tDataHash360"), []byte("bSig"), lpmscore.P360p30fps4x3)
	cm.AddReceipt(seqNum, []byte("data720"), []byte("tDataHash720"), []byte("bSig"), lpmscore.P720p30fps4x3)
	seg, _ := cm.segClaimMap[seqNum]
	seg.claimStart = start
	seg.claimEnd = end
	seg.claimBlkNum = big.NewInt(blkNum)
	seg.claimBlkHash = common.HexToHash("0xDEADBEEF")

	if err := cm.Verify(); err != nil {
		t.Errorf("Error: %v", err)
	}
	if ethClient.VerifyCounter != 0 {
		t.Errorf("Expect verify to NOT be triggered")
	}

	// //Add a seg that SHOULD trigger
	cm.AddReceipt(seqNum, []byte("data240"), []byte("tDataHash240"), []byte("bSig"), lpmscore.P240p30fps16x9)
	cm.AddReceipt(seqNum, []byte("data360"), []byte("tDataHash360"), []byte("bSig"), lpmscore.P360p30fps4x3)
	cm.AddReceipt(seqNum, []byte("data720"), []byte("tDataHash720"), []byte("bSig"), lpmscore.P720p30fps4x3)
	seg, _ = cm.segClaimMap[seqNum]
	seg.claimStart = start
	seg.claimEnd = end
	seg.claimBlkNum = big.NewInt(blkNum)
	seg.claimBlkHash = blkHash
	seg.claimProof = []byte("proof")

	if err := cm.Verify(); err != nil {
		t.Errorf("Error: %v", err)
	}
	lpCommon.WaitUntil(100*time.Millisecond, func() bool {
		return ethClient.VerifyCounter != 0
	})
	if ethClient.VerifyCounter != 1 {
		t.Errorf("Expect verify to be triggered")
	}
	if string(ethClient.Proof) != string(seg.claimProof) {
		t.Errorf("Expect proof to be %v, got %v", seg.claimProof, ethClient.Proof)
	}
}
