package drivers

import (
	"context"
	"fmt"
	"io"
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

func (ostore *MemorySession) ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error) {

	return nil, fmt.Errorf("Not implemented")
}

func (ostore *MemorySession) ReadData(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {

	return nil, nil, fmt.Errorf("Not implemented")
}

// GetData returns the cached data for a name.
//
// A name can be an absolute or relative URI.
// An absolute URI has the following format:
// - ostore.os.baseURI + /stream/ + ostore.path + path + file
// The following are valid relative URIs:
// - /stream/ + ostore.path + path + file (if ostore.os.baseURI is empty)
// - ostore.path + path + file
func (ostore *MemorySession) GetData(name string) []byte {
	// Since the memory cache uses the path as the key for fetching data we make sure that
	// ostore.os.baseURI and /stream/ are stripped before splitting into a path and a filename
	prefix := ""
	if ostore.os.baseURI != nil {
		prefix += ostore.os.baseURI.String()
	}
	prefix += "/stream/"

	path, file := path.Split(strings.TrimPrefix(name, prefix))

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

func (ostore *MemorySession) IsOwn(url string) bool {
	return strings.HasPrefix(url, ostore.path)
}

func (ostore *MemorySession) GetInfo() *net.OSInfo {
	return nil
}

func (ostore *MemorySession) SaveData(name string, data []byte, meta map[string]string) (string, error) {
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
	if ostore.os.baseURI != nil {
		return ostore.os.baseURI.String() + name
	}
	return name
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
