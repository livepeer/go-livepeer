package eth

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var (
	RpcTimeout = 10 * time.Second
)

type ClaimManager interface {
	AddReceipt(seqNo int64, bData []byte, bSig []byte, tData map[ffmpeg.VideoProfile][]byte, tStart time.Time, tEnd time.Time) error
	SufficientBroadcasterDeposit() (bool, error)
	ClaimVerifyAndDistributeFees() error
	CanClaim() (bool, error)
	DidFirstClaim() bool
	BroadcasterAddr() ethcommon.Address
}

type claimData struct {
	seqNo                int64
	segData              []byte
	dataHash             []byte
	bSig                 []byte
	claimConcatTDatahash []byte
	transcodeProof       []byte
}

//BasicClaimManager manages the claim process for a Livepeer transcoder.  Check the Livepeer protocol for more details.
type BasicClaimManager struct {
	client LivepeerEthClient
	db     *common.DB
	ipfs   ipfs.IpfsApi

	strmID   string
	jobID    *big.Int
	profiles []ffmpeg.VideoProfile
	pLookup  map[ffmpeg.VideoProfile]int

	segClaimMap   map[int64]*claimData
	unclaimedSegs map[int64]bool
	cost          *big.Int

	broadcasterAddr ethcommon.Address
	pricePerSegment *big.Int
	totalSegCost    *big.Int

	claims     int64
	claimsLock sync.Mutex
}

//NewBasicClaimManager creates a new claim manager.
func NewBasicClaimManager(job *ethTypes.Job, c LivepeerEthClient, ipfs ipfs.IpfsApi, db *common.DB) *BasicClaimManager {
	p := job.Profiles
	seqNos := make([][]int64, len(p), len(p))
	rHashes := make([][]ethcommon.Hash, len(p), len(p))
	sd := make([][][]byte, len(p), len(p))
	dHashes := make([][]string, len(p), len(p))
	tHashes := make([][]string, len(p), len(p))
	sigs := make([][][]byte, len(p), len(p))
	pLookup := make(map[ffmpeg.VideoProfile]int)

	sort.Sort(ffmpeg.ByName(p))
	for i := 0; i < len(p); i++ {
		sNo := make([]int64, 0)
		seqNos[i] = sNo
		rh := make([]ethcommon.Hash, 0)
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
		db:              db,
		ipfs:            ipfs,
		strmID:          job.StreamId,
		jobID:           job.JobId,
		cost:            big.NewInt(0),
		totalSegCost:    new(big.Int).Mul(job.MaxPricePerSegment, big.NewInt(int64(len(p)))),
		broadcasterAddr: job.BroadcasterAddress,
		pricePerSegment: job.MaxPricePerSegment,
		profiles:        p,
		pLookup:         pLookup,
		segClaimMap:     make(map[int64]*claimData),
		unclaimedSegs:   make(map[int64]bool),
		claims:          job.TotalClaims.Int64(),
	}
}

func RecoverClaims(c LivepeerEthClient, ipfs ipfs.IpfsApi, db *common.DB) error {
	// XXX While this will recover claims for jobs that haven't submitted a
	// claim yet, it doesn't attempt to recover if the node restarts mid-process
	// eg, between the claim, verify and distributeFees calls.
	glog.V(common.DEBUG).Info("Initialized DB node")
	jobReceipts, err := db.UnclaimedReceipts()
	if err != nil {
		return err
	}
	for jid, receipts := range jobReceipts {
		glog.V(common.DEBUG).Info("claimmanager: Fetching claims for job ", jid)
		j, err := c.GetJob(big.NewInt(jid)) // benchmark; may be faster to reconstruct locally?
		if err != nil {
			glog.Error("Unable to get job ", jid, err)
			continue
		}
		cm := NewBasicClaimManager(j, c, ipfs, db)
		for _, r := range receipts {
			cm.unclaimedSegs[r.SeqNo] = true
			cm.segClaimMap[r.SeqNo] = &claimData{
				seqNo:                r.SeqNo,
				segData:              []byte{},
				dataHash:             r.BcastHash,
				bSig:                 r.BcastSig,
				claimConcatTDatahash: r.TcodeHash,
			}
		}
		go func() {
			glog.V(common.DEBUG).Info("claimmanager: Starting recovery for ", jid)
			err := cm.ClaimVerifyAndDistributeFees()
			if err != nil {
				glog.Error("Unable to claim/verify/distribute: ", err)
			}
		}()
	}
	return nil
}

func (c *BasicClaimManager) BroadcasterAddr() ethcommon.Address {
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
	tData map[ffmpeg.VideoProfile][]byte, tStart time.Time, tEnd time.Time) error {

	_, ok := c.segClaimMap[seqNo]
	if ok {
		return fmt.Errorf("Receipt for %v:%v already exists", c.jobID.String(), seqNo)
	}

	// ensure that all our profiles match up: check that lengths match
	if len(c.pLookup) != len(tData) {
		return fmt.Errorf("Job %v Mismatched profiles in segment; not claiming", c.jobID)
		// XXX record error in db
	}

	// ensure profiles match up, part 2: check for unknown profiles in the list
	hashes := make([][]byte, len(tData))
	for profile, td := range tData {
		i, ok := c.pLookup[profile]
		if !ok {
			return fmt.Errorf("Job %v cannot find profile: %v", c.jobID, profile)
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

	c.db.InsertReceipt(c.jobID, seqNo, bHash, bSig, tHash, tStart, tEnd)
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

func (c *BasicClaimManager) setClaimStatus(id int64, status string) {
	if c.db != nil {
		c.db.SetClaimStatus(c.jobID, id, status)
	}
}

//Claim creates the onchain claim for all the claims added through AddReceipt
func (c *BasicClaimManager) ClaimVerifyAndDistributeFees() error {
	segs := make([]int64, 0)
	for k, _ := range c.unclaimedSegs {
		segs = append(segs, k)
	}
	ranges := c.makeRanges()
	glog.V(common.SHORT).Infof("Job %v Claiming for segs: %v ranges: %v", c.jobID, segs, ranges)

	for _, segRange := range ranges {
		//create concat hashes for each seg
		receiptHashes := make([]ethcommon.Hash, segRange[1]-segRange[0]+1)
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
			glog.Errorf("Job %v Error: %v - creating merkle root for %v", c.jobID, err, receiptHashes)
			continue
		}

		// Preemptively guess at the claim ID and confirm after the tx succeeds.
		// If the ID check fails, we likely had concurrent claims in-flight.
		claimID := c.claims
		if c.db != nil {
			// check db in case we had any concurrent claims
			claimIDp, _ := c.db.InsertClaim(c.jobID, segRange, root.Hash)
			claimID = *claimIDp
		}

		bigRange := [2]*big.Int{big.NewInt(segRange[0]), big.NewInt(segRange[1])}
		tx, err := c.client.ClaimWork(c.jobID, bigRange, root.Hash)
		if err != nil {
			glog.Errorf("Job %v Could not claim work - error %v", c.jobID, err)
			c.setClaimStatus(claimID, "FAIL submit")
			return err
		}

		err = c.client.CheckTx(tx)
		if err != nil {
			glog.Errorf("Job %v tx failed %v", c.jobID, tx)
			c.setClaimStatus(claimID, "FAIL check tx")
			return err
		}

		glog.V(common.SHORT).Infof("Job %v Submitted transcode claim for segments %v - %v", c.jobID, segRange[0], segRange[1])

		c.markClaimedSegs(segRange)
		c.claims = claimID + 1
		c.setClaimStatus(claimID, "Submitted")

		claim, err := c.client.GetClaim(c.jobID, big.NewInt(claimID))
		if err != nil || claim == nil {
			glog.Errorf("Could not get claim %v: %v", claimID, err)
			c.setClaimStatus(claimID, "FAIL get claim")
			return err
		}

		// Confirm claim ranges and root matches the estimated ID
		if segRange[0] != claim.SegmentRange[0].Int64() ||
			segRange[1] != claim.SegmentRange[1].Int64() ||
			root.Hash != claim.ClaimRoot {
			err = fmt.Errorf("Job %v claim %v does not match! Expected segments %v ; got %v, expected root %v got %v", c.jobID, claimID, segRange, claim.SegmentRange, root.Hash, claim.ClaimRoot)
			glog.Error(err.Error())
			// XXX fix; maybe the user can manually retry the tx for now.
			c.setClaimStatus(claimID, "FAIL id mismatch")
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
				c.setClaimStatus(claimID, "FAIL Unable to get backend: "+err.Error())
				return
			}

			// Wait one block for claimBlock + 1 to be mined
			Wait(b, RpcTimeout, big.NewInt(1))

			plusOneBlk, err := b.BlockByNumber(context.Background(), new(big.Int).Add(claim.ClaimBlock, big.NewInt(1)))
			if err != nil {
				c.setClaimStatus(claimID, "FAIL waiting for block :"+err.Error())
				return
			}

			// Submit for verification if necessary
			c.verify(claim.ClaimId, claim.ClaimBlock.Int64(), plusOneBlk.Hash(), segRange)
			// Distribute fees once verification is complete
			c.distributeFees(claimID)
		}(segRange, claim)
	}

	return nil
}

func (c *BasicClaimManager) verify(claimID *big.Int, claimBlkNum int64, plusOneBlkHash ethcommon.Hash, segRange [2]int64) error {
	//Get verification rate
	verifyRate, err := c.client.VerificationRate()
	if err != nil {
		glog.Errorf("Job %v Error getting verification rate: %v", c.jobID, err)
		return err
	}

	//Iterate through segments, determine which one needs to be verified.
	for segNo := segRange[0]; segNo <= segRange[1]; segNo++ {
		if c.shouldVerifySegment(segNo, segRange[0], segRange[1], claimBlkNum, plusOneBlkHash, verifyRate) {
			glog.V(common.SHORT).Infof("Job %v Segment %v challenged for verification", c.jobID, segNo)

			seg := c.segClaimMap[segNo]

			// XXX load segment data from disk here
			dataStorageHash, err := c.ipfs.Add(bytes.NewReader(seg.segData))
			if err != nil {
				glog.Errorf("Job %v Error uploading segment data to IPFS: %v", c.jobID, err)
				continue
			}

			dataHashes := [2][32]byte{ethcommon.BytesToHash(seg.dataHash), ethcommon.BytesToHash(seg.claimConcatTDatahash)}

			tx, err := c.client.Verify(c.jobID, claimID, big.NewInt(segNo), dataStorageHash, dataHashes, seg.bSig, seg.transcodeProof)
			if err != nil {
				glog.Errorf("Job %v Error submitting segment %v for verification: %v", c.jobID, segNo, err)
				continue
			}

			err = c.client.CheckTx(tx)
			if err != nil {
				glog.Errorf("Job %v Failed to verify segment %v: %v", c.jobID, segNo, err)
				continue
			}

			glog.V(common.SHORT).Infof("Job %v Verified segment %v", c.jobID, segNo)
		}
	}

	return nil
}

func (c *BasicClaimManager) distributeFees(claimID int64) error {
	verificationPeriod, err := c.client.VerificationPeriod()
	if err != nil {
		c.setClaimStatus(claimID, "FAIL Verification period "+err.Error())
		return err
	}

	slashingPeriod, err := c.client.VerificationSlashingPeriod()
	if err != nil {
		c.setClaimStatus(claimID, "FAIL Slashing period "+err.Error())
		return err
	}

	b, err := c.client.Backend()
	if err != nil {
		c.setClaimStatus(claimID, "FAIL Distribute backend "+err.Error())
		return err
	}

	Wait(b, RpcTimeout, new(big.Int).Add(verificationPeriod, slashingPeriod))

	tx, err := c.client.DistributeFees(c.jobID, big.NewInt(claimID))
	if err != nil {
		c.setClaimStatus(claimID, "FAIL Submit distribute "+err.Error())
		return err
	}

	err = c.client.CheckTx(tx)
	if err != nil {
		c.setClaimStatus(claimID, "FAIL Check distribute txn "+err.Error())
		return err
	}

	glog.V(common.SHORT).Infof("Distributed fees for job %v claim %v", c.jobID, claimID)
	c.setClaimStatus(claimID, "Complete")

	return nil
}

func (c *BasicClaimManager) shouldVerifySegment(seqNum int64, start int64, end int64, blkNum int64, plusOneBlkHash ethcommon.Hash, verifyRate uint64) bool {
	if seqNum < start || seqNum > end {
		return false
	}

	bigSeqNumBytes := ethcommon.LeftPadBytes(new(big.Int).SetInt64(seqNum).Bytes(), 32)
	bigBlkNumBytes := ethcommon.LeftPadBytes(new(big.Int).SetInt64(blkNum+1).Bytes(), 32)

	combH := crypto.Keccak256(bigBlkNumBytes, plusOneBlkHash.Bytes(), bigSeqNumBytes)
	hashNum := new(big.Int).SetBytes(combH)
	result := new(big.Int).Mod(hashNum, new(big.Int).SetInt64(int64(verifyRate)))

	if result.Cmp(new(big.Int).SetInt64(int64(0))) == 0 {
		return true
	} else {
		return false
	}
}
