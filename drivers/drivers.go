// Package drivers abstracts different object storages, such as local, s3
package drivers

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

// NodeStorage is current node's primary driver
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

// NewSession returns new session based on OSInfo received from the network
func NewSession(info *net.OSInfo) OSSession {
	if info == nil {
		return nil
	}
	switch info.StorageType {
	case net.OSInfo_S3:
		return newS3Session(info.S3Info)
	case net.OSInfo_GOOGLE:
		return newGSSession(info.S3Info)
	}
	return nil
}

func IsOwnExternal(uri string) bool {
	return IsOwnStorageS3(uri) || IsOwnStorageGS(uri)
}

func GetSegmentData(uri string) ([]byte, error) {
	return getSegmentDataHTTP(uri)
}

func ParseOSURL(input string) (OSDriver, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "s3" {
		pw, ok := u.User.Password()
		if ok == false {
			return nil, fmt.Errorf("password is required with s3:// OS")
		}
		base := path.Base(u.Path)
		return NewS3Driver(u.Host, base, u.User.Username(), pw), nil
	}
	return nil, fmt.Errorf("unrecognized OS scheme: %s", u.Scheme)
	// return NewS3Driver("a", "b", "c", "d"), nil
}

var httpc = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   common.HTTPTimeout / 2,
}

func getSegmentDataHTTP(uri string) ([]byte, error) {
	glog.V(common.VERBOSE).Infof("Downloading uri=%s", uri)
	started := time.Now()
	resp, err := httpc.Get(uri)
	if err != nil {
		glog.Errorf("Error getting HTTP uri=%s err=%v", uri, err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		glog.Errorf("Non-200 response for status=%v uri=%s", resp.Status, uri)
		return nil, fmt.Errorf(resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading body uri=%s err=%v", uri, err)
		return nil, err
	}
	took := time.Since(started)
	glog.V(common.VERBOSE).Infof("Downloaded uri=%s dur=%s", uri, took)
	return body, nil
}
