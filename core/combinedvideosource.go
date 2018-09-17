package core

import (
	"sync"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

var liveListLev uint = 10

// CombinedVideoSource holds data in different OSes
type CombinedVideoSource struct {
	sources      map[ManifestID]*masterSource
	sourcesPLock sync.Mutex
}

type masterSource struct {
	manifestID         ManifestID
	masterLivePList    *m3u8.MasterPlaylist
	masterFullPList    *m3u8.MasterPlaylist
	streams            []*videoStream
	masterManifestName string
}

type videoStream struct {
	streamID          StreamID
	osSession         drivers.OSSession
	mediaLivePlayList *m3u8.MediaPlaylist
	// Contains full list of segments.
	mediaFullPlayList       *m3u8.MediaPlaylist
	doNotSaveMasterPlaylist bool
	doNotSaveMediaPlaylist  bool
}

func NewCombinedVideoSource() VideoSource {
	return &CombinedVideoSource{
		sourcesPLock: sync.Mutex{},
		sources:      make(map[ManifestID]*masterSource),
	}
}

func (c *CombinedVideoSource) AddStream(manifestID ManifestID, streamID StreamID, profile string,
	osSession drivers.OSSession, doNotSaveMasterPlaylist, doNotSaveMediaPlaylist bool) {

	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	glog.Infof("Adding stream %s to manifest %s with profile %s", streamID, manifestID, profile)
	src, ok := c.sources[manifestID]
	if !ok {
		src = &masterSource{
			manifestID:      manifestID,
			masterLivePList: m3u8.NewMasterPlaylist(),
			masterFullPList: m3u8.NewMasterPlaylist(),
			streams:         make([]*videoStream, 0, 16),
		}
		c.sources[manifestID] = src
	}
	livePL, err := m3u8.NewMediaPlaylist(liveListLev, liveListLev)
	if err != nil {
		glog.Error(err)
	}
	// XXX what if list will be longer?
	fullPL, err := m3u8.NewMediaPlaylist(131072, 131072)
	if err != nil {
		glog.Error(err)
	}
	strm := &videoStream{
		streamID:                streamID,
		osSession:               osSession,
		mediaLivePlayList:       livePL,
		mediaFullPlayList:       fullPL,
		doNotSaveMasterPlaylist: doNotSaveMasterPlaylist,
		doNotSaveMediaPlaylist:  doNotSaveMediaPlaylist,
	}
	src.streams = append(src.streams, strm)
	if !doNotSaveMediaPlaylist {
		vParams := ffmpeg.VideoProfileToVariantParams(ffmpeg.VideoProfileLookup[profile])
		sid := string(streamID)
		_, url, err := strm.osSession.SaveData(sid, sid+"Full.m3u8", strm.mediaFullPlayList.Encode().Bytes())
		if err != nil {
			glog.Warningf("Error saving media playlist for %s stream %v", streamID, err)
		}
		src.masterFullPList.Append(url, fullPL, vParams)
		_, url, err = strm.osSession.SaveData(sid, sid+".m3u8", strm.mediaLivePlayList.Encode().Bytes())
		if err != nil {
			glog.Warningf("Error saving media playlist for %s stream %v", streamID, err)
		}
		src.masterLivePList.Append(url, livePL, vParams)
	}
}

func (c *CombinedVideoSource) makeMediaSegment(seg *stream.HLSSegment, url string) *m3u8.MediaSegment {
	mseg := new(m3u8.MediaSegment)
	mseg.URI = url
	mseg.Duration = seg.Duration
	mseg.Title = seg.Name
	mseg.SeqId = seg.SeqNo
	return mseg
}

func hasSegment(mpl *m3u8.MediaPlaylist, seqNum uint64) bool {
	for _, seg := range mpl.Segments {
		if seg != nil && seg.SeqId == seqNum {
			return true
		}
	}
	return false
}

func (c *CombinedVideoSource) InsertLink(turi *net.TypedURI, seqNo uint64) (bool, error) {
	inserted := false
	for _, src := range c.sources {
		for _, stream := range src.streams {
			if string(stream.streamID) == turi.StreamID && stream.osSession.IsOwnStorage(turi) {
				mseg := new(m3u8.MediaSegment)
				// only SeqId matters because this playlist only used to check if segment was alread added
				mseg.SeqId = seqNo
				stream.addMediaSegment(mseg)
				inserted = true
			}
		}
	}
	return inserted, nil
}

func (vs *videoStream) addMediaSegment(seg *m3u8.MediaSegment) error {
	if vs.mediaLivePlayList.Count() >= vs.mediaLivePlayList.WinSize() {
		vs.mediaLivePlayList.Remove()
	}
	if vs.mediaLivePlayList.Count() == 0 {
		vs.mediaLivePlayList.SeqNo = seg.SeqId
	}
	return vs.mediaLivePlayList.AppendSegment(seg)
}

func (c *CombinedVideoSource) InsertHLSSegment(streamID StreamID, seg *stream.HLSSegment) ([]*net.TypedURI, error) {
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	res := make([]*net.TypedURI, 0, 4)
	for _, src := range c.sources {
		for _, stream := range src.streams {
			if stream.streamID == streamID {
				if hasSegment(stream.mediaLivePlayList, seg.SeqNo) {
					continue
				}
				glog.Infof("Inserting segment with duration %v in stream %s", seg.Duration, streamID)
				sid := string(streamID)
				// turi, err := stream.osSession.SaveSegment(sid, seg)
				turi, _, err := stream.osSession.SaveData(sid, seg.Name, seg.Data)
				if err != nil {
					glog.Error("Error saving segment", err)
				} else {
					// for ipfs we should insert 'ipfs://' link?
					mseg := c.makeMediaSegment(seg, turi.UriInManifest)
					err := stream.addMediaSegment(mseg)
					if err != nil {
						glog.Error(err)
					}
					// glog.Infof("%s live list after append:", sid)
					// glog.Info(stream.mediaLivePlayList.String())
					mseg = c.makeMediaSegment(seg, turi.UriInManifest)
					if stream.mediaFullPlayList.Count() == 0 {
						stream.mediaFullPlayList.SeqNo = seg.SeqNo
					}
					err = stream.mediaFullPlayList.AppendSegment(mseg)
					if err != nil {
						glog.Error(err)
					}
					res = append(res, turi)
					if !stream.doNotSaveMediaPlaylist {
						_, plURL, err := stream.osSession.SaveData(sid, sid+"Full.m3u8", stream.mediaFullPlayList.Encode().Bytes())
						if err != nil {
							glog.Warningf("Error saving full media playlist for %s stream %v", streamID, err)
						} else {
							// update master play list with new link to media playlist (IPFS will have new link after each save)
							c.updateVariantURL(src.masterFullPList, stream.mediaFullPlayList, plURL)
						}
						_, plURL, err = stream.osSession.SaveData(sid, sid+".m3u8", stream.mediaLivePlayList.Encode().Bytes())
						if err != nil {
							glog.Warningf("Error saving live media playlist for %s stream %v", streamID, err)
						} else {
							// XXX update master play list with new link to media playlist (IPFS will have new link after each save)
							glog.Info(plURL)
							c.updateVariantURL(src.masterLivePList, stream.mediaLivePlayList, plURL)
						}
					}
					if !stream.doNotSaveMasterPlaylist {
						_, _, err = stream.osSession.SaveData(sid,
							string(src.manifestID)+"MasterFull.m3u8", src.masterFullPList.Encode().Bytes())
						if err != nil {
							glog.Warningf("Error saving master full playlist for %s stream %v", streamID, err)
						}
					}
				}
			}
		}
	}
	return res, nil
}

func (c *CombinedVideoSource) updateVariantURL(mspl *m3u8.MasterPlaylist, mdpl *m3u8.MediaPlaylist, newURL string) {
	for _, v := range mspl.Variants {
		if v.Chunklist == mdpl {
			v.URI = newURL
			break
		}
	}
}

func (c *CombinedVideoSource) GetLiveHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist {
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	src, ok := c.sources[manifestID]
	if !ok {
		return nil
	}
	return src.masterLivePList
}

func (c *CombinedVideoSource) GetFullHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist {
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	src, ok := c.sources[manifestID]
	if !ok {
		return nil
	}
	return src.masterFullPList
}

func (c *CombinedVideoSource) EvictHLSMasterPlaylist(manifestID ManifestID) {
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	src := c.sources[manifestID]
	if src != nil {
		for _, strm := range src.streams {
			strm.osSession.EndSession()
		}
	}
	delete(c.sources, manifestID)
}

func (c *CombinedVideoSource) GetLiveHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist {
	if stream := c.getFirstStream(streamID); stream != nil {
		return stream.mediaLivePlayList
	}
	return nil
}

func (c *CombinedVideoSource) GetFullHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist {
	if stream := c.getFirstStream(streamID); stream != nil {
		return stream.mediaFullPlayList
	}
	return nil
}

func (c *CombinedVideoSource) getFirstStream(streamID StreamID) *videoStream {
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	for _, source := range c.sources {
		for _, stream := range source.streams {
			if stream.streamID == streamID {
				return stream
			}
		}
	}
	return nil
}

func (c *CombinedVideoSource) GetNodeStatus() *net.NodeStatus {
	// not threadsafe; need to deep copy the playlist
	c.sourcesPLock.Lock()
	defer c.sourcesPLock.Unlock()
	m := make(map[string]*m3u8.MasterPlaylist, 0)
	for k, src := range c.sources {
		m[string(k)] = src.masterLivePList
	}
	return &net.NodeStatus{Manifests: m}
}
