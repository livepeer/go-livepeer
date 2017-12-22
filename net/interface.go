package net

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/stream"
)

//VideoNetwork describes the interface for a Livepeer node network-layer library.
type VideoNetwork interface {
	GetMasterPlaylist(nodeID string, manifestID string) (chan *m3u8.MasterPlaylist, error)
	GetSubscriber(strmID string) (stream.Subscriber, error)
	UpdateMasterPlaylist(manifestID string, mpl *m3u8.MasterPlaylist) error
	GetBroadcaster(strmID string) (stream.Broadcaster, error)
	GetNodeID() string
	Connect(nodeID string, nodeAddr []string) error
	SetupProtocol() error
	SendTranscodeResponse(nodeID string, manifestID string, transcodeResult map[string]string) error
	ReceivedTranscodeResponse(manifestID string, gotResult func(transcodeResult map[string]string))
	GetNodeStatus(nodeID string) (chan *NodeStatus, error)
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
		if len(mstrArr) == 2 {
			m := m3u8.NewMasterPlaylist()
			if err := m.DecodeFrom(strings.NewReader(mstrArr[1]), true); err != nil {
				glog.Errorf("Error decoding playlist: %v", err)
			} else {
				manifests[mstrArr[0]] = m
			}
		}
	}
	n.Manifests = manifests

	return nil
}
