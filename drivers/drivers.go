// Package drivers abstracts different object storages, such as local, s3, ipfs
package drivers

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

// Current node's primary driver
var NodeStorage OSDriver

// OSDriver common interface for Object Storage
type OSDriver interface {
	NewSession(path string) OSSession
}

type OSSession interface {
	SaveData(name string, data []byte) (string, error)
	EndSession()

	// Info in order to have this session used via RPC
	GetInfo() *net.OSInfo

	// Indicates whether data may be external to this node
	IsExternal() bool
}

// NewDriver returns new session based on OSInfo received from the network
func NewSession(info *net.OSInfo) OSSession {
	if info == nil {
		return nil
	}
	switch info.StorageType {
	case net.OSInfo_IPFS:
		return newIPFSSession()
	case net.OSInfo_S3:
		return newS3Session(info.S3Info)
	}
	return nil
}

func IsOwnExternal(uri string) bool {
	return IsOwnStorageS3(uri)
}

func GetSegmentData(uri string) ([]byte, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("Invalid URI")
	}
	if parsed.Scheme == "ipfs" {
		return GetSegmentDataIpfs(uri)
	}
	return getSegmentDataHTTP(uri)
}

var httpc = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

func getSegmentDataHTTP(uri string) ([]byte, error) {
	glog.V(common.VERBOSE).Info("Downloading ", uri)
	resp, err := httpc.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(`Error reading body:` + err.Error())
		return nil, err
	}
	glog.V(common.VERBOSE).Info("Downloaded ", uri)
	return body, nil
}
