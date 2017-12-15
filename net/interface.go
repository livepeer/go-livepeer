package net

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	lpmscore "github.com/livepeer/lpms/core"
)

//VideoNetwork describes the interface for a Livepeer node network-layer library.
type VideoNetwork interface {
	GetNodeID() string
	GetMasterPlaylist(nodeID string, manifestID string) (chan *m3u8.MasterPlaylist, error)
	UpdateMasterPlaylist(manifestID string, mpl *m3u8.MasterPlaylist) error
	GetBroadcaster(strmID string) (Broadcaster, error)
	GetSubscriber(strmID string) (Subscriber, error)
	Connect(nodeID string, nodeAddr []string) error
	SetupProtocol() error
	SendTranscodeResponse(nodeID string, manifestID string, transcodeResult map[string]string) error
	ReceivedTranscodeResponse(manifestID string, gotResult func(transcodeResult map[string]string))
	GetNodeStatus(nodeID string) (chan *NodeStatus, error)
	String() string
}

//Broadcaster takes a streamID and a reader, and broadcasts the data to whatever underlining network.
//Note the data param doesn't have to be the raw data.  The implementation can choose to encode any struct.
//Example:
// 	s := GetStream("StrmID")
// 	b := ppspp.NewBroadcaster("StrmID", s.Metadata())
// 	for seqNo, data := range s.Segments() {
// 		b.Broadcast(seqNo, data)
// 	}
//	b.Finish()
type Broadcaster interface {
	Broadcast(seqNo uint64, data []byte) error
	IsWorking() bool
	Finish() error
	String() string
}

//Subscriber subscribes to a stream defined by strmID.  It returns a reader that contains the stream.
//Example 1:
//	sub, metadata := ppspp.NewSubscriber("StrmID")
//	stream := NewStream("StrmID", metadata)
//	ctx, cancel := context.WithCancel(context.Background()
//	err := sub.Subscribe(ctx, func(seqNo uint64, data []byte){
//		stream.WriteSeg(seqNo, data)
//	})
//	time.Sleep(time.Second * 5)
//	cancel()
//
//Example 2:
//	sub.Unsubscribe() //This is the same with calling cancel()
type Subscriber interface {
	Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error
	IsWorking() bool
	Unsubscribe() error
	String() string
}

type TranscodeConfig struct {
	StrmID              string
	Profiles            []lpmscore.VideoProfile
	PerformOnchainClaim bool
	JobID               *big.Int
}

type Transcoder interface {
	Transcode(strmID string, config TranscodeConfig, gotPlaylist func(masterPlaylist []byte)) error
}

type NodeStatus struct {
	NodeID    string
	Manifests map[string]*m3u8.MasterPlaylist
}

func (n NodeStatus) String() string {
	mstrs := make([]string, 0)
	for mid, m := range n.Manifests {
		mstrs = append(mstrs, fmt.Sprintf("%v[]%v", mid, m.String()))
	}
	str := fmt.Sprintf("%v|%v", n.NodeID, strings.Join(mstrs, "|"))
	return str
}

func (n *NodeStatus) FromString(str string) error {
	arr := strings.Split(str, "|")
	if len(arr[0]) != 68 {
		return errors.New("Wrong format for NodeStatus")
	} else {
		n.NodeID = arr[0]
	}

	manifests := make(map[string]*m3u8.MasterPlaylist, 0)
	for _, mstr := range arr[1:] {
		//Decode the playlist from a string
		mstrArr := strings.Split(mstr, "[]")
		m := m3u8.NewMasterPlaylist()
		if err := m.DecodeFrom(strings.NewReader(mstrArr[1]), true); err != nil {
			glog.Errorf("Error decoding playlist: %v", err)
		} else {
			manifests[mstrArr[0]] = m
		}
	}
	n.Manifests = manifests

	return nil
}
