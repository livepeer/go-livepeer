package core

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/livepeer/golp/eth"
	ethTypes "github.com/livepeer/golp/eth/types"
	"github.com/livepeer/golp/net"
	"github.com/livepeer/lpms/stream"
	lptr "github.com/livepeer/lpms/transcoder"
)

var ErrTranscode = errors.New("ErrTranscode")
var ErrBroadcastTimeout = errors.New("ErrBroadcastTimeout")
var ErrBroadcastJob = errors.New("ErrBroadcastJob")
var ErrEOF = errors.New("ErrEOF")
var BroadcastTimeout = time.Second * 30
var ClaimInterval = int64(5)
var EthRpcTimeout = 5 * time.Second
var EthEventTimeout = 5 * time.Second
var EthMinedTxTimeout = 60 * time.Second
var VerifyRate = int64(1)

//NodeID can be converted from libp2p PeerID.
type NodeID string

type LivepeerNode struct {
	Identity     NodeID
	VideoNetwork net.VideoNetwork
	StreamDB     *StreamDB
	Eth          eth.LivepeerEthClient
	EthPassword  string
}

func NewLivepeerNode(port int, priv crypto.PrivKey, pub crypto.PubKey, e eth.LivepeerEthClient) (*LivepeerNode, error) {
	n, err := net.NewBasicNetwork(port, priv, pub)
	if err != nil {
		glog.Errorf("Cannot create network node: %v", err)
		return nil, err
	}
	return &LivepeerNode{StreamDB: NewStreamDB(peer.IDHexEncode(n.NetworkNode.Identity)), VideoNetwork: n, Identity: NodeID(peer.IDHexEncode(n.NetworkNode.Identity)), Eth: e}, nil
}

func (n *LivepeerNode) Start(bootID, bootAddr string) error {
	//Set up protocol (to handle incoming streams)
	if err := n.VideoNetwork.SetupProtocol(); err != nil {
		glog.Errorf("Error setting up protocol: %v", err)
		return err
	}

	//Connect to bootstrap node.  This currently also kicks off a bootstrap process, which periodically checks for new peers and connect to them.
	if err := n.VideoNetwork.Connect(bootID, bootAddr); err != nil {
		glog.Errorf("Cannot connect to node: %v", err)
		return err
	}

	//TODO:Kick off process to periodically monitor peer connection by pinging them

	return nil
}

//CreateTranscodeJob creates the onchain transcode job
//This can only be done by a broadcaster
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []net.VideoProfile, price uint64) error {
	//Verify the stream exists(assume it's a local stream)
	buf := n.StreamDB.GetHLSBuffer(strmID)
	if buf == nil {
		glog.Errorf("Cannot find stream %v for creating transcode job", strmID)
		return ErrNotFound
	}

	//Call eth client to create the job
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return ErrNotFound
	}

	tOpt := [32]byte{}
	p := big.NewInt(int64(price))
	count := 0
	for _, p := range profiles {
		count += copy(tOpt[count:], p.Name)
		if count > 32 {
			glog.Errorf("Too many profiles.  Names can best at most 32 bytes")
			return fmt.Errorf("Transcode Job Error")
		}
	}
	tx, err := n.Eth.Job(strmID.String(), tOpt, p)
	if err != nil || tx == nil {
		glog.Errorf("Error creating transcode job: %v", err)
		return err
	}

	glog.Infof("Created transcode job: %v", tx)
	return nil
}

func (n *LivepeerNode) CallReward() {
	if err := eth.CheckRoundAndInit(n.Eth, EthRpcTimeout, EthMinedTxTimeout); err != nil {
		glog.Errorf("%v", err)
		return
	}

	valid, err := n.Eth.ValidRewardTimeWindow()
	if err != nil {
		glog.Errorf("Error getting reward time window info: %v", err)
		return
	}

	if valid {
		glog.Infof("It's our window. Calling reward()")
		tx, err := n.Eth.Reward()
		if err != nil || tx == nil {
			glog.Errorf("Error calling reward: %v", err)
			return
		}
		r, err := eth.WaitForMinedTx(n.Eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
		if err != nil {
			glog.Errorf("Error waiting for mined tx: %v", err)
		}
		if tx.Gas().Cmp(r.GasUsed) == 0 {
			glog.Errorf("Call Reward Failed")
			return
		}
	} else {
		// glog.Infof("Not valid reward time window.")
	}
}

//Transcode transcodes one stream into multiple stream, and returns a list of StreamIDs, in the order of the video profiles.
func (n *LivepeerNode) Transcode(config net.TranscodeConfig) ([]StreamID, error) {
	s, err := n.VideoNetwork.GetSubscriber(config.StrmID)
	if err != nil {
		glog.Errorf("Error getting subscriber from network: %v", err)
	}

	transcoders := make(map[string]*lptr.FFMpegSegmentTranscoder)
	broadcasters := make(map[string]net.Broadcaster)
	ids := make(map[string]StreamID)
	resultStrmIDs := make([]StreamID, len(config.Profiles), len(config.Profiles))

	//Create broadcasters based on transcode video profiles
	for i, p := range config.Profiles {
		transcoders[p.Name] = lptr.NewFFMpegSegmentTranscoder(p.Bitrate, p.Framerate, p.Resolution, "", "./tmp")
		strmID, err := MakeStreamID(n.Identity, RandomVideoID(), p.Name)
		if err != nil {
			glog.Errorf("Error making stream ID")
			return nil, ErrTranscode
		}
		b, err := n.VideoNetwork.GetBroadcaster(strmID.String())
		if err != nil {
			glog.Errorf("Error creating broadcaster: %v", err)
			return nil, ErrTranscode
		}
		broadcasters[p.Name] = b
		ids[p.Name] = strmID
		resultStrmIDs[i] = strmID
	}

	tcHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
	dHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
	tHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
	sigs := make([][][]byte, len(config.Profiles), len(config.Profiles))
	for i := 0; i < len(config.Profiles); i++ {
		tch := make([]common.Hash, ClaimInterval, ClaimInterval)
		tcHashes[i] = tch
		dh := make([]common.Hash, ClaimInterval, ClaimInterval)
		dHashes[i] = dh
		th := make([]common.Hash, ClaimInterval, ClaimInterval)
		tHashes[i] = th
		s := make([][]byte, ClaimInterval, ClaimInterval)
		sigs[i] = s
	}

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	var startSeq, endSeq int64
	s.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if eof {
			glog.Infof("Stream finished")
		}

		//Decode the segment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		for pi, p := range config.Profiles {
			t := transcoders[p.Name]
			td, err := t.Transcode(ss.Seg.Data)
			if err != nil {
				glog.Errorf("Error transcoding for %v: %v", p.Name, err)
				continue
			}

			//Encode the transcoded segment into bytes
			b := broadcasters[p.Name]
			newSeg := stream.HLSSegment{SeqNo: seqNo, Name: fmt.Sprintf("%s_%d.ts", ids[p.Name], seqNo), Data: td, Duration: ss.Seg.Duration}
			newSegb, err := SignedSegmentToBytes(SignedSegment{Seg: newSeg, Sig: nil}) //We don't need to sign the transcoded streams now
			if err != nil {
				glog.Errorf("Error encoding segment to []byte: %v", err)
				continue
			}

			//Broadcast the transcoded segment
			err = b.Broadcast(seqNo, newSegb)
			if err != nil {
				glog.Errorf("Error broadcasting segment to network: %v", err)
			}

			//Don't do the onchain stuff unless we want to
			if !config.PerformOnchainClaim {
				continue
			}

			//Record the segment hashes, so we can call claim
			claim := &ethTypes.TranscodeClaim{
				StreamID:              config.StrmID,
				SegmentSequenceNumber: big.NewInt(int64(seqNo)),
				DataHash:              common.BytesToHash(ss.Seg.Data),
				TranscodedDataHash:    common.BytesToHash(td),
				BroadcasterSig:        ss.Sig,
			}

			tcHashes[pi][int64(seqNo)%ClaimInterval] = claim.Hash()
			dHashes[pi][int64(seqNo)%ClaimInterval] = common.BytesToHash(ss.Seg.Data)
			tHashes[pi][int64(seqNo)%ClaimInterval] = common.BytesToHash(td)
			sigs[pi][int64(seqNo)%ClaimInterval] = ss.Sig

			if int64(seqNo)%ClaimInterval == ClaimInterval-1 {
				endSeq = startSeq + ClaimInterval - 1
				go claimAndVerify(config.JobID, dHashes[pi], tcHashes[pi], tHashes[pi], sigs[pi], startSeq, endSeq, n.Eth)
				startSeq = endSeq + 1

				//Claimed that part of the work.  Now refresh all the containers.
				tcHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
				dHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
				tHashes := make([][]common.Hash, len(config.Profiles), len(config.Profiles))
				sigs := make([][][]byte, len(config.Profiles), len(config.Profiles))
				for i := 0; i < len(config.Profiles); i++ {
					tch := make([]common.Hash, ClaimInterval, ClaimInterval)
					tcHashes[i] = tch
					dh := make([]common.Hash, ClaimInterval, ClaimInterval)
					dHashes[i] = dh
					th := make([]common.Hash, ClaimInterval, ClaimInterval)
					tHashes[i] = th
					s := make([][]byte, ClaimInterval, ClaimInterval)
					sigs[i] = s
				}
			}
		}
	})

	return resultStrmIDs, nil
}

func claimAndVerify(jid *big.Int, dHashes []common.Hash, tcHashes []common.Hash, thashes []common.Hash, sigs [][]byte, start, end int64, c eth.LivepeerEthClient) {
	if end-start+1 != int64(len(dHashes)) {
		glog.Errorf("Start(%v) and End(%v) doesn't match up with hash length: %v", start, end, len(dHashes))
		return
	}

	ranges := make([][2]int64, 0)
	for i := 0; i < len(dHashes); i++ {
		empty := common.Hash{}
		if dHashes[i] != empty {
			st := i
			for ed := i + 1; ed < len(dHashes); ed++ {
				if dHashes[ed] == empty {
					ranges = append(ranges, [2]int64{int64(st), int64(ed - 1)})
					i = ed + 1
					break
				}

				if ed == len(dHashes)-1 {
					ranges = append(ranges, [2]int64{int64(st), int64(ed)})
					i = ed
					break
				}
			}
		}
	}

	// glog.Infof("ranges: %v", ranges)

	for _, r := range ranges {
		st := r[0]
		ed := r[1]
		root, proofs, err := ethTypes.NewMerkleTree(tcHashes[st : ed+1])
		if err != nil {
			glog.Errorf("Error: %v - creating merkle root for: %v", err, tcHashes[st:ed+1])
			//TODO: If this happens, should we cancel the job?
		}

		tx, err := c.ClaimWork(jid, big.NewInt(start+st), big.NewInt(start+ed), root.Hash)
		if err != nil {
			glog.Errorf("Error claiming work: %v", err)
		} else {
			verify(jid, dHashes[st:ed+1], thashes[st:ed+1], sigs[st:ed+1], proofs, start+st, start+ed, c, tx.Hash())
		}
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

	num, i := binary.Uvarint(ethCrypto.Keccak256(blkNumB, blkHash.Bytes(), seqNumB))
	if i != 0 {
		glog.Errorf("Error converting bytes in shouldVerifySegment.  num: %v, i: %v.  blkNumB:%x, blkHash:%x, seqNumB:%x", num, i, blkNumB, blkHash.Bytes(), seqNumB)
	}
	if num%uint64(VerifyRate) == 0 {
		return true
	} else {
		return false
	}
}

//BroadcastToNetwork is called when a new broadcast stream is available.  It lets the network decide how
//to deal with the stream.
func (n *LivepeerNode) BroadcastToNetwork(strm *stream.VideoStream) error {
	b, err := n.VideoNetwork.GetBroadcaster(strm.GetStreamID())
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	//TODO: Prepare the broadcast (for example, for Adaptive Bitrate Streaming, we have to send the MasterPlaylist)

	//Broadcast stream to network
	counter := uint64(0)
	lastSuccess := time.Now()
	for {
		if time.Since(lastSuccess) > BroadcastTimeout {
			glog.Errorf("Broadcast Timeout")
			return ErrBroadcastTimeout
		}

		//Read segment
		seg, err := strm.ReadHLSSegment()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if seg.EOF == true {
			glog.Info("Got EOF for HLS Broadcast, Terminating Broadcast.")
			return ErrEOF
		}

		//Get segment signature
		segHash := (&ethTypes.Segment{StreamID: strm.GetStreamID(), SegmentSequenceNumber: big.NewInt(int64(counter)), DataHash: common.BytesToHash(seg.Data)}).Hash()
		var sig []byte
		if c, ok := n.Eth.(*eth.Client); ok {
			sig, err = eth.SignSegmentHash(c, n.EthPassword, segHash.Bytes())
			if err != nil {
				glog.Errorf("Error signing segment %v-%v: %v", strm.GetStreamID(), counter, err)
				continue
			}
		}

		//Encode segment into []byte, broadcast it
		if ssb, err := SignedSegmentToBytes(SignedSegment{Seg: seg, Sig: sig}); err == nil {
			if err = b.Broadcast(counter, ssb); err != nil {
				glog.Errorf("Error broadcasting segment to network: %v", err)
			}
		} else {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			continue
		}

		lastSuccess = time.Now()
		counter++
	}
}

//SubscribeFromNetwork subscribes to a stream on the network.  Returns the stream as a reference.
func (n *LivepeerNode) SubscribeFromNetwork(ctx context.Context, strmID StreamID) (*stream.VideoStream, error) {
	s, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber from network: %v", err)
	}

	//Create a new video stream
	strm := n.StreamDB.GetStream(strmID)
	if strm != nil {
		strm, err = n.StreamDB.AddNewStream(strmID, stream.HLS)
		if err != nil {
			glog.Errorf("Error creating stream when subscribing: %v", err)
		}
	}
	err = s.Subscribe(ctx, func(seqNo uint64, data []byte, eof bool) {
		if eof {
			//TODO: Remove stream, remove subscriber.
			n.StreamDB.UnsubscribeToHLSStream(strmID.String(), "local")
			n.StreamDB.DeleteHLSBuffer(strmID)
			n.StreamDB.DeleteStream(strmID)

			// n.VideoNetwork.DeleteSubscriber(strmID.String())
			return
		}

		//TOOD: Check for segNo, make sure it's not out of order

		//Decode data into SignedSegment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		//Add segment into stream
		if err := strm.WriteHLSSegmentToStream(ss.Seg); err != nil {
			glog.Errorf("Error writing HLS Segment: %v", err)
		}
	})
	if err != nil {
		glog.Errorf("Error subscribing from network: %v", err)
		return nil, err
	}
	return strm, nil
}

//UnsubscribeFromNetwork unsubscribes to a stream on the network.
func (n *LivepeerNode) UnsubscribeFromNetwork(strmID StreamID) error {
	s, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber when unsubscribing from network: %v", err)
		return ErrNotFound
	}

	err = s.Unsubscribe()
	if err != nil {
		glog.Errorf("Error unsubscribing from network: %v", err)
		return err
	}

	return nil
}

func (n *LivepeerNode) NotifyBroadcaster(nid NodeID, strmID StreamID, transcodeStrmIDs map[StreamID]net.VideoProfile) error {
	ids := make(map[string]string)
	for sid, p := range transcodeStrmIDs {
		ids[sid.String()] = p.Name
	}
	return n.VideoNetwork.SendTranscodResult(string(nid), strmID.String(), ids)
}
