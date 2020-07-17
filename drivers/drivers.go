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

// RecordStorage is current node's "stream recording" driver
var RecordStorage OSDriver

// OSDriver common interface for Object Storage
type OSDriver interface {
	NewSession(path string) OSSession
}

type OSSession interface {
	SaveData(name string, data []byte, meta map[string]string) (string, error)
	EndSession()

	// Info in order to have this session used via RPC
	GetInfo() *net.OSInfo

	// Indicates whether data may be external to this node
	IsExternal() bool

	// Indicates whether this is the correct OS for a given URL
	IsOwn(url string) bool
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

// Used for resolving files when necessary and turning into a URL. Don't use
// this when the URL comes from untrusted sources e.g. AuthWebhookUrl.
func PrepareOSURL(input string) (string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", err
	}
	if u.Scheme == "gs" {
		m, _ := url.ParseQuery(u.RawQuery)
		keyfiles, ok := m["keyfile"]
		if !ok {
			return u.String(), nil
		}

		keyfile := keyfiles[0]
		content, err := ioutil.ReadFile(keyfile)
		if err != nil {
			return "", err
		}
		u.User = url.User(string(content))
	}
	return u.String(), nil
}

// Return the correct OS for a given OS url
func ParseOSURL(input string, own bool) (OSDriver, error) {
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
		if own {
			if S3BUCKET != "" {
				return nil, fmt.Errorf("we already have our own bucket (%s) but we tried to add another one (%s)", S3BUCKET, base)
			}
			S3BUCKET = base
		}
		return NewS3Driver(u.Host, base, u.User.Username(), pw), nil
	}
	// custom s3-compatible store
	if u.Scheme == "s3+http" || u.Scheme == "s3+https" {
		scheme := "http"
		if u.Scheme == "s3+https" {
			scheme = "https"
		}
		_, bucket := path.Split(u.Path)
		hosturl, err := url.Parse(input)
		if err != nil {
			return nil, err
		}
		hosturl.User = nil
		hosturl.Scheme = scheme
		pw, ok := u.User.Password()
		if ok == false {
			return nil, fmt.Errorf("password is required with s3:// OS")
		}
		return NewCustomS3Driver(hosturl.String(), bucket, u.User.Username(), pw), nil
	}
	if u.Scheme == "gs" {
		file := u.User.Username()
		if own {
			if GSBUCKET != "" {
				return nil, fmt.Errorf("we already have our own gs bucket (%s) but we tried to add another one (%s)", GSBUCKET, u.Host)
			}
			GSBUCKET = u.Host
		}
		return NewGoogleDriver(u.Host, file)
	}
	return nil, fmt.Errorf("unrecognized OS scheme: %s", u.Scheme)
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
