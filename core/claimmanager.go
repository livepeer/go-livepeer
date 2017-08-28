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
	"github.com/livepeer/go-livepeer/types"
)

var ErrClaim = errors.New("ErrClaim")

//ClaimManager manages the claim process for a Livepeer transcoder.  Check the Livepeer protocol for more details.
type ClaimManager struct {
	client eth.LivepeerEthClient

	strmID   string
	jobID    *big.Int
	profiles []types.VideoProfile
	pLookup  map[types.VideoProfile]int

	seqNos        [][]int64
	receiptHashes [][]common.Hash
	dataHashes    [][]string
	tDataHashes   [][]string
	bSigs         [][][]byte

	cost *big.Int

	broadcasterAddr common.Address
	pricePerSegment *big.Int
}

//NewClaimManager creates a new claim manager.
func NewClaimManager(sid string, jid *big.Int, broadcaster common.Address, pricePerSegment *big.Int, p []types.VideoProfile, c eth.LivepeerEthClient) *ClaimManager {
	seqNos := make([][]int64, len(p), len(p))
	rHashes := make([][]common.Hash, len(p), len(p))
	dHashes := make([][]string, len(p), len(p))
	tHashes := make([][]string, len(p), len(p))
	sigs := make([][][]byte, len(p), len(p))
	pLookup := make(map[types.VideoProfile]int)

	for i := 0; i < len(p); i++ {
		sNo := make([]int64, 0)
		seqNos[i] = sNo
		rh := make([]common.Hash, 0)
		rHashes[i] = rh
		dh := make([]string, 0)
		dHashes[i] = dh
		th := make([]string, 0)
		tHashes[i] = th
		s := make([][]byte, 0)
		sigs[i] = s
		pLookup[p[i]] = i
	}

	return &ClaimManager{client: c, strmID: sid, jobID: jid, cost: big.NewInt(0), broadcasterAddr: broadcaster, pricePerSegment: pricePerSegment, seqNos: seqNos, receiptHashes: rHashes, dataHashes: dHashes, tDataHashes: tHashes, bSigs: sigs, profiles: p, pLookup: pLookup}
}

//AddClaim adds a claim for a given video segment.
func (c *ClaimManager) AddReceipt(seqNo int64, dataHash string, tDataHash string, bSig []byte, profile types.VideoProfile) {
	receipt := &ethTypes.TranscodeReceipt{
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

	glog.Infof("Add receipt. Seq no %v Receipt hash %v Data hash %v Tdata hash %v BSig %v", seqNo, receipt.Hash(), dataHash, tDataHash, bSig)

	c.seqNos[pi] = append(c.seqNos[pi], seqNo)
	c.receiptHashes[pi] = append(c.receiptHashes[pi], receipt.Hash())
	c.dataHashes[pi] = append(c.dataHashes[pi], dataHash)
	c.tDataHashes[pi] = append(c.tDataHashes[pi], tDataHash)
	c.bSigs[pi] = append(c.bSigs[pi], bSig)
	c.cost = new(big.Int).Add(c.cost, c.pricePerSegment)
}

func (c *ClaimManager) SufficientBroadcasterDeposit() (bool, error) {
	bDeposit, err := c.client.GetBroadcasterDeposit(c.broadcasterAddr)
	if err != nil {
		glog.Errorf("Error getting broadcaster deposit: %v", err)
		return false, err
	}

	//If broadcaster does not have enough for a segment, return false
	//If broadcaster has enough for at least one transcoded segment, return true
	currDeposit := new(big.Int).Sub(bDeposit, c.cost)
	if new(big.Int).Sub(currDeposit, new(big.Int).Mul(big.NewInt(int64(len(c.profiles))), c.pricePerSegment)).Cmp(big.NewInt(0)) == -1 {
		return false, nil
	} else {
		return true, nil
	}
}

//Claim creates the onchain claim for all the claims added through AddClaim
func (c *ClaimManager) Claim(p types.VideoProfile) error {
	pi, ok := c.pLookup[p]
	if !ok {
		glog.Errorf("Cannot find video profile: %v", p)
		return ErrClaim
	}

	if len(c.seqNos[pi]) == 0 {
		glog.Infof("No segments to claim for %v", pi)
	} else {
		go claimVerifyDistribute(c.client, c.jobID, c.seqNos[pi], c.dataHashes[pi], c.receiptHashes[pi], c.tDataHashes[pi], c.bSigs[pi])
	}

	return nil
}

//TODO: Can't just check for empty - need to check for seq No...
func claimVerifyDistribute(client eth.LivepeerEthClient, jid *big.Int, seqNos []int64, dHashes []string, rHashes []common.Hash, tHashes []string, sigs [][]byte) {
	claimLen := len(seqNos)
	if len(dHashes) != claimLen || len(rHashes) != claimLen || len(tHashes) != claimLen || len(sigs) != claimLen {
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
		}
	}

	glog.Infof("Segment ranges: %v", ranges)

	startIdx := int64(0)
	for idx, r := range ranges {
		endIdx := startIdx + r[1] - r[0]

		root, proofs, err := ethTypes.NewMerkleTree(rHashes[startIdx : endIdx+1])
		if err != nil {
			glog.Errorf("Error: %v - creating merkle root for: %v", err, rHashes[startIdx:endIdx+1])
			//TODO: If this happens, should we cancel the job?
		}

		glog.Infof("Submitting claim root: %v", root.Hash.Hex())

		resCh, errCh := client.ClaimWork(jid, [2]*big.Int{big.NewInt(r[0]), big.NewInt(r[1])}, [32]byte(root.Hash))
		select {
		case <-resCh:
			bNum, bHash, err := getBlockInfo(client)
			if err != nil {
				glog.Errorf("Error getting block info: %v", err)
				return
			}

			verify(client, jid, big.NewInt(int64(idx)), dHashes[startIdx:endIdx+1], tHashes[startIdx:endIdx+1], sigs[startIdx:endIdx+1], proofs, r[0], r[1], int64(bNum), bHash)
		case err := <-errCh:
			glog.Errorf("Error claiming work: %v", err)
		}

		startIdx = endIdx + 1
	}
}

func verify(client eth.LivepeerEthClient, jid *big.Int, cid *big.Int, dataHashes []string, tHashes []string, sigs [][]byte, proofs []*ethTypes.MerkleProof, start, end int64, bNum int64, bHash common.Hash) {
	num := end - start + 1
	if len(dataHashes) != int(num) || len(tHashes) != int(num) || len(sigs) != int(num) || len(proofs) != int(num) {
		glog.Errorf("Wrong input data length in verify: dHashes(%v), tHashes(%v), sigs(%v), proofs(%v)", len(dataHashes), len(tHashes), len(sigs), len(proofs))
	}

	verifyRate, err := client.VerificationRate()
	if err != nil {
		glog.Errorf("Error getting verification rate: %v", err)
		return
	}

	glog.Infof("Checking which segments need to be verified...")
	for i := 0; i < len(dataHashes); i++ {
		if shouldVerifySegment(start+int64(i), start, end, int64(bNum), bHash, int64(verifyRate)) {
			glog.Infof("Should verify seg no %v", start+int64(i))
			//Call verify
			resCh, errCh := client.Verify(jid, cid, big.NewInt(start+int64(i)), dataHashes[i], tHashes[i], sigs[i], proofs[i].Bytes())
			select {
			case <-resCh:
				glog.Infof("Invoked verification for seg no %v", start+int64(i))
			case err := <-errCh:
				glog.Errorf("Error submitting verify transaction: %v", err)
			}
		} else {
			glog.Infof("Don't need to verify seg no %v", start+int64(i))
		}
	}

	distributeFees(client, jid, cid)
}

func distributeFees(client eth.LivepeerEthClient, jid *big.Int, cid *big.Int) {
	verificationPeriod, err := client.VerificationPeriod()
	if err != nil {
		return
	}

	slashingPeriod, err := client.SlashingPeriod()
	if err != nil {
		return
	}

	eth.Wait(client.Backend(), client.RpcTimeout(), new(big.Int).Add(verificationPeriod, slashingPeriod))

	resCh, errCh := client.DistributeFees(jid, cid)
	select {
	case <-resCh:
		glog.Infof("Distributed fees")

		bond, err := client.TranscoderBond()
		if err != nil {
			glog.Errorf("Error getting token balance: %v", err)
		}

		glog.Infof("Transcoder bond after fees: %v", bond)
	case err := <-errCh:
		glog.Infof("Error distributing fees: %v", err)
	}
}

func getBlockInfo(client eth.LivepeerEthClient) (uint64, common.Hash, error) {
	ctx, _ := context.WithTimeout(context.Background(), client.RpcTimeout())

	block, err := client.Backend().BlockByNumber(ctx, nil)
	if err != nil {
		return 0, common.Hash{}, err
	}

	return block.NumberU64(), block.Hash(), nil
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
	if i == 0 {
		glog.Errorf("Error converting bytes in shouldVerifySegment.  num: %v, i: %v.  blkNumB:%x, blkHash:%x, seqNumB:%x", num, i, blkNumB, blkHash.Bytes(), seqNumB)
	}
	if num%uint64(verifyRate) == 0 {
		return true
	} else {
		return false
	}
}
