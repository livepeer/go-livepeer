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
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
)

// DownloadData downloads data while refusing connections to non-public
// addresses (loopback, private, link-local, multicast, …) and rejecting
// URLs with unsafe schemes, embedded credentials, or path-traversal
// sequences. These checks run both on the original URL and after DNS
// resolution or any HTTP redirect.
func DownloadData(ctx context.Context, uri string) ([]byte, error) {
	if err := validateDownloadURL(uri); err != nil {
		return nil, err
	}
	return downloadDataHTTP(ctx, uri, ssrfBlockedHTTPClient)
}

// DownloadDataAllowLocalhost downloads data without blocking local host
// addresses. Only use this security opt-out when the URL comes from a trusted
// source and localhost access is required.
func DownloadDataAllowLocalhost(ctx context.Context, uri string) ([]byte, error) {
	return downloadDataHTTP(ctx, uri, httpc)
}

var (
	errLocalhostDownload = errors.New("localhost downloads are blocked")
	errPrivateDownload   = errors.New("private/internal address downloads are blocked")
	errUnsafeURL         = errors.New("unsafe download URL")
)

// validateDownloadURL performs pre-dial, pre-request validation on the raw URL
// string. It rejects non-HTTP(S) schemes, embedded credentials, path traversal,
// and fragment-only or opaque URIs.
func validateDownloadURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", errUnsafeURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("%w: scheme %q not allowed", errUnsafeURL, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: embedded credentials not allowed", errUnsafeURL)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", errUnsafeURL)
	}
	cleaned := strings.ReplaceAll(u.Path, "\\", "/")
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return fmt.Errorf("%w: path traversal not allowed", errUnsafeURL)
		}
	}
	return nil
}

// rejectNonPublicDial blocks connections to any resolved address that is not
// a global-unicast, routable address. This covers loopback, private (RFC 1918),
// link-local, multicast, and unspecified addresses — catching DNS-rebinding
// and redirect-based SSRF bypasses at the socket level.
func rejectNonPublicDial(_ context.Context, _, address string, _ syscall.RawConn) error {
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
	if addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return fmt.Errorf("%w: %s", errPrivateDownload, address)
	}
	if !addr.IsGlobalUnicast() {
		return fmt.Errorf("%w: %s is not a public address", errPrivateDownload, address)
	}
	return nil
}

// Kept for backward compatibility with callers that reference the old name.
func rejectLocalhostDial(ctx context.Context, network, address string, conn syscall.RawConn) error {
	return rejectNonPublicDial(ctx, network, address, conn)
}

var httpc = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	Timeout:   common.HTTPTimeout / 2,
}

var ssrfBlockedHTTPClient = &http.Client{
	Transport: &http.Transport{
		DialContext:     (&gonet.Dialer{ControlContext: rejectNonPublicDial}).DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		return validateDownloadURL(req.URL.String())
	},
	Timeout: common.HTTPTimeout / 2,
}

// localhostBlockedHTTPClient is kept as an alias for backward compatibility.
var localhostBlockedHTTPClient = ssrfBlockedHTTPClient

// LocalhostBlockedHTTPClient returns the shared HTTP client that refuses
// connections to non-public addresses. Callers must not mutate the client.
func LocalhostBlockedHTTPClient() *http.Client {
	return ssrfBlockedHTTPClient
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
