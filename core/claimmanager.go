package core

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
)

var ErrClaim = errors.New("ErrClaim")

//ClaimManager manages the claim process for a Livepeer transcoder.  Check the Livepeer protocol for more details.
type ClaimManager struct {
	client eth.LivepeerEthClient

	strmID   string
	jobID    *big.Int
	profiles []net.VideoProfile
	pLookup  map[net.VideoProfile]int

	seqNos      [][]int64
	claimHashes [][]common.Hash
	dataHashes  [][]common.Hash
	tDataHashes [][]common.Hash
	bSigs       [][][]byte
}

//NewClaimManager creates a new claim manager.
func NewClaimManager(sid string, jid *big.Int, p []net.VideoProfile, c eth.LivepeerEthClient) *ClaimManager {
	seqNos := make([][]int64, len(p), len(p))
	tcHashes := make([][]common.Hash, len(p), len(p))
	dHashes := make([][]common.Hash, len(p), len(p))
	tHashes := make([][]common.Hash, len(p), len(p))
	sigs := make([][][]byte, len(p), len(p))
	pLookup := make(map[net.VideoProfile]int)

	for i := 0; i < len(p); i++ {
		sNo := make([]int64, 0)
		seqNos[i] = sNo
		tch := make([]common.Hash, 0)
		tcHashes[i] = tch
		dh := make([]common.Hash, 0)
		dHashes[i] = dh
		th := make([]common.Hash, 0)
		tHashes[i] = th
		s := make([][]byte, 0)
		sigs[i] = s
		pLookup[p[i]] = i
	}

	return &ClaimManager{client: c, strmID: sid, jobID: jid, seqNos: seqNos, claimHashes: tcHashes, dataHashes: dHashes, tDataHashes: tHashes, bSigs: sigs, profiles: p, pLookup: pLookup}
}

//AddClaim adds a claim for a given video segment.
func (c *ClaimManager) AddClaim(seqNo int64, dataHash common.Hash, tDataHash common.Hash, bSig []byte, profile net.VideoProfile) {
	claim := &ethTypes.TranscodeClaim{
		StreamID:              c.strmID,
		SegmentSequenceNumber: big.NewInt(seqNo),
		DataHash:              dataHash,
		TranscodedDataHash:    tDataHash,
		BroadcasterSig:        bSig,
	}

	pi, ok := c.pLookup[profile]
	if !ok {
		glog.Errorf("Cannot find profile: %v", profile)
		return
	}

	if len(c.seqNos[pi]) != 0 && c.seqNos[pi][len(c.seqNos[pi])-1] >= seqNo {
		glog.Errorf("Cannot insert out of order.  Trying to insert %v into %v", c.seqNos[pi], seqNo)
	}

	c.seqNos[pi] = append(c.seqNos[pi], seqNo)
	c.claimHashes[pi] = append(c.claimHashes[pi], claim.Hash())
	c.dataHashes[pi] = append(c.dataHashes[pi], dataHash)
	c.tDataHashes[pi] = append(c.tDataHashes[pi], tDataHash)
	c.bSigs[pi] = append(c.bSigs[pi], bSig)
}

//Claim creates the onchain claim for all the claims added through AddClaim
func (c *ClaimManager) Claim(p net.VideoProfile) error {
	pi, ok := c.pLookup[p]
	if !ok {
		glog.Errorf("Cannot find video profile: %v", p)
		return ErrClaim
	}

	c.claimAndVerify(c.jobID, c.seqNos[pi], c.dataHashes[pi], c.claimHashes[pi], c.tDataHashes[pi], c.bSigs[pi])
	return nil
}

//TODO: Can't just check for empty - need to check for seq No...
func (c *ClaimManager) claimAndVerify(jid *big.Int, seqNos []int64, dHashes []common.Hash, tcHashes []common.Hash, thashes []common.Hash, sigs [][]byte) {
	claimLen := len(seqNos)
	if len(dHashes) != claimLen || len(tcHashes) != claimLen || len(thashes) != claimLen || len(sigs) != claimLen {
		glog.Errorf("Claim data length doesn't match")
		return
	}

	ranges := make([][2]int64, 0)
	start := seqNos[0]
	for i := int64(0); i < int64(len(seqNos)); i++ {
		if i+1 == int64(len(seqNos)) || seqNos[i+1] != seqNos[i]+1 {
			ranges = append(ranges, [2]int64{start, seqNos[i]})
			if i+1 != int64(len(seqNos)) {
				start = seqNos[i+1]
			}
			continue
		}
	}

	// glog.Infof("seq: %v, ranges: %v", seqNos, ranges)

	startIdx := int64(0)
	for _, r := range ranges {
		endIdx := startIdx + r[1] - r[0]
		root, proofs, err := ethTypes.NewMerkleTree(tcHashes[startIdx : endIdx+1])
		if err != nil {
			glog.Errorf("Error: %v - creating merkle root for: %v", err, tcHashes[startIdx:endIdx+1])
			//TODO: If this happens, should we cancel the job?
		}

		tx, err := c.client.ClaimWork(jid, big.NewInt(r[0]), big.NewInt(r[1]), root.Hash)
		if err != nil {
			glog.Errorf("Error claiming work: %v", err)
		} else {
			verify(jid, dHashes[startIdx:endIdx+1], thashes[startIdx:endIdx+1], sigs[startIdx:endIdx+1], proofs, r[0], r[1], c.client, tx.Hash())
		}

		startIdx = endIdx + 1
	}
}

func verify(jid *big.Int, dataHashes []common.Hash, tHashes []common.Hash, sigs [][]byte, proofs []*ethTypes.MerkleProof, start, end int64, c eth.LivepeerEthClient, txHash common.Hash) {
	num := end - start + 1
	if len(dataHashes) != int(num) || len(tHashes) != int(num) || len(sigs) != int(num) || len(proofs) != int(num) {
		glog.Errorf("Wrong input data length in verify: dHashes(%v), tHashes(%v), sigs(%v), proofs(%v)", len(dataHashes), len(tHashes), len(sigs), len(proofs))
	}
	//Wait until tx is mined
	_, err := eth.WaitForMinedTx(c.Backend(), EthRpcTimeout, EthMinedTxTimeout, txHash)
	if err != nil {
		glog.Errorf("Error waiting for tx mine in verify: %v", err)
	}

	//Get block info
	bNum, bHash := getBlockInfo(c)

	for i := 0; i < len(dataHashes); i++ {
		//Figure out which seg needs to be verified
		if shouldVerifySegment(start+int64(i), start, end, int64(bNum), bHash, VerifyRate) {
			//Call verify
			_, err := c.Verify(jid, big.NewInt(start+int64(i)), dataHashes[i], tHashes[i], sigs[i], proofs[i].Bytes())
			if err != nil {
				glog.Errorf("Error submitting verify transaction: %v", err)
			}
		}
	}
}

func getBlockInfo(c eth.LivepeerEthClient) (uint64, common.Hash) {
	if c.Backend() == nil {
		return 0, common.StringToHash("abc")
	} else {
		sp, err := c.Backend().SyncProgress(context.Background())
		if err != nil || sp == nil {
			glog.Errorf("Error getting block: %v", err)
			return 0, common.Hash{}
		}
		blk, err := c.Backend().BlockByNumber(context.Background(), big.NewInt(int64(sp.CurrentBlock)))
		if err != nil {
			glog.Errorf("Error getting block: %v", err)
		}
		return blk.NumberU64(), blk.Hash()
	}
}

func shouldVerifySegment(seqNum int64, start int64, end int64, blkNum int64, blkHash common.Hash, verifyRate int64) bool {
	if seqNum < start || seqNum > end {
		return false
	}

	blkNumTmp := make([]byte, 8)
	binary.PutVarint(blkNumTmp, blkNum)
	blkNumB := make([]byte, 32)
	copy(blkNumB[24:], blkNumTmp)

	seqNumTmp := make([]byte, 8)
	binary.PutVarint(seqNumTmp, seqNum)
	seqNumB := make([]byte, 32)
	copy(seqNumB[24:], blkNumTmp)

	num, i := binary.Uvarint(crypto.Keccak256(blkNumB, blkHash.Bytes(), seqNumB))
	if i != 0 {
		glog.Errorf("Error converting bytes in shouldVerifySegment.  num: %v, i: %v.  blkNumB:%x, blkHash:%x, seqNumB:%x", num, i, blkNumB, blkHash.Bytes(), seqNumB)
	}
	if num%uint64(VerifyRate) == 0 {
		return true
	} else {
		return false
	}
}
