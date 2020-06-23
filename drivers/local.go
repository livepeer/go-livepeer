package drivers

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/livepeer/go-livepeer/net"
)

var dataCacheLen = 12

type MemoryOS struct {
	baseURI  *url.URL
	sessions map[string]*MemorySession
	lock     sync.RWMutex
}

type MemorySession struct {
	os     *MemoryOS
	path   string
	ended  bool
	dCache map[string]*dataCache
	dLock  sync.RWMutex
}

func NewMemoryDriver(baseURI *url.URL) *MemoryOS {
	return &MemoryOS{
		baseURI:  baseURI,
		sessions: make(map[string]*MemorySession),
		lock:     sync.RWMutex{},
	}
}

func (ostore *MemoryOS) NewSession(path string) OSSession {
	ostore.lock.Lock()
	defer ostore.lock.Unlock()
	if session, ok := ostore.sessions[path]; ok {
		return session
	}
	session := &MemorySession{
		os:     ostore,
		path:   path,
		dCache: make(map[string]*dataCache),
		dLock:  sync.RWMutex{},
	}
	ostore.sessions[path] = session
	return session
}

func (ostore *MemoryOS) getSession(path string) *MemorySession {
	ostore.lock.Lock()
	defer ostore.lock.Unlock()
	if session, ok := ostore.sessions[path]; ok {
		return session
	}
	return nil
}

// EndSession clears memory cache
func (ostore *MemorySession) EndSession() {
	ostore.dLock.Lock()
	ostore.ended = true
	for k := range ostore.dCache {
		delete(ostore.dCache, k)
	}
	ostore.dLock.Unlock()

	ostore.os.lock.Lock()
	delete(ostore.os.sessions, ostore.path)
	ostore.os.lock.Unlock()
}

func (os *MemoryOS) SaveData(name string, data []byte) error {
	// TODO should this be implemented?
	return nil
}

// GetData returns the cached data for a name.
//
// An name has the following format:
//   sessionName / renditionName / fileName
// The session is looked up based on the sessionName.
// Within the session lookup, renditionName / fileName is used.
func (os *MemoryOS) GetData(name string) ([]byte, error) {

	parts := strings.SplitN(name, "/", 2)
	if len(parts) <= 0 {
		return nil, fmt.Errorf("memory os: invalid path")
	}

	// We index the session by the first entry of the path, eg
	// <session>/<more-path>/<data>
	sess := os.getSession(parts[0])
	if sess == nil {
		return nil, fmt.Errorf("memory os: invalid session")
	}
	return sess.GetData(parts[1])
}

func (ostore *MemorySession) IsExternal() bool {
	return false
}

func (ostore *MemorySession) GetInfo() *net.OSInfo {
	return nil
}

func (ostore *MemorySession) SaveData(name string, data []byte) (string, error) {
	path, file := path.Split(name)

	ostore.dLock.Lock()
	defer ostore.dLock.Unlock()

	if ostore.ended {
		return "", fmt.Errorf("memory os: session ended")
	}

	dc := ostore.getCacheForStream(path)
	dc.Insert(file, data)

	return name, nil
}

func (ostore *MemorySession) GetData(name string) ([]byte, error) {
	// Since the memory cache uses the path as the key for fetching data we make sure that
	// ostore.os.baseURI and /stream/ are stripped before splitting into a path and a filename

	path, file := path.Split(name)

	ostore.dLock.RLock()
	defer ostore.dLock.RUnlock()

	if cache, ok := ostore.dCache[path]; ok {
		return cache.GetData(file), nil
	}
	return nil, fmt.Errorf("memory os: object does not exist")
}

func (ostore *MemorySession) getCacheForStream(streamID string) *dataCache {
	sc, ok := ostore.dCache[streamID]
	if !ok {
		sc = newDataCache(dataCacheLen)
		ostore.dCache[streamID] = sc
	}
	return sc
}

type dataCache struct {
	cacheLen int
	nextFree int
	cache    []dataCacheItem
}

type dataCacheItem struct {
	name string
	data []byte
}

func newDataCache(len int) *dataCache {
	return &dataCache{cacheLen: len, cache: make([]dataCacheItem, len)}
}

func (dc *dataCache) Insert(name string, data []byte) {
	// replace existing item
	for i, item := range dc.cache {
		if item.name == name {
			dc.cache[i] = dataCacheItem{name: name, data: data}
			return
		}
	}
	dc.cache[dc.nextFree].name = name
	dc.cache[dc.nextFree].data = data
	dc.nextFree++
	if dc.nextFree >= dc.cacheLen {
		dc.nextFree = 0
	}
}

func (dc *dataCache) GetData(name string) []byte {
	for _, s := range dc.cache {
		if s.name == name {
			return s.data
		}
	}
	return nil
}
