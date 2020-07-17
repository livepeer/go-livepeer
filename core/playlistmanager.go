package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/m3u8"
)

const LIVE_LIST_LENGTH uint = 6

//	PlaylistManager manages playlists and data for one video stream, backed by one object storage.
type PlaylistManager interface {
	ManifestID() ManifestID
	// Implicitly creates master and media playlists
	// Inserts in media playlist given a link to a segment
	InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) error

	GetHLSMasterPlaylist() *m3u8.MasterPlaylist

	GetHLSMediaPlaylist(rendition string) *m3u8.MediaPlaylist

	GetOSSession() drivers.OSSession

	GetRecordOSSession() drivers.OSSession

	FlushRecord()

	Cleanup()
}

type BasicPlaylistManager struct {
	storageSession drivers.OSSession
	recordSession  drivers.OSSession
	manifestID     ManifestID
	// Live playlist used for broadcasting
	masterPList *m3u8.MasterPlaylist
	mediaLists  map[string]*m3u8.MediaPlaylist
	jsonList    *jsonPlaylist
	mapSync     *sync.RWMutex
}

type jsonSeg struct {
	SeqNo      uint64 `json:"seq_no,omitempty"`
	URI        string `json:"uri,omitempty"`
	DurationMS uint64 `json:"duration_ms,omitempty"`
}

type jsonPlaylist struct {
	name       string
	DurationMs uint64               `json:"duration_ms,omitempty"` // total duration of the saved sagments
	Segments   map[string][]jsonSeg `json:"segments,omitempty"`
}

func newJSONPlaylist() *jsonPlaylist {
	return &jsonPlaylist{
		name:     fmt.Sprintf("playlist_%d.json", time.Now().Unix()),
		Segments: make(map[string][]jsonSeg),
	}
}

func (jpl *jsonPlaylist) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string,
	duration float64) {

	durationMs := uint64(duration * 1000)
	if profile.Name == "source" {
		jpl.DurationMs += durationMs
	}
	jpl.Segments[profile.Name] = append(jpl.Segments[profile.Name], jsonSeg{
		URI:        uri,
		DurationMS: durationMs,
		SeqNo:      seqNo,
	})
}

// NewBasicPlaylistManager create new BasicPlaylistManager struct
func NewBasicPlaylistManager(manifestID ManifestID,
	storageSession, recordSession drivers.OSSession) *BasicPlaylistManager {

	bplm := &BasicPlaylistManager{
		storageSession: storageSession,
		recordSession:  recordSession,
		manifestID:     manifestID,
		masterPList:    m3u8.NewMasterPlaylist(),
		mediaLists:     make(map[string]*m3u8.MediaPlaylist),
		mapSync:        &sync.RWMutex{},
	}
	if recordSession != nil {
		bplm.jsonList = newJSONPlaylist()
	}
	return bplm
}

func (mgr *BasicPlaylistManager) ManifestID() ManifestID {
	return mgr.manifestID
}

func (mgr *BasicPlaylistManager) Cleanup() {
	mgr.storageSession.EndSession()
}

func (mgr *BasicPlaylistManager) GetOSSession() drivers.OSSession {
	return mgr.storageSession
}

func (mgr *BasicPlaylistManager) GetRecordOSSession() drivers.OSSession {
	return mgr.recordSession
}

func (mgr *BasicPlaylistManager) FlushRecord() {
	if mgr.recordSession != nil {
		b, err := json.Marshal(mgr.jsonList)
		if err != nil {
			glog.Error("Error encoding playlist: ", err)
			return
		}
		mgr.recordSession.SaveData(mgr.jsonList.name, b, nil)
	}
}

func (mgr *BasicPlaylistManager) getPL(rendition string) *m3u8.MediaPlaylist {
	mgr.mapSync.RLock()
	mpl := mgr.mediaLists[rendition]
	mgr.mapSync.RUnlock()
	return mpl
}

func (mgr *BasicPlaylistManager) getOrCreatePL(profile *ffmpeg.VideoProfile) (*m3u8.MediaPlaylist, error) {
	mgr.mapSync.Lock()
	defer mgr.mapSync.Unlock()
	if pl, ok := mgr.mediaLists[profile.Name]; ok {
		return pl, nil
	}
	mpl, err := m3u8.NewMediaPlaylist(LIVE_LIST_LENGTH, LIVE_LIST_LENGTH)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	mgr.mediaLists[profile.Name] = mpl
	vParams := ffmpeg.VideoProfileToVariantParams(*profile)
	url := fmt.Sprintf("%v/%v.m3u8", mgr.manifestID, profile.Name)
	mgr.masterPList.Append(url, mpl, vParams)
	return mpl, nil
}

func (mgr *BasicPlaylistManager) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string,
	duration float64) error {

	mpl, err := mgr.getOrCreatePL(profile)
	if err != nil {
		return err
	}
	mseg := newMediaSegment(uri, duration)
	if mpl.Count() >= mpl.WinSize() {
		mpl.Remove()
	}
	if mpl.Count() == 0 {
		mpl.SeqNo = mseg.SeqId
	}
	if mgr.jsonList != nil {
		mgr.jsonList.InsertHLSSegment(profile, seqNo, uri, duration)
	}

	return mpl.InsertSegment(seqNo, mseg)
}

// GetHLSMasterPlaylist ..
func (mgr *BasicPlaylistManager) GetHLSMasterPlaylist() *m3u8.MasterPlaylist {
	return mgr.masterPList
}

// GetHLSMediaPlaylist ...
func (mgr *BasicPlaylistManager) GetHLSMediaPlaylist(rendition string) *m3u8.MediaPlaylist {
	return mgr.getPL(rendition)
}

func newMediaSegment(uri string, duration float64) *m3u8.MediaSegment {
	return &m3u8.MediaSegment{
		URI:      uri,
		Duration: duration,
	}
}
