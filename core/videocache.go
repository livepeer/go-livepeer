package core

import (
	"sync"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

var GetMasterPlaylistWaitTime = time.Second * 5
var GetMediaPlaylistWaitTime = time.Second * 10
var SegCacheLen = 10

type VideoCache interface {
	GetHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist
	EvictHLSMasterPlaylist(manifestID ManifestID)
	GetHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist
	GetHLSSegment(streamID StreamID, segName string) *stream.HLSSegment
	EvictHLSStream(streamID StreamID) error
}

type segCache struct {
	cacheLen int
	cache    []*stream.HLSSegment
}

func newSegCache(len int) *segCache {
	return &segCache{cacheLen: len, cache: make([]*stream.HLSSegment, 0)}
}

func (sc *segCache) Insert(seg *stream.HLSSegment) {
	if len(sc.cache) >= sc.cacheLen {
		sc.cache = sc.cache[1:]
	}
	sc.cache = append(sc.cache, seg)
}

func (sc *segCache) GetSeg(segName string) *stream.HLSSegment {
	for _, s := range sc.cache {
		if s.Name == segName {
			return s
		}
	}
	return nil
}

func (sc *segCache) GetMediaPlaylist() *m3u8.MediaPlaylist {
	//Make a media playlist
	pl, _ := m3u8.NewMediaPlaylist(uint(sc.cacheLen), uint(sc.cacheLen))
	for _, seg := range sc.cache {
		pl.Append(seg.Name, seg.Duration, "")
	}
	pl.SeqNo = sc.cache[0].SeqNo
	return pl
}

type BasicVideoCache struct {
	network  net.VideoNetwork
	segCache map[StreamID]*segCache
	segLock  sync.Mutex
}

func NewBasicVideoCache(nw net.VideoNetwork) *BasicVideoCache {
	return &BasicVideoCache{network: nw, segCache: make(map[StreamID]*segCache), segLock: sync.Mutex{}}
}

func (c *BasicVideoCache) GetCache(strmID StreamID) (*segCache, bool) {
	c.segLock.Lock()
	defer c.segLock.Unlock()

	sc, ok := c.segCache[strmID]
	return sc, ok
}

func (c *BasicVideoCache) DeleteCache(strmID StreamID) {
	c.segLock.Lock()
	defer c.segLock.Unlock()
	delete(c.segCache, strmID)
}

func (c *BasicVideoCache) GetHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist {
	plc, err := c.network.GetMasterPlaylist(string(manifestID.GetNodeID()), string(manifestID))
	if err != nil {
		return nil
	}

	timer := time.NewTimer(GetMasterPlaylistWaitTime)
	select {
	case pl := <-plc:
		return pl
	case <-timer.C:
		return nil
	}
}

func (c *BasicVideoCache) EvictHLSMasterPlaylist(manifestID ManifestID) {
	c.network.UpdateMasterPlaylist(string(manifestID), nil)
	return
}

func (c *BasicVideoCache) GetHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist {
	//If we have the stream, just return the playlist
	if cache, ok := c.GetCache(streamID); ok {
		return cache.GetMediaPlaylist()
	}

	//If we don't already have the stream, subscribe and return the playlist
	//XXX implement
	return nil
}

func (c *BasicVideoCache) GetHLSSegment(streamID StreamID, segName string) *stream.HLSSegment {
	if cache, ok := c.segCache[streamID]; !ok {
		return nil
	} else {
		return cache.GetSeg(segName)
	}
}

func (c *BasicVideoCache) EvictHLSStream(streamID StreamID) error {
	c.DeleteCache(streamID)
	return nil
}
