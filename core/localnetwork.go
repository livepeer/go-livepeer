package core

import (
	"sync"

	"github.com/ericxtang/m3u8"

	"github.com/livepeer/go-livepeer/net"
)

/* Basically a small cache for master playlists. Move into videocache? */

type LocalNetwork struct {
	mutex     *sync.Mutex
	playlists map[string]*m3u8.MasterPlaylist
}

func NewLocalNetwork() *LocalNetwork {
	return &LocalNetwork{
		mutex:     &sync.Mutex{},
		playlists: make(map[string]*m3u8.MasterPlaylist),
	}
}

func (n *LocalNetwork) GetMasterPlaylist(nodeID string, manifestID string) (chan *m3u8.MasterPlaylist, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	pl, ok := n.playlists[manifestID]
	ret := make(chan *m3u8.MasterPlaylist)
	go func() { ret <- pl }()
	var err error
	if !ok {
		err = ErrNotFound
	}
	return ret, err
}

func (n *LocalNetwork) UpdateMasterPlaylist(manifestID string, mpl *m3u8.MasterPlaylist) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if mpl == nil {
		delete(n.playlists, manifestID)
	} else {
		n.playlists[manifestID] = mpl
	}
	return nil
}

func (n *LocalNetwork) GetNodeStatus(nodeID string) (chan *net.NodeStatus, error) {
	// not threadsafe; need to deep copy the playlist
	st := &net.NodeStatus{NodeID: nodeID, Manifests: n.playlists}
	ret := make(chan *net.NodeStatus)
	go func() { ret <- st }()
	return ret, nil
}

func (n *LocalNetwork) String() string {
	return "LocalNetwork"
}
