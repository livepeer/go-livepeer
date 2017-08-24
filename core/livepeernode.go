/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ericxtang/m3u8"

	"github.com/ethereum/go-ethereum/common"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/types"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
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
var DefaultMasterPlaylistWaitTime = 10 * time.Second
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
	workDir      string
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork, wd string) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}
	return &LivepeerNode{StreamDB: NewStreamDB(vn.GetNodeID()), VideoNetwork: vn, Identity: NodeID(vn.GetNodeID()), Eth: e, workDir: wd}, nil
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

	//TODO:Kick off process to periodically monitor peer connection by pinging them
	return nil
}

//CreateTranscodeJob creates the on-chain transcode job.
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []types.VideoProfile, price uint64) error {
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return ErrNotFound
	}

	//Verify the stream exists(assume it's a local stream)
	if hlsStream := n.StreamDB.GetHLSStream(strmID); hlsStream == nil {
		glog.Errorf("Cannot find stream %v for creating transcode job", strmID)
		return ErrNotFound
	}

	//Call eth client to create the job
	p := big.NewInt(int64(price))

	var tOpt bytes.Buffer
	for _, p := range profiles {
		tOpt.WriteString(p.Name)
	}

	n.Eth.Job(strmID.String(), tOpt.String(), p)

	return nil
}

//TranscodeAndBroadcast transcodes one stream into multiple streams (specified by TranscodeConfig), broadcasts the streams, and returns a list of streamIDs.
func (n *LivepeerNode) TranscodeAndBroadcast(config net.TranscodeConfig, cm *ClaimManager) ([]StreamID, error) {
	//Get TranscodeProfiles from VideoProfiles, create the broadcasters
	tProfiles := make([]lptr.TranscodeProfile, len(config.Profiles), len(config.Profiles))
	broadcasters := make(map[StreamID]net.Broadcaster)
	resultStrmIDs := make([]StreamID, len(config.Profiles), len(config.Profiles))
	for i, vp := range config.Profiles {
		strmID, err := MakeStreamID(n.Identity, RandomVideoID(), vp.Name)
		if err != nil {
			glog.Errorf("Error making stream ID: %v", err)
			return nil, ErrTranscode
		}
		resultStrmIDs[i] = strmID
		tProfiles[i] = lptr.TranscodeProfileLookup[vp.Name]

		b, err := n.VideoNetwork.GetBroadcaster(strmID.String())
		if err != nil {
			glog.Errorf("Error creating broadcaster: %v", err)
			return nil, ErrTranscode
		}
		broadcasters[strmID] = b
	}

	//Create the transcoder
	t := transcoder.NewFFMpegSegmentTranscoder(tProfiles, "", n.workDir)

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	sub, err := n.VideoNetwork.GetSubscriber(config.StrmID)
	if err != nil {
		glog.Errorf("Error getting subscriber for stream %v from network: %v", config.StrmID, err)
	}
	glog.Infof("Config strm ID: %v", config.StrmID)
	glog.Infof("Subscriber: %v", sub)
	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if eof {
			glog.Infof("Stream finished.  Claiming work.")
			for _, p := range config.Profiles {
				cm.Claim(p)
			}

			return
		}

		//Decode the segment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		//Do the transcoding
		tData, err := t.Transcode(ss.Seg.Data)
		if err != nil {
			glog.Errorf("Error transcoding seg: %v - %v", ss.Seg.Name, err)
		}

		//Encode and broadcast the segment
		for i, strmID := range resultStrmIDs {

			//Encode the transcoded segment into bytes
			b := broadcasters[strmID]
			newSeg := stream.HLSSegment{SeqNo: seqNo, Name: fmt.Sprintf("%v_%d.ts", strmID, seqNo), Data: tData[i], Duration: ss.Seg.Duration}
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

			//Don't do the onchain stuff unless specified
			if config.PerformOnchainClaim {
				cm.AddReceipt(int64(seqNo), common.BytesToHash(ss.Seg.Data).Hex(), common.BytesToHash(tData[i]).Hex(), ss.Sig, config.Profiles[i])
			}

		}
	})

	return resultStrmIDs, nil
}

func (n *LivepeerNode) BroadcastFinishMsg(strmID string) error {
	b, err := n.VideoNetwork.GetBroadcaster(strmID)
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	return b.Finish()
}

//BroadcastToNetwork is called when a new broadcast stream is available.  It lets the network decide how
//to deal with the stream.
func (n *LivepeerNode) BroadcastToNetwork(strm stream.HLSVideoStream) error {
	b, err := n.VideoNetwork.GetBroadcaster(strm.GetStreamID())
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	//Set up the callback for when we get transcode results back
	n.VideoNetwork.ReceivedTranscodeResponse(strm.GetStreamID(), func(result map[string]string) {
		//Parse through the results
		for strmID, tProfile := range result {
			vParams := transcoder.TranscodeProfileToVariantParams(transcoder.TranscodeProfileLookup[tProfile])
			pl, _ := m3u8.NewMediaPlaylist(stream.DefaultMediaPlLen, stream.DefaultMediaPlLen)
			if err := strm.AddVariant(strmID, &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", strmID), Chunklist: pl, VariantParams: vParams}); err != nil {
				glog.Errorf("Error adding variant: %v", err)
			}
		}

		//Update the master playlist on the network
		mpl, err := strm.GetMasterPlaylist()
		if err != nil {
			glog.Errorf("Error getting master playlist: %v", err)
			return
		}
		if err = n.VideoNetwork.UpdateMasterPlaylist(strm.GetStreamID(), mpl); err != nil {
			glog.Errorf("Error updating master playlist on network: %v", err)
			return
		}

		glog.Infof("Updated master playlist for %v", strm.GetStreamID())
	})

	//Broadcast stream to network
	counter := uint64(0)
	strm.SetSubscriber(func(strm stream.HLSVideoStream, strmID string, seg *stream.HLSSegment) {
		//Get segment signature
		segHash := (&ethTypes.Segment{StreamID: strm.GetStreamID(), SegmentSequenceNumber: big.NewInt(int64(counter)), DataHash: common.BytesToHash(seg.Data).Hex()}).Hash()
		var sig []byte
		if c, ok := n.Eth.(*eth.Client); ok {
			sig, err = c.SignSegmentHash(n.EthPassword, segHash.Bytes())
			if err != nil {
				glog.Errorf("Error signing segment %v-%v: %v", strm.GetStreamID(), counter, err)
				return
			}
		}

		//Encode segment into []byte, broadcast it
		if ssb, err := SignedSegmentToBytes(SignedSegment{Seg: *seg, Sig: sig}); err == nil {
			// if ssb, err := SignedSegmentToBytes(SignedSegment{Seg: *seg, Sig: nil}); err == nil {
			if err = b.Broadcast(counter, ssb); err != nil {
				glog.Errorf("Error broadcasting segment to network: %v", err)
			}
		} else {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			return
		}

		counter++

	})
	return nil
}

//SubscribeFromNetwork subscribes to a stream on the network.  Returns the stream as a reference.
func (n *LivepeerNode) SubscribeFromNetwork(ctx context.Context, strmID StreamID, strm stream.HLSVideoStream) (net.Subscriber, error) {
	glog.Infof("Subscriber from network...")
	sub, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber: %v", err)
		return nil, err
	}

	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		//Two possibilities of ending the stream.
		//1 - the subscriber quits
		if eof {
			glog.Infof("Got EOF, writing to buf")
			strm.AddHLSSegment(strmID.String(), &stream.HLSSegment{Name: fmt.Sprintf("%v_eof", strmID), EOF: true})
			if err := sub.Unsubscribe(); err != nil {
				glog.Errorf("Unsubscribe error: %v", err)
				return
			}
		}

		//Decode data into HLSSegment
		ss, err := BytesToSignedSegment(data)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
			return
		}

		//Two possibilities of ending the stream.
		//2 - receive a EOF segment
		if ss.Seg.EOF {
			glog.Infof("Got EOF, writing to buf")
			// strm.AddHLSSegment(strmID.String(), &stream.HLSSegment{EOF: true})
			strm.AddHLSSegment(strmID.String(), &ss.Seg)
			if err := sub.Unsubscribe(); err != nil {
				glog.Errorf("Unsubscribe error: %v", err)
				return
			}
		}

		//Add segment into a HLS buffer in StreamDB
		// glog.Infof("Inserting seg %v into stream %v", ss.Seg.Name, strmID)
		if err = strm.AddHLSSegment(strmID.String(), &ss.Seg); err != nil {
			glog.Errorf("Error adding segment: %v", err)
		}
	})

	return sub, nil
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
func (n *LivepeerNode) GetMasterPlaylistFromNetwork(strmID StreamID) *m3u8.MasterPlaylist {
	timer := time.NewTimer(DefaultMasterPlaylistWaitTime)
	plChan, err := n.VideoNetwork.GetMasterPlaylist(string(strmID.GetNodeID()), strmID.String())
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
func (n *LivepeerNode) NotifyBroadcaster(nid NodeID, strmID StreamID, transcodeStrmIDs map[StreamID]types.VideoProfile) error {
	ids := make(map[string]string)
	for sid, p := range transcodeStrmIDs {
		ids[sid.String()] = p.Name
	}
	return n.VideoNetwork.SendTranscodeResponse(string(nid), strmID.String(), ids)
}
