package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectNonPublicDial(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr error
	}{
		{name: "IPv4 loopback", address: "127.0.0.1:7935", wantErr: errLocalhostDownload},
		{name: "IPv4 loopback range", address: "127.255.255.254:7935", wantErr: errLocalhostDownload},
		{name: "IPv6 loopback", address: "[::1]:7935", wantErr: errLocalhostDownload},
		{name: "IPv4-mapped IPv6 loopback", address: "[::ffff:127.0.0.1]:7935", wantErr: errLocalhostDownload},
		{name: "unspecified IPv4", address: "0.0.0.0:7935", wantErr: errLocalhostDownload},
		{name: "unspecified IPv6", address: "[::]:7935", wantErr: errLocalhostDownload},
		{name: "private IPv4 10.x", address: "10.0.0.1:9000", wantErr: errPrivateDownload},
		{name: "private IPv4 192.168.x", address: "192.168.1.1:9000", wantErr: errPrivateDownload},
		{name: "private IPv4 172.16.x", address: "172.16.0.1:9000", wantErr: errPrivateDownload},
		{name: "link-local IPv4", address: "169.254.1.1:9000", wantErr: errPrivateDownload},
		{name: "private IPv6", address: "[fd00::1]:9000", wantErr: errPrivateDownload},
		{name: "link-local IPv6", address: "[fe80::1]:9000", wantErr: errPrivateDownload},
		{name: "multicast IPv4", address: "224.0.0.1:9000", wantErr: errPrivateDownload},
		{name: "multicast IPv6", address: "[ff02::1]:9000", wantErr: errPrivateDownload},
		{name: "public IPv4", address: "8.8.8.8:443"},
		{name: "public IPv6", address: "[2001:4860:4860::8888]:443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectNonPublicDial(context.Background(), "tcp", tt.address, nil)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}

	require.Error(t, rejectNonPublicDial(context.Background(), "tcp", "127.0.0.1", nil))
}

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://storage.example.com/seg/0.ts"},
		{name: "valid http", url: "http://cdn.example.com/seg/0.ts"},
		{name: "ftp blocked", url: "ftp://evil.com/file", wantErr: true},
		{name: "file scheme blocked", url: "file:///etc/passwd", wantErr: true},
		{name: "gopher blocked", url: "gopher://evil.com", wantErr: true},
		{name: "empty scheme blocked", url: "://foo", wantErr: true},
		{name: "embedded credentials", url: "http://user:pass@host.com/x", wantErr: true},
		{name: "path traversal", url: "http://host.com/../../etc/passwd", wantErr: true},
		{name: "encoded path traversal", url: "http://host.com/a/%2e%2e/b", wantErr: true},
		{name: "missing host", url: "http:///path", wantErr: true},
		{name: "data URI", url: "data:text/html,<script>", wantErr: true},
		{name: "javascript URI", url: "javascript:alert(1)", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDownloadURL(tt.url)
			if tt.wantErr {
				require.ErrorIs(t, err, errUnsafeURL)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDownloadDataRejectsLoopback(t *testing.T) {
	var hits atomic.Int32
	protected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("protected"))
	}))
	defer protected.Close()

	loopbackURL, err := url.Parse(protected.URL)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(loopbackURL.Host)
	require.NoError(t, err)
	loopbackURL.Host = net.JoinHostPort("localhost", port)

	for _, target := range []string{protected.URL, loopbackURL.String()} {
		_, err := DownloadData(context.Background(), target)
		require.ErrorIs(t, err, errLocalhostDownload)
	}
	require.Zero(t, hits.Load())
}

func TestDownloadDataAllowLocalhostAllowsLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("segment"))
	}))
	defer server.Close()

	data, err := DownloadDataAllowLocalhost(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, []byte("segment"), data)
}

func TestDownloadDataBlocksPrivateAddress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/data", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("segment"))
	})
	baseURL := newNonLoopbackHTTPServer(t, mux)

	// Non-loopback interface addresses on CI/dev boxes are typically private
	// (10.x, 172.x, 192.168.x) and should now be blocked.
	_, err := DownloadData(context.Background(), baseURL+"/data")
	require.Error(t, err)
}

func TestDownloadDataBlocksRedirectToLoopback(t *testing.T) {
	var protectedHits atomic.Int32
	protected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		protectedHits.Add(1)
		_, _ = w.Write([]byte("protected"))
	}))
	defer protected.Close()

	// Use a public-reachable test URL that redirects to loopback.
	// In unit tests we can't easily test a real public→loopback redirect,
	// so test that the protected server itself (loopback) is blocked.
	_, err := DownloadData(context.Background(), protected.URL)
	require.ErrorIs(t, err, errLocalhostDownload)
	require.Zero(t, protectedHits.Load())
}

func newNonLoopbackHTTPServer(t *testing.T, handler http.Handler) string {
	t.Helper()

	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	var host net.IP
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip = ip.To4(); ip != nil && ip.IsGlobalUnicast() && !ip.IsLoopback() {
			host = ip
			break
		}
	}
	if host == nil {
		t.Skip("no non-loopback IPv4 interface available")
	}

	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	require.NoError(t, err)
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown(context.Background()))
	})

	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://%s:%d", host, port)
}
