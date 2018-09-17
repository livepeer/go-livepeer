// Package drivers abstracts different object storages, such as local, s3, ipfs
package drivers

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
)

// Storages configured at start (used by broadcaster)
var Storages []OSDriver

// ExternalStorage own external storage, if configured
var ExternalStorage OSDriver

// LocalStorage single instance of local storage
var LocalStorage OSDriver

// LocalSaveMode if true local storage will be saving all the segments into special directory
var LocalSaveMode bool

// OSDriver common interface for Object Storage
type OSDriver interface {
	// StartSession session is unique identified by nonce, should return existing session
	StartSession(jobID int64, manifestID string, nonce uint64) OSSession
	// IsExternal
	IsExternal() bool
}

// OSSession ...
type OSSession interface {
	SaveData(streamID, name string, data []byte) (*net.TypedURI, string, error)
	// IsOwnStorage
	IsOwnStorage(turi *net.TypedURI) bool
	// GetInfo
	GetInfo() net.OSInfo
	// EndSession
	EndSession()
}

// AddStorageInstance adds storage driver instance to the list of storages used by broadcaster
func AddStorageInstance(driver OSDriver) {
	Storages = append(Storages, driver)
}

// NewDriver returns new driver
func NewDriver(storageType string, info *net.OSInfo) OSDriver {
	switch storageType {
	case "ipfs":
		return newIPSFDriver()
	case "s3":
		return newS3Driver(info.S3Info)
	case "local":
		var li *net.LocalOSInfo
		if info != nil {
			li = info.LocalInfo
		}
		return newLocalDriver(li)
	}
	panic("unknown driver")
}

func IsExternal(turi *net.TypedURI) bool {
	switch turi.Storage {
	case "s3":
		return true
	}
	return false
}

func GetSegmentData(turi *net.TypedURI) ([]byte, error) {
	switch turi.Storage {
	case "ipfs":
		return GetSegmentDataIpfs(turi)
	case "s3", "local":
		return getSegmentDataHTTP(turi)
	}
	return nil, fmt.Errorf("UnknownStrorageProvider")
}

func getSegmentDataHTTP(turi *net.TypedURI) ([]byte, error) {
	resp, err := http.Get(turi.Uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(`Error reading body:` + err.Error())
		return nil, err
	}
	return body, nil
}
