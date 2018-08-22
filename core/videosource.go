package core

import (
	"strings"
	"sync"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

var GetMasterPlaylistWaitTime = time.Second * 5
var GetMediaPlaylistWaitTime = time.Second * 10
var SegCacheLen = 10

type VideoSource interface {
	GetHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist
	UpdateHLSMasterPlaylist(manifestID ManifestID, mpl *m3u8.MasterPlaylist)
	EvictHLSMasterPlaylist(manifestID ManifestID)
	GetHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist
	InsertHLSSegment(streamID StreamID, seg *stream.HLSSegment)
	GetHLSSegment(streamID StreamID, segName string) *stream.HLSSegment
	EvictHLSStream(streamID StreamID) error

	GetNodeStatus() *net.NodeStatus
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

type BasicVideoSource struct {
	segCache map[StreamID]*segCache
	segLock  sync.Mutex

	masterPList map[ManifestID]*m3u8.MasterPlaylist
	masterPLock sync.Mutex
}

func NewBasicVideoSource() VideoSource {
	return &BasicVideoSource{segCache: make(map[StreamID]*segCache), segLock: sync.Mutex{}, masterPLock: sync.Mutex{}, masterPList: make(map[ManifestID]*m3u8.MasterPlaylist)}
}

func (c *BasicVideoSource) GetCache(strmID StreamID) (*segCache, bool) {
	c.segLock.Lock()
	defer c.segLock.Unlock()

	sc, ok := c.segCache[strmID]
	return sc, ok
}

func (c *BasicVideoSource) DeleteCache(strmID StreamID) {
	c.segLock.Lock()
	defer c.segLock.Unlock()
	delete(c.segCache, strmID)
}

func (c *BasicVideoSource) GetHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist {
	c.masterPLock.Lock()
	defer c.masterPLock.Unlock()
	pl, ok := c.masterPList[manifestID]
	if !ok {
		return nil
	}
	return pl
}

func (c *BasicVideoSource) UpdateHLSMasterPlaylist(manifestID ManifestID, mpl *m3u8.MasterPlaylist) {
	c.masterPLock.Lock()
	defer c.masterPLock.Unlock()
	if mpl == nil {
		delete(c.masterPList, manifestID)
	} else {
		c.masterPList[manifestID] = mpl
	}
}

func (c *BasicVideoSource) EvictHLSMasterPlaylist(manifestID ManifestID) {
	c.masterPLock.Lock()
	mpl := c.masterPList[manifestID]
	if mpl != nil {
		for _, variant := range mpl.Variants {
			if strings.Contains(variant.URI, ".m3u8") {
				streamID := strings.Replace(variant.URI, ".m3u8", "", -1) // remove .m3u8 from end
				c.DeleteCache(StreamID(streamID))
			}
		}
	}
	c.masterPLock.Unlock()
	c.UpdateHLSMasterPlaylist(manifestID, nil)
}

func (c *BasicVideoSource) GetHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist {
	//If we have the stream, just return the playlist
	if cache, ok := c.GetCache(streamID); ok {
		return cache.GetMediaPlaylist()
	}
	return nil
}

func (c *BasicVideoSource) InsertHLSSegment(streamID StreamID, seg *stream.HLSSegment) {
	c.segLock.Lock()
	defer c.segLock.Unlock()
	sc, ok := c.segCache[streamID]
	if !ok {
		sc = newSegCache(SegCacheLen)
		c.segCache[streamID] = sc
	}
	sc.Insert(seg)
}

func (c *BasicVideoSource) GetHLSSegment(streamID StreamID, segName string) *stream.HLSSegment {
	if cache, ok := c.segCache[streamID]; !ok {
		return nil
	} else {
		return cache.GetSeg(segName)
	}
}

func (c *BasicVideoSource) EvictHLSStream(streamID StreamID) error {
	c.DeleteCache(streamID)
	return nil
}

func (c *BasicVideoSource) GetNodeStatus() *net.NodeStatus {
	// not threadsafe; need to deep copy the playlist
	c.masterPLock.Lock()
	defer c.masterPLock.Unlock()
	m := make(map[string]*m3u8.MasterPlaylist, 0)
	for k, v := range c.masterPList {
		m[string(k)] = v
	}
	return &net.NodeStatus{Manifests: m}
}
