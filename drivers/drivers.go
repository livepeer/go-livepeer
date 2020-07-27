// Package drivers abstracts different object storages, such as local, s3
package drivers

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

// ErrNoNextPage indicates that there is no next page in ListFiles
var ErrNoNextPage = fmt.Errorf("no next page")

type FileInfo struct {
	Name         string
	ETag         string
	LastModified time.Time
	Size         int64
}

type FileInfoReader struct {
	FileInfo
	Metadata map[string]string
	Body     io.ReadCloser
}

type PageInfo interface {
	Files() []FileInfo
	Directories() []string
	HasNextPage() bool
	NextPage() (PageInfo, error)
}

type OSSession interface {
	OS() OSDriver

	SaveData(name string, data []byte, meta map[string]string) (string, error)
	EndSession()

	// Info in order to have this session used via RPC
	GetInfo() *net.OSInfo

	// Indicates whether data may be external to this node
	IsExternal() bool

	// Indicates whether this is the correct OS for a given URL
	IsOwn(url string) bool

	// ListFiles return list of files
	ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error)

	ReadData(ctx context.Context, name string) (*FileInfoReader, error)
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

func GetSegmentData(uri string) ([]byte, error) {
	return getSegmentDataHTTP(uri)
}

// PrepareOSURL used for resolving files when necessary and turning into a URL. Don't use
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

// ParseOSURL returns the correct OS for a given OS url
func ParseOSURL(input string, useFullAPI bool) (OSDriver, error) {
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
		return NewS3Driver(u.Host, base, u.User.Username(), pw, useFullAPI), nil
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
		hosturl.Path = ""
		pw, ok := u.User.Password()
		if ok == false {
			return nil, fmt.Errorf("password is required with s3:// OS")
		}
		return NewCustomS3Driver(hosturl.String(), bucket, u.User.Username(), pw, useFullAPI), nil
	}
	if u.Scheme == "gs" {
		file := u.User.Username()
		return NewGoogleDriver(u.Host, file, useFullAPI)
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
