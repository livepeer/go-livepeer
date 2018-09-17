package drivers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
)

// LocalOSinstance instance to be acessed by media server's http hanlders
var LocalOSinstance *localOS

// DataDir base dir to save data to
var DataDir string

var dataCacheLen = 12

type localOS struct {
	baseURL      string
	sessions     map[uint64]*localOSSession
	sessionsLock sync.Mutex
}

type localOSSession struct {
	storage    *localOS
	jobID      int64
	nonce      uint64
	manifestID string
	baseURL    string
	dCache     map[string]*dataCache
	dLock      sync.Mutex
	save       bool
	saveDir    string
}

func newLocalDriver(info *net.LocalOSInfo) OSDriver {
	if LocalStorage != nil {
		panic("Should only be one instance of LocalStorage driver")
	}
	baseURL := ""
	if info != nil {
		baseURL = info.BaseURL
	}
	od := &localOS{
		baseURL:      baseURL,
		sessions:     make(map[uint64]*localOSSession),
		sessionsLock: sync.Mutex{},
	}
	LocalOSinstance = od
	return od
}

func (ostore *localOS) GetData(name string) []byte {
	ostore.sessionsLock.Lock()
	ostore.sessionsLock.Unlock()
	for _, dc := range ostore.sessions {
		data := dc.GetData(name)
		if data != nil {
			return data
		}
	}
	return nil
}

func (ostore *localOS) IsExternal() bool {
	return false
}

func (ostore *localOS) StartSession(jobID int64, manifestID string, nonce uint64) OSSession {
	ostore.sessionsLock.Lock()
	defer ostore.sessionsLock.Unlock()
	if es, ok := ostore.sessions[nonce]; ok {
		return es
	}
	s := localOSSession{
		storage: ostore,
		jobID:      jobID,
		nonce:      nonce,
		manifestID: manifestID,
		baseURL:    ostore.baseURL,
		dCache:     make(map[string]*dataCache),
		dLock:      sync.Mutex{},
		save:       LocalSaveMode,
		saveDir:    filepath.Join(DataDir, "saved", fmt.Sprintf("%05d_%s_%d", jobID, manifestID, nonce)),
	}
	ostore.sessions[nonce] = &s

	if s.save {
		if _, err := os.Stat(s.saveDir); os.IsNotExist(err) {
			glog.Infof("Creating data dir: %v", s.saveDir)
			if err = os.MkdirAll(s.saveDir, 0755); err != nil {
				glog.Errorf("Error creating datadir: %v", err)
			}
		}
	}

	return &s
}

func (ostore *localOS) removeSession(session *localOSSession) {
	ostore.sessionsLock.Lock()
	defer ostore.sessionsLock.Unlock()
	delete(ostore.sessions, session.nonce)
}

func (session *localOSSession) EndSession() {
	session.storage.removeSession(session)
}

func (session *localOSSession) IsOwnStorage(turi *net.TypedURI) bool {
	if turi.Storage != "local" {
		return false
	}
	if session.baseURL != "" && strings.Index(turi.Uri, session.baseURL) == 0 {
		return true
	}
	return false
}

func (session *localOSSession) GetInfo() net.OSInfo {
	info := net.OSInfo{}
	return info
}

func (session *localOSSession) GetData(name string) []byte {
	session.dLock.Lock()
	defer session.dLock.Unlock()
	for _, cache := range session.dCache {
		for _, d := range cache.cache {
			if d.name == name {
				return d.data
			}
		}
	}
	return nil
}

func (session *localOSSession) getCacheForStream(streamID string) *dataCache {
	session.dLock.Lock()
	defer session.dLock.Unlock()
	sc, ok := session.dCache[streamID]
	if !ok {
		sc = newDataCache(dataCacheLen)
		session.dCache[streamID] = sc
	}
	return sc
}

func (session *localOSSession) getAbsoluteURL(name string) string {
	if session.baseURL != "" {
		return session.baseURL + "/stream/" + name
	}
	return name
}

func (session *localOSSession) SaveData(streamID, name string, data []byte) (*net.TypedURI, string, error) {
	dc := session.getCacheForStream(streamID)
	dc.Insert(name, data)
	abs := session.getAbsoluteURL(name)
	turl := &net.TypedURI{
		Storage:       "local",
		StreamID:      streamID,
		Uri:           abs,
		UriInManifest: name,
	}
	if session.save {
		fName := filepath.Join(session.saveDir, name)
		err := ioutil.WriteFile(fName, data, 0644)
		if err != nil {
			glog.Error(err)
		}
	}
	return turl, abs, nil
}

type dataCache struct {
	cacheLen int
	cache    []*dataCacheItem
}

type dataCacheItem struct {
	name     string
	data     []byte
}

func newDataCache(len int) *dataCache {
	return &dataCache{cacheLen: len, cache: make([]*dataCacheItem, 0)}
}

func (dc *dataCache) Insert(name string, data []byte) {
	// replace existing item
	for _, item := range dc.cache {
		if item.name == name {
			item.data = data
			return
		}
	}
	if len(dc.cache) >= dc.cacheLen {
		dc.cache = dc.cache[1:]
	}
	item := &dataCacheItem{name: name, data: data}
	dc.cache = append(dc.cache, item)
}

// GetData returns mime type and data
func (dc *dataCache) GetData(name string) []byte {
	for _, s := range dc.cache {
		if s.name == name {
			return s.data
		}
	}
	return nil
}
