/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path"
	"sort"
	"time"

	"github.com/ericxtang/m3u8"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var ErrBroadcastTimeout = errors.New("ErrBroadcastTimeout")
var ErrBroadcastJob = errors.New("ErrBroadcastJob")
var ErrBroadcast = errors.New("ErrBroadcast")
var ErrEOF = errors.New("ErrEOF")
var ErrNotFound = errors.New("ErrNotFound")
var BroadcastTimeout = time.Second * 30
var EthRpcTimeout = 5 * time.Second
var EthEventTimeout = 5 * time.Second
var EthMinedTxTimeout = 60 * time.Second
var DefaultMasterPlaylistWaitTime = 60 * time.Second
var DefaultJobLength = int64(5760) //Avg 1 day in 15 sec blocks
var ConnFileWriteFreq = time.Duration(60) * time.Second
var LivepeerVersion = "0.1.14-unstable"

//NodeID can be converted from libp2p PeerID.
type NodeID string

type PeerConn struct {
	NodeID   string
	NodeAddr string
}

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {
	Identity        NodeID
	Addrs           []string
	VideoNetwork    net.VideoNetwork
	VideoCache      VideoCache
	Eth             eth.LivepeerEthClient
	EthEventMonitor eth.EventMonitor
	EthServices     []eth.EventService
	Ipfs            ipfs.IpfsApi
	WorkDir         string
	PeerConns       []PeerConn
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork, nodeId NodeID, addrs []string, wd string) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}

	return &LivepeerNode{VideoCache: NewBasicVideoCache(vn), VideoNetwork: vn, Identity: nodeId, Addrs: addrs, Eth: e, WorkDir: wd, PeerConns: make([]PeerConn, 0)}, nil
}

//Start sets up the Livepeer protocol and connects the node to the network
func (n *LivepeerNode) Start(ctx context.Context, bootID, bootAddr string) error {
	//Set up protocol (to handle incoming streams)
	if err := n.VideoNetwork.SetupProtocol(); err != nil {
		glog.Errorf("Error setting up protocol: %v", err)
		return err
	}

	//Connect to bootstrap node.  This currently also kicks off a bootstrap process, which periodically checks for new peers and connect to them.
	if bootID != "" && bootAddr != "" {
		glog.Infof("Connecting to %v %v", bootID, bootAddr)
		if err := n.VideoNetwork.Connect(bootID, []string{bootAddr}); err != nil {
			glog.Errorf("Cannot connect to node: %v", err)
		} else {
			n.PeerConns = append(n.PeerConns, PeerConn{NodeID: bootID, NodeAddr: bootAddr})
		}
	}

	//TODO:Kick off process to periodically monitor peer connection by pinging them
	return nil
}

//CreateTranscodeJob creates the on-chain transcode job.
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []ffmpeg.VideoProfile, price *big.Int) error {
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return ErrNotFound
	}

	//Sort profiles first
	sort.Sort(ffmpeg.ByName(profiles))
	transOpts := []byte{}
	for _, prof := range profiles {
		transOpts = append(transOpts, crypto.Keccak256([]byte(prof.Name))[0:4]...)
	}

	//Call eth client to create the job
	b, err := n.Eth.Backend()
	if err != nil {
		return err
	}

	blk, err := b.BlockByNumber(context.Background(), nil)
	if err != nil {
		glog.Errorf("Cannot get current block number: %v", err)
		return ErrNotFound
	}

	tx, err := n.Eth.Job(strmID.String(), ethcommon.ToHex(transOpts)[2:], price, big.NewInt(0).Add(blk.Number(), big.NewInt(DefaultJobLength)))
	if err != nil {
		glog.Errorf("Error creating transcode job: %v", err)
		return err
	}

	err = n.Eth.CheckTx(tx)
	if err != nil {
		return err
	}

	glog.Infof("Created broadcast job. Price: %v. Type: %v", price, ethcommon.ToHex(transOpts)[2:])

	return nil
}

//TranscodeAndBroadcast transcodes one stream into multiple streams (specified by TranscodeConfig), broadcasts the streams, and returns a list of streamIDs.
func (n *LivepeerNode) TranscodeAndBroadcast(config net.TranscodeConfig, cm eth.ClaimManager, t transcoder.Transcoder) ([]StreamID, error) {
	//Create the broadcasters
	tProfiles := make([]ffmpeg.VideoProfile, len(config.Profiles), len(config.Profiles))
	resultStrmIDs := make([]StreamID, len(config.Profiles), len(config.Profiles))
	broadcasters := make(map[StreamID]stream.Broadcaster)
	sid := StreamID(config.StrmID)
	for i, vp := range config.Profiles {
		strmID, err := MakeStreamID(n.Identity, sid.GetVideoID(), vp.Name)
		if err != nil {
			glog.Errorf("Error making stream ID: %v", err)
			return nil, ErrTranscode
		}
		resultStrmIDs[i] = strmID
		tProfiles[i] = ffmpeg.VideoProfileLookup[vp.Name]

		broadcaster, err := n.VideoNetwork.GetBroadcaster(string(strmID))
		if err != nil {
			glog.Errorf("Error making new stream: %v", err)
			return nil, ErrTranscode
		}
		broadcasters[strmID] = broadcaster
	}

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	sub, err := n.VideoCache.GetHLSSubscriber(StreamID(config.StrmID))
	if err != nil {
		glog.Errorf("Error getting subscriber for stream %v from network: %v", config.StrmID, err)
	}
	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		glog.V(common.DEBUG).Infof("Starting to transcode segment %v", seqNo)
		totalStart := time.Now()
		if eof {
			if cm != nil && config.PerformOnchainClaim {
				glog.V(common.SHORT).Infof("Stream finished. Claiming work.")

				canClaim, err := cm.CanClaim()
				if err != nil {
					glog.Error(err)
				}

				if canClaim {
					if err := cm.ClaimVerifyAndDistributeFees(); err != nil {
						glog.Errorf("Error claiming work: %v", err)
					}
				} else {
					glog.Infof("No segments to claim")
				}
			}
			return
		}

		if cm != nil && config.PerformOnchainClaim {
			sufficient, err := cm.SufficientBroadcasterDeposit()
			if err != nil {
				glog.Errorf("Error checking broadcaster funds: %v", err)
			}

			if !sufficient {
				glog.V(common.SHORT).Infof("Broadcaster does not have enough funds. Claiming work.")

				canClaim, err := cm.CanClaim()
				if err != nil {
					glog.Error(err)
				}

				if canClaim {
					if err := cm.ClaimVerifyAndDistributeFees(); err != nil {
						glog.Errorf("Error claiming work: %v", err)
					}
				} else {
					glog.Infof("No segments to claim")
				}
				return
			}
		}

		//Decode the segment
		start := time.Now()
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}
		glog.V(common.DEBUG).Infof("Decoding of segment took %v", time.Since(start))

		//If running in on-chain mode, check that segment was signed by broadcaster ETH address
		segHash := (&ethTypes.Segment{StreamID: config.StrmID, SegmentSequenceNumber: big.NewInt(int64(seqNo)), DataHash: crypto.Keccak256Hash(ss.Seg.Data)}).Hash()
		if cm == nil || (cm.BroadcasterAddr() == ethcommon.Address{}) || eth.VerifySig(cm.BroadcasterAddr(), segHash.Bytes(), ss.Sig) {
			glog.V(common.DEBUG).Infof("Verified segment received from stream broadcaster")

			n.transcodeAndBroadcastSeg(&ss.Seg, ss.Sig, cm, t, resultStrmIDs, broadcasters, config)
			glog.V(common.DEBUG).Infof("Encoding and broadcasting of segment %v took %v", ss.Seg.SeqNo, time.Since(start))
			glog.V(common.SHORT).Infof("Finished transcoding segment %v, overall took %v\n\n\n", seqNo, time.Since(totalStart))
		} else {
			glog.Errorf("Invalid broadcaster signature for received segment for stream. Dropping segment")
		}
	})
	return resultStrmIDs, nil
}

func (n *LivepeerNode) transcodeAndBroadcastSeg(seg *stream.HLSSegment, sig []byte, cm eth.ClaimManager, t transcoder.Transcoder, resultStrmIDs []StreamID, broadcasters map[StreamID]stream.Broadcaster, config net.TranscodeConfig) {

	//Assume d is in the right format, write it to disk
	inName := randName()
	if _, err := os.Stat(n.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(n.WorkDir, 0700)
		if err != nil {
			glog.Errorf("Transcoder cannot create workdir: %v", err)
			return // TODO return error?
		}
	}
	fname := path.Join(n.WorkDir, inName)
	defer os.Remove(fname)
	if err := ioutil.WriteFile(fname, seg.Data, 0644); err != nil {
		glog.Errorf("Transcoder cannot write file: %v", err)
		return // TODO return error?
	}

	// Ensure length matches expectations. 4 second + 25% wiggle factor, 60fps
	err := ffmpeg.CheckMediaLen(fname, 4*1.25*1000, 60*4*1.25)
	if err != nil {
		glog.Errorf("Media length check failed: %v", err)
		return
	}
	//Do the transcoding
	start := time.Now()
	tData, err := t.Transcode(fname)
	if err != nil {
		glog.Errorf("Error transcoding seg: %v - %v", seg.Name, err)
		return
	}
	glog.V(common.DEBUG).Infof("Transcoding of segment %v took %v", seg.SeqNo, time.Since(start))

	//Encode and broadcast the segment
	start = time.Now()
	for i, resultStrmID := range resultStrmIDs {
		//Insert the transcoded segments into the streams (streams are already broadcasted to the network)
		if tData[i] == nil {
			glog.Errorf("Cannot find transcoded segment for %v", seg.SeqNo)
			continue
		}

		newSeg := &stream.HLSSegment{SeqNo: seg.SeqNo, Name: fmt.Sprintf("%v_%d.ts", resultStrmID, seg.SeqNo), Data: tData[i], Duration: seg.Duration}
		broadcaster, ok := broadcasters[resultStrmID]
		if !ok {
			// glog.Errorf("Cannot find stream for %v", tranStrms)
			glog.Errorf("Cannot find broadcaster for %v", resultStrmID)
			continue
		}
		if err := n.BroadcastHLSSegToNetwork(string(resultStrmID), newSeg, broadcaster); err != nil {
			glog.Errorf("Error inserting transcoded segment into network: %v", err)
		}

		//Don't do the onchain stuff unless specified
		if cm != nil && config.PerformOnchainClaim {
			cm.AddReceipt(int64(seg.SeqNo), seg.Data, crypto.Keccak256(tData[i]), sig, config.Profiles[i])
		}
	}
}

func (n *LivepeerNode) BroadcastFinishMsg(strmID string) error {
	b, err := n.VideoNetwork.GetBroadcaster(strmID)
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	return b.Finish()
}

func (n *LivepeerNode) BroadcastHLSSegToNetwork(strmID string, seg *stream.HLSSegment, b stream.Broadcaster) error {
	var sig []byte
	var err error
	if n.Eth != nil {
		segHash := (&ethTypes.Segment{StreamID: strmID, SegmentSequenceNumber: big.NewInt(int64(seg.SeqNo)), DataHash: crypto.Keccak256Hash(seg.Data)}).Hash()

		sig, err = n.Eth.Sign(segHash.Bytes())
		if err != nil {
			glog.Errorf("Error signing segment %v-%v: %v", strmID, seg.SeqNo, err)
			return err
		}
	}

	//Encode segment into []byte, broadcast it
	if ssb, err := SignedSegmentToBytes(SignedSegment{Seg: *seg, Sig: sig}); err == nil {
		if err = b.Broadcast(seg.SeqNo, ssb); err != nil {
			glog.Errorf("Error broadcasting segment to network: %v", err)
		}
	} else {
		glog.Errorf("Error encoding segment to []byte: %v", err)
		return err
	}

	return nil
}

//SubscribeFromNetwork subscribes to a stream on the network.  Returns the stream as a reference.
func (n *LivepeerNode) SubscribeFromNetwork(ctx context.Context, strmID StreamID, strm stream.HLSVideoStream) error {
	glog.V(common.DEBUG).Infof("Subscribe from network: %v", strmID)
	sub, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber: %v", err)
		return err
	}

	return sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		//1 - the subscriber quits
		if eof {
			glog.Infof("Got EOF, unsubscribing to %v", strmID)
			if err := sub.Unsubscribe(); err != nil {
				glog.Errorf("Unsubscribe error: %v", err)
				return
			}
			strm.End()
			return
		}

		//Decode data into HLSSegment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
			return
		}

		//Add segment into a HLS buffer in VideoDB
		// glog.Infof("Inserting seg %v into stream %v", ss.Seg.Name, strmID)
		if err = strm.AddHLSSegment(&ss.Seg); err != nil {
			glog.Errorf("Error adding segment: %v", err)
		}
	})
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

//GetMasterPlaylistFromNetwork blocks until it gets the playlist, or it times out.
func (n *LivepeerNode) GetMasterPlaylistFromNetwork(mid ManifestID) *m3u8.MasterPlaylist {
	timer := time.NewTimer(DefaultMasterPlaylistWaitTime)
	plChan, err := n.VideoNetwork.GetMasterPlaylist(string(mid.GetNodeID()), mid.String())
	if err != nil {
		glog.Errorf("Error getting master playlist: %v", err)
		return nil
	}
	select {
	case pl := <-plChan:
		//Got pl
		return pl
	case <-timer.C:
		//timed out
		return nil
	}

}

//NotifyBroadcaster sends a messages to the broadcaster of the video stream, containing the new streamIDs of the transcoded video streams.
func (n *LivepeerNode) NotifyBroadcaster(nid NodeID, strmID StreamID, transcodeStrmIDs map[StreamID]ffmpeg.VideoProfile) error {
	ids := make(map[string]string)
	for sid, p := range transcodeStrmIDs {
		ids[sid.String()] = p.Name
	}
	if nid == n.Identity {
		return nil
	}
	return n.VideoNetwork.SendTranscodeResponse(string(nid), strmID.String(), ids)
}

func (n *LivepeerNode) StartEthServices() error {
	var err error
	for _, s := range n.EthServices {
		err = s.Start(context.Background())
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *LivepeerNode) StopEthServices() error {
	var err error
	for _, s := range n.EthServices {
		err = s.Stop()
		if err != nil {
			return err
		}
	}

	return nil
}

func randName() string {
	rand.Seed(time.Now().UnixNano())
	x := make([]byte, 10, 10)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x.ts", x)
}
