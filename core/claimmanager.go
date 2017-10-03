package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	lpmscore "github.com/livepeer/lpms/core"
)

var ErrClaim = errors.New("ErrClaim")
var ErrClaimManager = errors.New("ErrClaimManager")

type ClaimManager interface {
	AddReceipt(seqNo int64, data []byte, tDataHash []byte, bSig []byte, profile lpmscore.VideoProfile) error
	SufficientBroadcasterDeposit() (bool, error)
	Claim() (claimCount int, rc chan types.Receipt, ec chan error)
	Verify() error
	DistributeFees() error
}

type claimData struct {
	seqNo       int64
	segData     []byte
	dataHash    []byte
	tDataHashes map[lpmscore.VideoProfile][]byte
	bSig        []byte
	// receiptHashes        map[lpmscore.VideoProfile]common.Hash
	claimStart           int64
	claimEnd             int64
	claimBlkNum          *big.Int
	claimBlkHash         common.Hash
	claimProof           []byte
	claimId              *big.Int
	claimConcatTDatahash []byte
}

//BasicClaimManager manages the claim process for a Livepeer transcoder.  Check the Livepeer protocol for more details.
type BasicClaimManager struct {
	client eth.LivepeerEthClient
	ipfs   ipfs.IPFSShell

	strmID   string
	jobID    *big.Int
	profiles []lpmscore.VideoProfile
	pLookup  map[lpmscore.VideoProfile]int

	segClaimMap map[int64]*claimData
	cost        *big.Int

	broadcasterAddr common.Address
	pricePerSegment *big.Int

	r *ethTypes.TranscodeReceipt
}

//NewBasicClaimManager creates a new claim manager.
func NewBasicClaimManager(sid string, jid *big.Int, broadcaster common.Address, pricePerSegment *big.Int, p []lpmscore.VideoProfile, c eth.LivepeerEthClient, ipfs ipfs.IPFSShell) *BasicClaimManager {
	seqNos := make([][]int64, len(p), len(p))
	rHashes := make([][]common.Hash, len(p), len(p))
	sd := make([][][]byte, len(p), len(p))
	dHashes := make([][]string, len(p), len(p))
	tHashes := make([][]string, len(p), len(p))
	sigs := make([][][]byte, len(p), len(p))
	pLookup := make(map[lpmscore.VideoProfile]int)

	sort.Sort(lpmscore.ByName(p))
	for i := 0; i < len(p); i++ {
		sNo := make([]int64, 0)
		seqNos[i] = sNo
		rh := make([]common.Hash, 0)
		rHashes[i] = rh
		d := make([][]byte, 0)
		sd[i] = d
		dh := make([]string, 0)
		dHashes[i] = dh
		th := make([]string, 0)
		tHashes[i] = th
		s := make([][]byte, 0)
		sigs[i] = s
		pLookup[p[i]] = i
	}
	// return &BasicClaimManager{client: c, ipfs: ipfs, strmID: sid, jobID: jid, cost: big.NewInt(0), broadcasterAddr: broadcaster, pricePerSegment: pricePerSegment, seqNos: seqNos, receiptHashes: rHashes, segData: sd, dataHashes: dHashes, tDataHashes: tHashes, bSigs: sigs, profiles: p, pLookup: pLookup}
	return &BasicClaimManager{client: c, ipfs: ipfs, strmID: sid, jobID: jid, cost: big.NewInt(0), broadcasterAddr: broadcaster, pricePerSegment: pricePerSegment, profiles: p, pLookup: pLookup, segClaimMap: make(map[int64]*claimData)}
}

//AddReceipt adds a claim for a given video segment.
func (c *BasicClaimManager) AddReceipt(seqNo int64, data []byte, tDataHash []byte, bSig []byte, profile lpmscore.VideoProfile) error {
	dataHash := crypto.Keccak256(data)

	_, ok := c.pLookup[profile]
	if !ok {
		glog.Errorf("Cannot find profile: %v", profile)
		return ErrClaimManager
	}

	cd, ok := c.segClaimMap[seqNo]
	if !ok {
		cd = &claimData{
			seqNo:       seqNo,
			segData:     data,
			dataHash:    dataHash,
			tDataHashes: make(map[lpmscore.VideoProfile][]byte),
			bSig:        bSig,
		}
		c.segClaimMap[seqNo] = cd
	}
	if _, ok := cd.tDataHashes[profile]; ok {
		return ErrClaimManager
	}
	cd.tDataHashes[profile] = tDataHash

	c.cost = new(big.Int).Add(c.cost, c.pricePerSegment)
	return nil
}

func (c *BasicClaimManager) SufficientBroadcasterDeposit() (bool, error) {
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

type SortUint64 []int64

func (a SortUint64) Len() int           { return len(a) }
func (a SortUint64) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortUint64) Less(i, j int) bool { return a[i] < a[j] }

func (c *BasicClaimManager) makeRanges() [][2]int64 {
	//Get seqNos, sort them
	keys := []int64{}
	for key := range c.segClaimMap {
		keys = append(keys, key)
	}
	sort.Sort(SortUint64(keys))

	//Iterate through, check to make sure all tHashes are present (otherwise break and start new range),
	start := keys[0]
	ranges := make([][2]int64, 0)
	for i, key := range keys {
		startNewRange := false
		scm := c.segClaimMap[key]

		//If not all profiles exist in transcoded hashes, remove current key and start new range (don't claim for current segment)
		for _, p := range c.profiles {
			if _, ok := scm.tDataHashes[p]; !ok {
				ranges = append(ranges, [2]int64{start, keys[i-1]})
				startNewRange = true
				break
			}
		}

		//If the next key is not 1 more than the current key, it's not contiguous - start a new range
		if startNewRange == false && (i+1 == len(keys) || keys[i+1] != keys[i]+1) {
			ranges = append(ranges, [2]int64{start, keys[i]})
			startNewRange = true
		}

		if startNewRange {
			if i+1 != len(keys) {
				start = keys[i+1]
			}
		}
	}
	return ranges
}

//Claim creates the onchain claim for all the claims added through AddReceipt
func (c *BasicClaimManager) Claim() (claimCount int, rc chan types.Receipt, ec chan error) {
	ranges := c.makeRanges()
	ec = make(chan error)
	rc = make(chan types.Receipt)

	for rangeIdx, segRange := range ranges {
		//create concat hashes for each seg
		receiptHashes := make([]common.Hash, segRange[1]-segRange[0]+1)
		for i := segRange[0]; i <= segRange[1]; i++ {
			segTDataHashes := make([][]byte, len(c.profiles))
			for pi, p := range c.profiles {
				segTDataHashes[pi] = []byte(c.segClaimMap[i].tDataHashes[p])
			}
			seg, _ := c.segClaimMap[i]
			seg.claimConcatTDatahash = crypto.Keccak256(segTDataHashes...)

			receipt := &ethTypes.TranscodeReceipt{
				StreamID:                 c.strmID,
				SegmentSequenceNumber:    big.NewInt(seg.seqNo),
				DataHash:                 seg.dataHash,
				ConcatTranscodedDataHash: seg.claimConcatTDatahash,
				BroadcasterSig:           seg.bSig,
			}

			receiptHashes[i-segRange[0]] = receipt.Hash()
		}

		//create merkle root for concat hashes
		root, proofs, err := ethTypes.NewMerkleTree(receiptHashes)
		if err != nil {
			glog.Errorf("Error: %v - creating merkle root for %v", err, receiptHashes)
		}

		//Do the claim
		go func(rangeIdx int, segRange [2]int64, rc chan types.Receipt, ec chan error) {
			bigRange := [2]*big.Int{big.NewInt(segRange[0]), big.NewInt(segRange[1])}
			resCh, errCh := c.client.ClaimWork(c.jobID, bigRange, root.Hash)
			select {
			case res := <-resCh:
				blkNum, blkHash, err := c.client.GetBlockInfoByTxHash(context.Background(), res.TxHash)
				if err != nil {
					glog.Infof("Error getting block number / hash: %v", err)
					ec <- err
					return
				}
				glog.Infof("Got block hash: %x, block number: %v", blkHash, blkNum)
				//Record claim information for verification later
				for i := segRange[0]; i <= segRange[1]; i++ {
					seg, _ := c.segClaimMap[i]
					seg.claimStart = segRange[0]
					seg.claimEnd = segRange[1]
					seg.claimBlkNum = blkNum
					seg.claimBlkHash = blkHash
					seg.claimProof = proofs[i-segRange[0]].Bytes()
					seg.claimId = big.NewInt(int64(rangeIdx))
				}

				rc <- res
			case err := <-errCh:
				glog.Errorf("Error claiming work: %v", err)
				ec <- err
			}
		}(rangeIdx, segRange, rc, ec)
	}

	return len(ranges), rc, ec
}

func (c *BasicClaimManager) Verify() error {
	//Get verification rate
	verifyRate, err := c.client.VerificationRate()
	if err != nil {
		glog.Errorf("Error getting verification rate: %v", err)
		return err
	}

	//Iterate through segments, determine which one needs to be verified.
	for segNo, scm := range c.segClaimMap {
		glog.Infof("blkHash: %x", scm.claimBlkHash)
		glog.Infof("segNo: %v", segNo)
		glog.Infof("blkNum: %v", scm.claimBlkNum.Int64())
		if shouldVerifySegment(segNo, scm.claimStart, scm.claimEnd, scm.claimBlkNum.Int64(), scm.claimBlkHash, verifyRate) {
			glog.Infof("Calling verify")
			//TODO: Add data to IPFS
			dataStorageHash, err := c.ipfs.Add(bytes.NewReader(c.segClaimMap[segNo].segData))
			if err != nil {
				glog.Errorf("Error uploading segment data to IPFS: %v", err)
				continue
			}

			//Call Verify
			go func() {
				dataHashes := [2][32]byte{common.BytesToHash(scm.dataHash), common.BytesToHash(scm.claimConcatTDatahash)}
				resCh, errCh := c.client.Verify(c.jobID, scm.claimId, big.NewInt(segNo), dataStorageHash, dataHashes, scm.bSig, scm.claimProof)
				select {
				case <-resCh:
					glog.Infof("Invoked verification for seg no %v", segNo)
				case err := <-errCh:
					glog.Errorf("Error submitting verify transaction: %v", err)
				}
			}()
		}
	}

	return nil
}

func (c *BasicClaimManager) DistributeFees() error {
	verificationPeriod, err := c.client.VerificationPeriod()
	if err != nil {
		return err
	}

	slashingPeriod, err := c.client.SlashingPeriod()
	if err != nil {
		return err
	}

	eth.Wait(c.client.Backend(), c.client.RpcTimeout(), new(big.Int).Add(verificationPeriod, slashingPeriod))

	for cid := range c.makeRanges() {
		resCh, errCh := c.client.DistributeFees(c.jobID, big.NewInt(int64(cid)))
		select {
		case <-resCh:
			glog.Infof("Distributed fees")

			bond, err := c.client.TranscoderBond()
			if err != nil {
				glog.Errorf("Error getting token balance: %v", err)
			}

			glog.Infof("Transcoder bond after fees: %v", bond)
		case err := <-errCh:
			glog.Infof("Error distributing fees: %v", err)
		}

	}
	return nil
}

func shouldVerifySegment(seqNum int64, start int64, end int64, blkNum int64, blkHash common.Hash, verifyRate uint64) bool {
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
