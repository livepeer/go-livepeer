// Package drivers abstracts different object storages, such as local, s3
package drivers

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

// NodeStorage is current node's primary driver
var NodeStorage OSDriver

// RecordStorage is current node's "stream recording" driver
var RecordStorage OSDriver

// Testing indicates that test is running
var Testing bool

// TestMemoryStorages used for testing purposes
var TestMemoryStorages map[string]*MemoryOS
var testMemoryStoragesLock = &sync.Mutex{}

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
	Size         *int64
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

	SaveData(ctx context.Context, name string, data io.Reader, meta map[string]string, timeout time.Duration) (string, error)
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

func GetSegmentData(ctx context.Context, uri string) ([]byte, error) {
	return getSegmentDataHTTP(ctx, uri)
}

// PrepareOSURL used for resolving files when necessary and turning into a URL. Don't use
// this when the URL comes from untrusted sources e.g. AuthWebhookUrl.
func PrepareOSURL(input string) (string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", err
	}
	if u.Scheme == "gs" {
		m, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", err
		}
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
		if !ok {
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
		region := "ignored"
		base, bucket := path.Split(u.Path)
		if len(base) > 1 && base[len(base)-1] == '/' {
			base = base[:len(base)-1]
			_, region = path.Split(base)
		}
		hosturl, err := url.Parse(input)
		if err != nil {
			return nil, err
		}
		hosturl.User = nil
		hosturl.Scheme = scheme
		hosturl.Path = ""
		pw, ok := u.User.Password()
		if !ok {
			return nil, fmt.Errorf("password is required with s3:// OS")
		}
		return NewCustomS3Driver(hosturl.String(), bucket, region, u.User.Username(), pw, useFullAPI), nil
	}
	if u.Scheme == "gs" {
		file := u.User.Username()
		return NewGoogleDriver(u.Host, file, useFullAPI)
	}
	if u.Scheme == "memory" && Testing {
		testMemoryStoragesLock.Lock()
		if TestMemoryStorages == nil {
			TestMemoryStorages = make(map[string]*MemoryOS)
		}
		os, ok := TestMemoryStorages[u.Host]
		if !ok {
			os = NewMemoryDriver(nil)
			TestMemoryStorages[u.Host] = os
		}
		testMemoryStoragesLock.Unlock()
		return os, nil
	}
	return nil, fmt.Errorf("unrecognized OS scheme: %s", u.Scheme)
}

// SaveRetried tries to SaveData specified number of times
func SaveRetried(ctx context.Context, sess OSSession, name string, data []byte, meta map[string]string, retryCount int) (string, error) {
	if retryCount < 1 {
		return "", fmt.Errorf("invalid retry count %d", retryCount)
	}
	var uri string
	var err error
	for i := 0; i < retryCount; i++ {
		uri, err = sess.SaveData(ctx, name, bytes.NewReader(data), meta, 0)
		if err == nil {
			return uri, err
		}
	}
	return uri, err
}

var httpc = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   common.HTTPTimeout / 2,
}

func getSegmentDataHTTP(ctx context.Context, uri string) ([]byte, error) {
	clog.V(common.VERBOSE).Infof(ctx, "Downloading uri=%s", uri)
	started := time.Now()
	resp, err := httpc.Get(uri)
	if err != nil {
		clog.Errorf(ctx, "Error getting HTTP uri=%s err=%q", uri, err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		clog.Errorf(ctx, "Non-200 response for status=%v uri=%s", resp.Status, uri)
		return nil, fmt.Errorf(resp.Status)
	}
	body, err := common.ReadAtMost(resp.Body, common.MaxSegSize)
	if err != nil {
		clog.Errorf(ctx, "Error reading body uri=%s err=%q", uri, err)
		return nil, err
	}
	took := time.Since(started)
	clog.V(common.VERBOSE).Infof(ctx, "Downloaded uri=%s dur=%s bytes=%d", uri, took, len(body))
	return body, nil
}
