package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-tools/drivers"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/m3u8"
)

const LIVE_LIST_LENGTH uint = 6

const (
	jsonPlaylistRotationInterval = 60 * 60 * 1000 // 1 hour (in ms)
	jsonPlaylistMaxRetries       = 30
	JsonPlaylistInitialTimeout   = 5 * time.Second
	JsonPlaylistMaxTimeout       = 120 * time.Second
)

var JsonPlaylistQuitTimeout = 60 * time.Second

// PlaylistManager manages playlists and data for one video stream, backed by one object storage.
type PlaylistManager interface {
	ManifestID() ManifestID
	// Implicitly creates master and media playlists
	// Inserts in media playlist given a link to a segment
	InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64) error

	InsertHLSSegmentJSON(profile *ffmpeg.VideoProfile, seqNo uint64, uri string, duration float64)

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
	masterPList        *m3u8.MasterPlaylist
	mediaLists         map[string]*m3u8.MediaPlaylist
	mapSync            *sync.RWMutex
	jsonList           *JsonPlaylist
	jsonListWriteQueue *drivers.OverwriteQueue
	jsonListSync       *sync.Mutex
}

type jsonSeg struct {
	SeqNo         uint64 `json:"seq_no,omitempty"`
	URI           string `json:"uri,omitempty"`
	DurationMs    uint64 `json:"duration_ms,omitempty"`
	discontinuity bool
}

func (js *jsonSeg) GetDiscontinuity() bool {
	return js.discontinuity
}

type JsonPlaylist struct {
	name       string
	DurationMs uint64               `json:"duration_ms,omitempty"` // total duration of the saved sagments
	Tracks     []JsonMediaTrack     `json:"tracks,omitempty"`
	Segments   map[string][]jsonSeg `json:"segments,omitempty"`
}

type JsonMediaTrack struct {
	Name       string `json:"name,omitempty"`
	Bandwidth  uint32 `json:"bandwidth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

func NewJSONPlaylist() *JsonPlaylist {
	return &JsonPlaylist{
		name:     fmt.Sprintf("playlist_%d.json", time.Now().UnixNano()),
		Segments: make(map[string][]jsonSeg),
	}
}

// AddMaster adds data about tracks
func (jpl *JsonPlaylist) AddMaster(ajpl *JsonPlaylist) {
	for _, track := range ajpl.Tracks {
		if !jpl.hasTrack(track.Name) {
			jpl.Tracks = append(jpl.Tracks, track)
		}
	}
}

// AddSegmentsToMPL adds segments to the MediaPlaylist
func (jpl *JsonPlaylist) AddSegmentsToMPL(manifestIDs []string, trackName string, mpl *m3u8.MediaPlaylist, extURL string) {
	for _, seg := range jpl.Segments[trackName] {
		// make relative URL from absolute one
		uri := seg.URI
		mindex, manifestIDlen := indexOf(uri, manifestIDs)
		if mindex != -1 {
			// If extURL was specified we will put absolute URL to the segment into manifest
			// extURL points to the root of object store, so we should take part of the 'uri'
			// which contains manifestID.
			// If extURL is not specified then we're serving relative URL to the segment,
			// and address at which manifest is served already contains manfiestID
			// (broadcaster.com/recordings/manifestID/index.m3u8), so we're taking
			// part of the 'uri' after the manifestID
			if extURL != "" {
				uri = common.JoinURL(extURL, uri[mindex:])
			} else {
				uri = uri[mindex+manifestIDlen+1:]
			}
		}
		mseg := &m3u8.MediaSegment{
			URI:           uri,
			Duration:      float64(seg.DurationMs) / 1000.0,
			Discontinuity: seg.discontinuity,
		}
		mpl.InsertSegment(seg.SeqNo, mseg)
	}
}

func (jpl *JsonPlaylist) hasTrack(trackName string) bool {
	for _, track := range jpl.Tracks {
		if track.Name == trackName {
			return true
		}
	}
	return false
}

// AddDiscontinuedTrack appends all segments for specified rendition, mark first one as discontinued
func (jpl *JsonPlaylist) AddDiscontinuedTrack(ajpl *JsonPlaylist, trackName string) {
	curSegs := jpl.Segments[trackName]
	var lastSeq uint64
	var disc bool
	if len(curSegs) > 0 {
		lastSeq = curSegs[len(curSegs)-1].SeqNo
		lastSeq++
		disc = true
	}
	for i, seg := range ajpl.Segments[trackName] {
		if i == 0 {
			seg.discontinuity = disc
		}
		seg.SeqNo += lastSeq
		curSegs = append(curSegs, seg)
	}
	jpl.Segments[trackName] = curSegs
}

// AddTrack adds segments data for specified rendition
func (jpl *JsonPlaylist) AddTrack(ajpl *JsonPlaylist, trackName string) {
	curSegs := jpl.Segments[trackName]
	var lastSeq uint64
	if len(curSegs) > 0 {
		lastSeq = curSegs[len(curSegs)-1].SeqNo
	}

	for _, seg := range ajpl.Segments[trackName] {
		needSort := false
		if seg.SeqNo > lastSeq {
			curSegs = append(curSegs, seg)
		} else {
			i := sort.Search(len(curSegs), func(i int) bool {
				return curSegs[i].SeqNo >= seg.SeqNo
			})
			if i < len(curSegs) && curSegs[i].SeqNo == seg.SeqNo {
				// x is present at data[i]
			} else {
				// x is not present in data,
				// but i is the index where it would be inserted.
				if i < len(curSegs) {
					needSort = true
				}
				curSegs = append(curSegs, seg)
			}
		}
		if needSort {
			sort.Slice(curSegs, func(i, j int) bool {
				return curSegs[i].SeqNo < curSegs[j].SeqNo
			})
		}
		lastSeq = curSegs[len(curSegs)-1].SeqNo
	}
	jpl.Segments[trackName] = curSegs
}

func (jpl *JsonPlaylist) InsertHLSSegment(profile *ffmpeg.VideoProfile, seqNo uint64, uri string,
	duration float64) {

	durationMs := uint64(duration * 1000)
	if profile.Name == "source" {
		jpl.DurationMs += durationMs
	}
	if _, has := jpl.Segments[profile.Name]; !has {
		vParams := ffmpeg.VideoProfileToVariantParams(*profile)
		jpl.Tracks = append(jpl.Tracks, JsonMediaTrack{
			Name:       profile.Name,
			Bandwidth:  vParams.Bandwidth,
			Resolution: vParams.Resolution,
		})
	}
	jpl.Segments[profile.Name] = append(jpl.Segments[profile.Name], jsonSeg{
		URI:        uri,
		DurationMs: durationMs,
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
		bplm.jsonList = NewJSONPlaylist()
		bplm.jsonListSync = &sync.Mutex{}
		bplm.makeNewOverwriteQueue()
	}
	return bplm
}

func (mgr *BasicPlaylistManager) makeNewOverwriteQueue() {
	if mgr.jsonListWriteQueue != nil {
		mgr.jsonListWriteQueue.StopAfter(JsonPlaylistQuitTimeout)
	}
	mgr.jsonListWriteQueue = drivers.NewOverwriteQueue(mgr.recordSession, mgr.jsonList.name,
		fmt.Sprintf("json playlist for manifestId=%s", mgr.manifestID),
		jsonPlaylistMaxRetries, JsonPlaylistInitialTimeout, JsonPlaylistMaxTimeout)
}

func (mgr *BasicPlaylistManager) ManifestID() ManifestID {
	return mgr.manifestID
}

func (mgr *BasicPlaylistManager) Cleanup() {
	if mgr.storageSession != nil {
		mgr.storageSession.EndSession()
	}
	if mgr.jsonListWriteQueue != nil {
		mgr.jsonListWriteQueue.StopAfter(JsonPlaylistQuitTimeout)
	}
}

func (mgr *BasicPlaylistManager) GetOSSession() drivers.OSSession {
	return mgr.storageSession
}

func (mgr *BasicPlaylistManager) GetRecordOSSession() drivers.OSSession {
	return mgr.recordSession
}

func (mgr *BasicPlaylistManager) FlushRecord() {
	if mgr.recordSession != nil {
		mgr.jsonListSync.Lock()
		defer mgr.jsonListSync.Unlock()
		b, err := json.Marshal(mgr.jsonList)
		if err != nil {
			glog.Error("Error encoding playlist: ", err)
			return
		}
		go mgr.jsonListWriteQueue.Save(b)
		if mgr.jsonList.DurationMs > jsonPlaylistRotationInterval {
			mgr.jsonList = NewJSONPlaylist()
			mgr.makeNewOverwriteQueue()
		}
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

func (mgr *BasicPlaylistManager) InsertHLSSegmentJSON(profile *ffmpeg.VideoProfile, seqNo uint64, uri string,
	duration float64) {

	if mgr.jsonList != nil {
		mgr.jsonListSync.Lock()
		mgr.jsonList.InsertHLSSegment(profile, seqNo, uri, duration)
		mgr.jsonListSync.Unlock()
	}
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

// indexOf finds index of one of substrings
// returns index and length of substring
func indexOf(str string, substrs []string) (int, int) {
	for _, sub := range substrs {
		mindex := strings.Index(str, sub)
		if mindex != -1 {
			return mindex, len(sub)
		}
	}
	return -1, 0
}
