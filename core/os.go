/*
Object store helper functions
*/
package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	gonet "net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
)

// DownloadData downloads data while refusing connections to addresses that
// reach the local host, including after DNS resolution or an HTTP redirect.
func DownloadData(ctx context.Context, uri string) ([]byte, error) {
	return downloadDataHTTP(ctx, uri, localhostBlockedHTTPClient)
}

// DownloadDataAllowLocalhost downloads data without blocking local host
// addresses. Only use this security opt-out when the URL comes from a trusted
// source and localhost access is required.
func DownloadDataAllowLocalhost(ctx context.Context, uri string) ([]byte, error) {
	return downloadDataHTTP(ctx, uri, httpc)
}

var errLocalhostDownload = errors.New("localhost downloads are blocked")

func rejectLocalhostDial(_ context.Context, _, address string, _ syscall.RawConn) error {
	host, _, err := gonet.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid resolved download address %q: %w", address, err)
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("invalid resolved download host %q: %w", host, err)
	}
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsUnspecified() {
		return fmt.Errorf("%w: %s", errLocalhostDownload, address)
	}

	return nil
}

var httpc = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   common.HTTPTimeout / 2,
}

var localhostBlockedHTTPClient = &http.Client{
	Transport: &http.Transport{
		DialContext:     (&gonet.Dialer{ControlContext: rejectLocalhostDial}).DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	Timeout: common.HTTPTimeout / 2,
}

// LocalhostBlockedHTTPClient returns the shared HTTP client that refuses
// connections to local host addresses. Callers must not mutate the client.
func LocalhostBlockedHTTPClient() *http.Client {
	return localhostBlockedHTTPClient
}

func FromNetOsInfo(os *net.OSInfo) *drivers.OSInfo {
	if os == nil {
		return nil
	}
	return &drivers.OSInfo{
		StorageType: drivers.OSInfo_StorageType(os.StorageType),
		S3Info:      FromNetS3Info(os.S3Info),
	}
}

func FromNetS3Info(storage *net.S3OSInfo) *drivers.S3OSInfo {
	if storage == nil {
		return nil
	}
	return &drivers.S3OSInfo{
		Host:       storage.Host,
		Key:        storage.Key,
		Policy:     storage.Policy,
		Signature:  storage.Signature,
		Credential: storage.Credential,
		XAmzDate:   storage.XAmzDate,
	}
}

func ToNetOSInfo(os *drivers.OSInfo) *net.OSInfo {
	if os == nil {
		return nil
	}
	return &net.OSInfo{
		StorageType: net.OSInfo_StorageType(os.StorageType),
		S3Info:      ToNetS3Info(os.S3Info),
	}
}

func ToNetS3Info(storage *drivers.S3OSInfo) *net.S3OSInfo {
	if storage == nil {
		return nil
	}
	return &net.S3OSInfo{
		Host:       storage.Host,
		Key:        storage.Key,
		Policy:     storage.Policy,
		Signature:  storage.Signature,
		Credential: storage.Credential,
		XAmzDate:   storage.XAmzDate,
	}
}

func downloadDataHTTP(ctx context.Context, uri string, client *http.Client) ([]byte, error) {
	clog.V(common.VERBOSE).Infof(ctx, "Downloading uri=%s", uri)
	started := time.Now()
	resp, err := client.Get(uri)
	if err != nil {
		clog.Errorf(ctx, "Error getting HTTP uri=%s err=%q", uri, err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		clog.Errorf(ctx, "Non-200 response for status=%v uri=%s", resp.Status, uri)
		return nil, fmt.Errorf("%v", resp.Status)
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
