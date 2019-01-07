package drivers

import (
	"fmt"
	"path"
	"sync"

	"github.com/livepeer/go-livepeer/net"
)

var dataCacheLen = 12

type MemoryOS struct {
	baseURI  string
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

func NewMemoryDriver(baseURI string) *MemoryOS {
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

func (ostore *MemoryOS) GetSession(path string) *MemorySession {
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

func (ostore *MemorySession) GetData(name string) []byte {

	path, file := path.Split(name)

	ostore.dLock.RLock()
	defer ostore.dLock.RUnlock()

	if cache, ok := ostore.dCache[path]; ok {
		return cache.GetData(file)
	}
	return nil
}

func (ostore *MemorySession) IsExternal() bool {
	return false
}

func (ostore *MemorySession) GetInfo() *net.OSInfo {
	return nil
}

func (ostore *MemorySession) SaveData(name string, data []byte) (string, error) {

	path, file := path.Split(ostore.getAbsolutePath(name))

	ostore.dLock.Lock()
	defer ostore.dLock.Unlock()

	if ostore.ended {
		return "", fmt.Errorf("Session ended")
	}

	dc := ostore.getCacheForStream(path)
	dc.Insert(file, data)

	return ostore.getAbsoluteURI(name), nil
}

func (ostore *MemorySession) getCacheForStream(streamID string) *dataCache {
	sc, ok := ostore.dCache[streamID]
	if !ok {
		sc = newDataCache(dataCacheLen)
		ostore.dCache[streamID] = sc
	}
	return sc
}

func (ostore *MemorySession) getAbsolutePath(name string) string {
	return path.Clean(ostore.path + "/" + name)
}

func (ostore *MemorySession) getAbsoluteURI(name string) string {
	name = "/stream/" + ostore.getAbsolutePath(name)
	if ostore.os.baseURI != "" {
		return ostore.os.baseURI + name
	}
	return name
}

type dataCache struct {
	cacheLen int
	cache    []*dataCacheItem
}

type dataCacheItem struct {
	name string
	data []byte
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

func (dc *dataCache) GetData(name string) []byte {
	for _, s := range dc.cache {
		if s.name == name {
			return s.data
		}
	}
	return nil
}
