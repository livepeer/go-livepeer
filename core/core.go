package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"math/big"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/livepeer/golp/eth"
	"github.com/livepeer/golp/net"
	"github.com/livepeer/lpms/stream"
	lptr "github.com/livepeer/lpms/transcoder"
)

var ErrTranscode = errors.New("ErrTranscode")
var ErrBroadcastTimeout = errors.New("ErrBroadcastTimeout")
var BroadcastTimeout = time.Second * 30

//NodeID can be converted from libp2p PeerID.
type NodeID string

type LivepeerNode struct {
	Identity     NodeID
	VideoNetwork net.VideoNetwork
	StreamDB     *StreamDB
	Eth          eth.LivepeerEthClient
	// IsTranscoder bool
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
	if err != nil {
		glog.Errorf("Error creating transcode job: %v", err)
		return err
	}

	glog.Infof("Created transcode job: %v", tx)

	return nil
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

	s.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
		if eof {
			glog.Infof("Stream finished")
		}

		//Decode the segment
		dec := gob.NewDecoder(bytes.NewReader(data))
		var seg stream.HLSSegment
		err := dec.Decode(&seg)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		for _, p := range config.Profiles {
			t := transcoders[p.Name]
			td, err := t.Transcode(seg.Data)
			if err != nil {
				glog.Errorf("Error transcoding for %v: %v", p.Name, err)
			} else {
				//Encode the transcoded segment
				b := broadcasters[p.Name]
				newSeg := stream.HLSSegment{SeqNo: seqNo, Name: fmt.Sprintf("%s_%d.ts", ids[p.Name], seqNo), Data: td, Duration: seg.Duration}
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err = enc.Encode(newSeg)
				if err != nil {
					glog.Errorf("Error encoding segment to []byte: %v", err)
					continue
				}

				//Broadcast the transcoded segment
				err = b.Broadcast(seqNo, buf.Bytes())
				if err != nil {
					glog.Errorf("Error broadcasting segment to network: %v", err)
				}
			}
		}
	})

	return resultStrmIDs, nil
}

//Monitor the smart contract for job creation (as a transcoder)
func (n *LivepeerNode) monitorEth() {

}

//StartTranscodeJob starts a transcode job, and sends the transcoded streamIDs to the broadcaster
func (n *LivepeerNode) StartTranscodeJob() error {
	if n.Eth == nil {
		glog.Errorf("Eth client undefined.")
		return ErrTranscode
	}

	//Start transcode jobs using the given config (async)

	//Send the streamIDs to the broadcaster

	//Subscribes to the original stream

	return nil
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

		seg, err := strm.ReadHLSSegment()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if seg.EOF == true {
			glog.Info("Got EOF for HLS Broadcast, Terminating Broadcast.")
			return nil
		}

		//Encode segment into []byte, broadcast it
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err = enc.Encode(seg)
		if err != nil {
			glog.Errorf("Error encoding segment to []byte: %v", err)
			continue
		}

		err = b.Broadcast(counter, buf.Bytes())
		if err != nil {
			glog.Errorf("Error broadcasting segment to network: %v", err)
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
	// s := n.VideoNetwork.GetSubscriber(strmID.String())
	// if s == nil {
	// 	s = n.VideoNetwork.NewSubscriber(strmID.String())
	// }

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

		//Decode data into HLSSegment
		dec := gob.NewDecoder(bytes.NewReader(data))
		var seg stream.HLSSegment
		err := dec.Decode(&seg)
		if err != nil {
			glog.Errorf("Error decoding byte array into segment: %v", err)
		}

		//Add segment into stream
		if err := strm.WriteHLSSegmentToStream(seg); err != nil {
			glog.Errorf("Error writing HLS Segment: %v", err)
		}

		// if buf == nil {
		// 	buf = s.LivepeerNode.StreamDB.AddNewHLSBuffer(strmID)
		// 	glog.Infof("Creating new buf in StreamDB: %v", s.LivepeerNode.StreamDB)
		// }
		// glog.Infof("Inserting seg %v into buf %v", seg.Name, buf)
		// buf.WriteSegment(seg.SeqNo, seg.Name, seg.Duration, seg.Data)
		// strm.WriteHLSSegmentToStream()
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
