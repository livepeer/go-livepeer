package eth

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var (
	RpcTimeout = 10 * time.Second
)

type ClaimManager interface {
	AddReceipt(seqNo int64, bData []byte, bSig []byte, tData map[ffmpeg.VideoProfile][]byte) error
	SufficientBroadcasterDeposit() (bool, error)
	ClaimVerifyAndDistributeFees() error
	CanClaim() (bool, error)
	DidFirstClaim() bool
	BroadcasterAddr() common.Address
}

type claimData struct {
	seqNo                int64
	segData              []byte
	dataHash             []byte
	bSig                 []byte
	transcodeProof       []byte
	claimConcatTDatahash []byte
}

//BasicClaimManager manages the claim process for a Livepeer transcoder.  Check the Livepeer protocol for more details.
type BasicClaimManager struct {
	client LivepeerEthClient
	ipfs   ipfs.IpfsApi

	strmID   string
	jobID    *big.Int
	profiles []ffmpeg.VideoProfile
	pLookup  map[ffmpeg.VideoProfile]int

	segClaimMap   map[int64]*claimData
	unclaimedSegs map[int64]bool
	cost          *big.Int

	broadcasterAddr common.Address
	pricePerSegment *big.Int
	totalSegCost    *big.Int

	claims     int64
	claimsLock sync.Mutex
}

//NewBasicClaimManager creates a new claim manager.
func NewBasicClaimManager(sid string, jid *big.Int, broadcaster common.Address, pricePerSegment *big.Int, p []ffmpeg.VideoProfile, c LivepeerEthClient, ipfs ipfs.IpfsApi) *BasicClaimManager {
	seqNos := make([][]int64, len(p), len(p))
	rHashes := make([][]common.Hash, len(p), len(p))
	sd := make([][][]byte, len(p), len(p))
	dHashes := make([][]string, len(p), len(p))
	tHashes := make([][]string, len(p), len(p))
	sigs := make([][][]byte, len(p), len(p))
	pLookup := make(map[ffmpeg.VideoProfile]int)

	sort.Sort(ffmpeg.ByName(p))
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

	return &BasicClaimManager{
		client:          c,
		ipfs:            ipfs,
		strmID:          sid,
		jobID:           jid,
		cost:            big.NewInt(0),
		totalSegCost:    new(big.Int).Mul(pricePerSegment, big.NewInt(int64(len(p)))),
		broadcasterAddr: broadcaster,
		pricePerSegment: pricePerSegment,
		profiles:        p,
		pLookup:         pLookup,
		segClaimMap:     make(map[int64]*claimData),
		unclaimedSegs:   make(map[int64]bool),
		claims:          0,
	}
}

func (c *BasicClaimManager) BroadcasterAddr() common.Address {
	return c.broadcasterAddr
}

func (c *BasicClaimManager) CanClaim() (bool, error) {
	// A transcoder can claim if:
	// - There are unclaimed segments
	// - If the on-chain job explicitly stores the transcoder's address OR the transcoder was assigned but did not make the first claim and it is within the first 230 blocks of the job's creation block
	if len(c.unclaimedSegs) == 0 {
		return false, nil
	}

	job, err := c.client.GetJob(c.jobID)
	if err != nil {
		return false, err
	}

	blknum, err := c.client.LatestBlockNum()
	if err != nil {
		return false, err
	}

	if job.TranscoderAddress == c.client.Account().Address || blknum.Cmp(new(big.Int).Add(job.CreationBlock, BlocksUntilFirstClaimDeadline)) != 1 {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *BasicClaimManager) DidFirstClaim() bool {
	return c.claims > 0
}

//AddReceipt adds a claim for a given video segment.
func (c *BasicClaimManager) AddReceipt(seqNo int64, bData []byte, bSig []byte,
	tData map[ffmpeg.VideoProfile][]byte) error {

	_, ok := c.segClaimMap[seqNo]
	if ok {
		return fmt.Errorf("Receipt for %v:%v already exists", c.jobID.String(), seqNo)
	}

	// ensure that all our profiles match up: check that lengths match
	if len(c.pLookup) != len(tData) {
		return fmt.Errorf("Mismatched profiles in segment; not claiming")
		// XXX record error in db
	}

	// ensure profiles match up, part 2: check for unknown profiles in the list
	hashes := make([][]byte, len(tData))
	for profile, td := range tData {
		i, ok := c.pLookup[profile]
		if !ok {
			return fmt.Errorf("cannot find profile: %v", profile)
			// XXX record error in db
		}
		hashes[i] = crypto.Keccak256(td) // set index based on profile ordering
	}
	tHash := crypto.Keccak256(hashes...)
	bHash := crypto.Keccak256(bData)

	cd := &claimData{
		seqNo:                seqNo,
		segData:              bData,
		dataHash:             bHash,
		bSig:                 bSig,
		claimConcatTDatahash: tHash,
	}

	c.cost = new(big.Int).Add(c.cost, c.totalSegCost)
	c.segClaimMap[seqNo] = cd
	c.unclaimedSegs[seqNo] = true
	// glog.Infof("Added %v. unclaimSegs: %v", seqNo, c.unclaimedSegs)

	return nil
}

func (c *BasicClaimManager) SufficientBroadcasterDeposit() (bool, error) {
	bDeposit, err := c.client.BroadcasterDeposit(c.broadcasterAddr)
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
	for key := range c.unclaimedSegs {
		keys = append(keys, key)
	}
	sort.Sort(SortUint64(keys))

	//Iterate through, check to make sure all tHashes are present (otherwise break and start new range),
	start := keys[0]
	ranges := make([][2]int64, 0)
	for i, _ := range keys {
		startNewRange := false

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

func (c *BasicClaimManager) markClaimedSegs(segRange [2]int64) {
	for segNo := segRange[0]; segNo <= segRange[1]; segNo++ {
		delete(c.unclaimedSegs, segNo)
	}
}

//Claim creates the onchain claim for all the claims added through AddReceipt
func (c *BasicClaimManager) ClaimVerifyAndDistributeFees() error {
	segs := make([]int64, 0)
	for k, _ := range c.unclaimedSegs {
		segs = append(segs, k)
	}
	ranges := c.makeRanges()
	glog.Infof("Claiming for segs: , ranges: %v", segs, ranges)

	for _, segRange := range ranges {
		//create concat hashes for each seg
		receiptHashes := make([]common.Hash, segRange[1]-segRange[0]+1)
		for i := segRange[0]; i <= segRange[1]; i++ {
			seg, _ := c.segClaimMap[i]

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
			continue
		}

		bigRange := [2]*big.Int{big.NewInt(segRange[0]), big.NewInt(segRange[1])}
		tx, err := c.client.ClaimWork(c.jobID, bigRange, root.Hash)
		if err != nil {
			return err
		}

		err = c.client.CheckTx(tx)
		if err != nil {
			return err
		}

		glog.Infof("Submitted transcode claim for segments %v - %v", segRange[0], segRange[1])

		c.markClaimedSegs(segRange)
		c.claims++

		claim, err := c.client.GetClaim(c.jobID, big.NewInt(c.claims-1))
		if err != nil {
			return err
		}

		//Record proofs for each segment in case the segment needs to be verified
		for i := segRange[0]; i <= segRange[1]; i++ {
			seg, _ := c.segClaimMap[i]
			seg.transcodeProof = proofs[i-segRange[0]].Bytes()
		}

		//Do the claim
		go func(segRange [2]int64, claim *ethTypes.Claim) {
			b, err := c.client.Backend()
			if err != nil {
				glog.Error(err)
				return
			}

			// Wait one block for claimBlock + 1 to be mined
			Wait(b, RpcTimeout, big.NewInt(1))

			plusOneBlk, err := b.BlockByNumber(context.Background(), new(big.Int).Add(claim.ClaimBlock, big.NewInt(1)))
			if err != nil {
				return
			}

			// Submit for verification if necessary
			c.verify(claim.ClaimId, claim.ClaimBlock.Int64(), plusOneBlk.Hash(), segRange)
			// Distribute fees once verification is complete
			c.distributeFees(claim.ClaimId)
		}(segRange, claim)
	}

	return nil
}

func (c *BasicClaimManager) verify(claimID *big.Int, claimBlkNum int64, plusOneBlkHash common.Hash, segRange [2]int64) error {
	//Get verification rate
	verifyRate, err := c.client.VerificationRate()
	if err != nil {
		glog.Errorf("Error getting verification rate: %v", err)
		return err
	}

	//Iterate through segments, determine which one needs to be verified.
	for segNo := segRange[0]; segNo <= segRange[1]; segNo++ {
		if c.shouldVerifySegment(segNo, segRange[0], segRange[1], claimBlkNum, plusOneBlkHash, verifyRate) {
			glog.Infof("Segment %v challenged for verification", segNo)

			seg := c.segClaimMap[segNo]

			// XXX load segment data from disk here
			dataStorageHash, err := c.ipfs.Add(bytes.NewReader(seg.segData))
			if err != nil {
				glog.Errorf("Error uploading segment data to IPFS: %v", err)
				continue
			}

			dataHashes := [2][32]byte{common.BytesToHash(seg.dataHash), common.BytesToHash(seg.claimConcatTDatahash)}

			tx, err := c.client.Verify(c.jobID, claimID, big.NewInt(segNo), dataStorageHash, dataHashes, seg.bSig, seg.transcodeProof)
			if err != nil {
				glog.Errorf("Error submitting segment %v for verification: %v", segNo, err)
				continue
			}

			err = c.client.CheckTx(tx)
			if err != nil {
				glog.Errorf("Failed to verify segment %v: %v", segNo, err)
				continue
			}

			glog.Infof("Verified segment %v", segNo)
		}
	}

	return nil
}

func (c *BasicClaimManager) distributeFees(claimID *big.Int) error {
	verificationPeriod, err := c.client.VerificationPeriod()
	if err != nil {
		return err
	}

	slashingPeriod, err := c.client.VerificationSlashingPeriod()
	if err != nil {
		return err
	}

	b, err := c.client.Backend()
	if err != nil {
		return err
	}

	Wait(b, RpcTimeout, new(big.Int).Add(verificationPeriod, slashingPeriod))

	tx, err := c.client.DistributeFees(c.jobID, claimID)
	if err != nil {
		return err
	}

	err = c.client.CheckTx(tx)
	if err != nil {
		return err
	}

	glog.Infof("Distributed fees for job %v claim %v", c.jobID, claimID)

	return nil
}

func (c *BasicClaimManager) shouldVerifySegment(seqNum int64, start int64, end int64, blkNum int64, plusOneBlkHash common.Hash, verifyRate uint64) bool {
	if seqNum < start || seqNum > end {
		return false
	}

	bigSeqNumBytes := common.LeftPadBytes(new(big.Int).SetInt64(seqNum).Bytes(), 32)
	bigBlkNumBytes := common.LeftPadBytes(new(big.Int).SetInt64(blkNum+1).Bytes(), 32)

	combH := crypto.Keccak256(bigBlkNumBytes, plusOneBlkHash.Bytes(), bigSeqNumBytes)
	hashNum := new(big.Int).SetBytes(combH)
	result := new(big.Int).Mod(hashNum, new(big.Int).SetInt64(int64(verifyRate)))

	if result.Cmp(new(big.Int).SetInt64(int64(0))) == 0 {
		return true
	} else {
		return false
	}
}
