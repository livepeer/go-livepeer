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
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/types"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"
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
var DefaultMasterPlaylistWaitTime = 60 * time.Second
var VerifyRate = int64(2)

//NodeID can be converted from libp2p PeerID.
type NodeID string

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {
	Identity     NodeID
	Addrs        []string
	VideoNetwork net.VideoNetwork
	StreamDB     *StreamDB
	Eth          eth.LivepeerEthClient
	EthAccount   string
	EthPassword  string
	WorkDir      string
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork, nodeId NodeID, addrs []string, wd string) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}

	return &LivepeerNode{StreamDB: NewStreamDB(vn.GetNodeID()), VideoNetwork: vn, Identity: nodeId, Addrs: addrs, Eth: e, WorkDir: wd}, nil
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
func (n *LivepeerNode) TranscodeAndBroadcast(config net.TranscodeConfig, cm ClaimManager, t transcoder.Transcoder) ([]StreamID, error) {
	//Get TranscodeProfiles from VideoProfiles, create the broadcasters
	tProfiles := make([]transcoder.TranscodeProfile, len(config.Profiles), len(config.Profiles))
	resultStrmIDs := make([]StreamID, len(config.Profiles), len(config.Profiles))
	tranStrms := make(map[StreamID]stream.HLSVideoStream)
	for i, vp := range config.Profiles {
		strmID, err := MakeStreamID(n.Identity, RandomVideoID(), vp.Name)
		if err != nil {
			glog.Errorf("Error making stream ID: %v", err)
			return nil, ErrTranscode
		}
		resultStrmIDs[i] = strmID
		tProfiles[i] = transcoder.TranscodeProfileLookup[vp.Name]
		newStrm, err := n.StreamDB.AddNewHLSStream(strmID)
		if err != nil {
			glog.Errorf("Error making new stream: %v", err)
			return nil, ErrTranscode
		}
		tranStrms[strmID] = newStrm
		if err := n.BroadcastToNetwork(newStrm); err != nil {
			glog.Errorf("Error broadcasting transcoded stream: %v", err)
			return nil, ErrTranscode
		}
	}

	//If we found a local stream, transcode and broadcast local stream.  This is for testing only, so we'll always set cm to nil.
	localStrm := n.StreamDB.GetHLSStream(StreamID(config.StrmID))
	if localStrm != nil {
		glog.V(common.SHORT).Infof("Transcoding local stream: %v", config.StrmID)
		localStrm.SetSubscriber(func(origStrm stream.HLSVideoStream, strmID string, seg *stream.HLSSegment) {
			n.transcodeAndBroadcastSeg(seg, nil, nil, t, resultStrmIDs, tranStrms, config)
		})
		for i, tStrmID := range resultStrmIDs {
			vParams := transcoder.TranscodeProfileToVariantParams(tProfiles[i])
			pl, _ := m3u8.NewMediaPlaylist(HLSStreamWinSize, stream.DefaultMediaPlLen)
			if err := localStrm.AddVariant(tStrmID.String(), &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", tStrmID), Chunklist: pl, VariantParams: vParams}); err != nil {
				glog.Errorf("Error adding variant: %v", err)
			}
			tranStrms[tStrmID].SetSubscriber(func(strm stream.HLSVideoStream, strmID string, seg *stream.HLSSegment) {
				localStrm.AddHLSSegment(strmID, seg)
			})
		}
		return resultStrmIDs, nil
	}

	//Subscribe to broadcast video, do the transcoding, broadcast the transcoded video, do the on-chain claim / verify
	sub, err := n.VideoNetwork.GetSubscriber(config.StrmID)
	if err != nil {
		glog.Errorf("Error getting subscriber for stream %v from network: %v", config.StrmID, err)
	}
	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		glog.V(common.DEBUG).Infof("Starting to transcode segment %v", seqNo)
		totalStart := time.Now()
		if eof {
			if cm != nil {
				glog.V(common.SHORT).Infof("Stream finished. Claiming work.")

				for _, p := range config.Profiles {
					cm.Claim(p)
				}
			}

			return
		}

		if cm != nil {
			sufficient, err := cm.SufficientBroadcasterDeposit()
			if err != nil {
				glog.Errorf("Error checking broadcaster funds: %v", err)
			}

			if !sufficient {
				glog.V(common.SHORT).Infof("Broadcaster does not have enough funds. Claiming work.")

				for _, p := range config.Profiles {
					cm.Claim(p)
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
		n.transcodeAndBroadcastSeg(&ss.Seg, ss.Sig, cm, t, resultStrmIDs, tranStrms, config)
		glog.V(common.DEBUG).Infof("Encoding and broadcasting of segment %v took %v", ss.Seg.SeqNo, time.Since(start))
		glog.V(common.SHORT).Infof("Finished transcoding segment %v, overall took %v\n\n\n", seqNo, time.Since(totalStart))
	})
	return resultStrmIDs, nil
}

func (n *LivepeerNode) transcodeAndBroadcastSeg(seg *stream.HLSSegment, sig []byte, cm ClaimManager, t transcoder.Transcoder, resultStrmIDs []StreamID, tranStrms map[StreamID]stream.HLSVideoStream, config net.TranscodeConfig) {
	//Do the transcoding
	start := time.Now()
	tData, err := t.Transcode(seg.Data)
	if err != nil {
		glog.Errorf("Error transcoding seg: %v - %v", seg.Name, err)
	}
	glog.V(common.DEBUG).Infof("Transcoding of segment %v took %v", seg.SeqNo, time.Since(start))

	//Encode and broadcast the segment
	start = time.Now()
	for i, resultStrmID := range resultStrmIDs {
		// if err := ioutil.WriteFile(filepath.Join(n.WorkDir, "transegs", fmt.Sprintf("%v_%d.ts", resultStrmID, seg.SeqNo)), tData[i], 0600); err != nil {
		// 	glog.Errorf("Error writing transcoded seg: %v", err)
		// }

		//Insert the transcoded segments into the streams (streams are already broadcasted to the network)
		if tData[i] == nil {
			glog.Errorf("Cannot find transcoded segment for %v", seg.SeqNo)
			continue
		}
		newSeg := stream.HLSSegment{SeqNo: seg.SeqNo, Name: fmt.Sprintf("%v_%d.ts", resultStrmID, seg.SeqNo), Data: tData[i], Duration: seg.Duration}
		transStrm, ok := tranStrms[resultStrmID]
		if !ok {
			glog.Errorf("Cannot find stream for %v", tranStrms)
			continue
		}
		if err := transStrm.AddHLSSegment(resultStrmID.String(), &newSeg); err != nil {
			glog.Errorf("Error insert transcoded segment into video stream: %v", err)
		}

		//Don't do the onchain stuff unless specified
		if cm != nil && config.PerformOnchainClaim {
			cm.AddReceipt(int64(seg.SeqNo), ethcommon.BytesToHash(seg.Data).Hex(), ethcommon.BytesToHash(tData[i]).Hex(), sig, config.Profiles[i])
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

//BroadcastToNetwork is called when a new broadcast stream is available.  It lets the network decide how
//to deal with the stream.
func (n *LivepeerNode) BroadcastToNetwork(strm stream.HLSVideoStream) error {
	//Update the playlist to the network
	mpl, err := strm.GetMasterPlaylist()
	if err != nil {
		glog.Errorf("Error getting master playlist: %v", err)
	}
	if err = n.VideoNetwork.UpdateMasterPlaylist(strm.GetStreamID(), mpl); err != nil {
		glog.Errorf("Error updating master playlist: %v", err)
	}

	//Get the broadcaster from the network
	b, err := n.VideoNetwork.GetBroadcaster(strm.GetStreamID())
	if err != nil {
		glog.Errorf("Error getting broadcaster from network: %v", err)
		return err
	}

	//Set up the callback for when we get transcode results back.  It's here because any stream can be transcoded.
	n.VideoNetwork.ReceivedTranscodeResponse(strm.GetStreamID(), func(result map[string]string) {
		//Parse through the results
		for strmID, tProfile := range result {
			vParams := transcoder.TranscodeProfileToVariantParams(transcoder.TranscodeProfileLookup[tProfile])
			pl, _ := m3u8.NewMediaPlaylist(HLSStreamWinSize, stream.DefaultMediaPlLen)
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

		glog.V(common.SHORT).Infof("Updated master playlist for %v", strm.GetStreamID())
	})

	//Broadcast stream to network
	counter := uint64(0)
	strm.SetSubscriber(func(strm stream.HLSVideoStream, strmID string, seg *stream.HLSSegment) {
		//Get segment signature
		segHash := (&ethTypes.Segment{StreamID: strm.GetStreamID(), SegmentSequenceNumber: big.NewInt(int64(counter)), DataHash: ethcommon.BytesToHash(seg.Data).Hex()}).Hash()
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
	glog.V(common.DEBUG).Infof("Subscribe from network: %v", strmID)
	sub, err := n.VideoNetwork.GetSubscriber(strmID.String())
	if err != nil {
		glog.Errorf("Error getting subscriber: %v", err)
		return nil, err
	}

	sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		//1 - the subscriber quits
		if eof {
			glog.V(common.SHORT).Infof("Got EOF, unsubscribing")
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
