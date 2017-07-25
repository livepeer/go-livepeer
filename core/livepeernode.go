/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/golp/eth"
	ethTypes "github.com/livepeer/golp/eth/types"
	"github.com/livepeer/golp/net"
	"github.com/livepeer/lpms/stream"
	lptr "github.com/livepeer/lpms/transcoder"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var ErrBroadcastTimeout = errors.New("ErrBroadcastTimeout")
var ErrBroadcastJob = errors.New("ErrBroadcastJob")
var ErrEOF = errors.New("ErrEOF")
var BroadcastTimeout = time.Second * 30
var ClaimInterval = int64(5)
var EthRpcTimeout = 5 * time.Second
var EthEventTimeout = 5 * time.Second
var EthMinedTxTimeout = 60 * time.Second
var VerifyRate = int64(2)

//NodeID can be converted from libp2p PeerID.
type NodeID string

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {
	Identity     NodeID
	VideoNetwork net.VideoNetwork
	StreamDB     *StreamDB
	Eth          eth.LivepeerEthClient
	EthPassword  string
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}
	return &LivepeerNode{StreamDB: NewStreamDB(vn.GetNodeID()), VideoNetwork: vn, Identity: NodeID(vn.GetNodeID()), Eth: e}, nil
}

//Start sets up the Livepeer protocol and connects the node to the network
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

	glog.Infof("Done Starting Livepeer Node")

	//TODO:Kick off process to periodically monitor peer connection by pinging them

	return nil
}

//CreateTranscodeJob creates the on-chain transcode job.
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []net.VideoProfile, price uint64) error {
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return ErrNotFound
	}

	//Verify the stream exists(assume it's a local stream)
	buf := n.StreamDB.GetHLSBuffer(strmID)
	if buf == nil {
		glog.Errorf("Cannot find stream %v for creating transcode job", strmID)
		return ErrNotFound
	}

	//Call eth client to create the job
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

//Transcode transcodes one stream into multiple stream (specified by TranscodeConfig), and returns a list of StreamIDs, in the order of the video profiles.
func (n *LivepeerNode) Transcode(config net.TranscodeConfig, cm *ClaimManager) ([]StreamID, error) {
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

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	// var startSeq, endSeq int64
	s.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if eof {
			glog.Infof("Stream finished.  Claiming work.")
			for _, p := range config.Profiles {
				cm.Claim(p)
			}
		}

		//Decode the segment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		for _, p := range config.Profiles {
			t := transcoders[p.Name]
			if ss.Seg.EOF {
				glog.Infof("Stream finished.  Claiming work.")
				cm.Claim(p)
				continue
			}

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
			if config.PerformOnchainClaim {
				cm.AddClaim(int64(seqNo), common.BytesToHash(ss.Seg.Data), common.BytesToHash(td), ss.Sig, p)
			}

		}
	})

	return resultStrmIDs, nil
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

		if seg.EOF == true {
			glog.Info("Got EOF for HLS Broadcast, Terminating Broadcast.")
			return ErrEOF
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

//NotifyBroadcaster sends a messages to the broadcaster of the video stream, containing the new streamIDs of the transcoded video streams.
func (n *LivepeerNode) NotifyBroadcaster(nid NodeID, strmID StreamID, transcodeStrmIDs map[StreamID]net.VideoProfile) error {
	ids := make(map[string]string)
	for sid, p := range transcodeStrmIDs {
		ids[sid.String()] = p.Name
	}
	return n.VideoNetwork.SendTranscodeResult(string(nid), strmID.String(), ids)
}
