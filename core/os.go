/*
Object store helper functions
*/
package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
)

func DownloadData(ctx context.Context, uri string) ([]byte, error) {
	return downloadDataHTTP(ctx, uri)
}

var osHttpClient = getHTTPClient()

// getHTTPClient creates an HTTP client with a timeout based on an environment variable or defaults to common.HTTPTimeout/2
func getHTTPClient() *http.Client {
	// Get the timeout value from the environment variable
	timeoutStr := os.Getenv("LIVEPEER_OS_HTTP_TIMEOUT")

	// Define a default timeout value as common.HTTPTimeout / 2
	defaultTimeout := common.HTTPTimeout / 2

	var timeout time.Duration
	var err error

	// If the environment variable is set, attempt to parse it
	if timeoutStr != "" {
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			timeout = defaultTimeout
		}
	} else {
		// If the environment variable is not set, use the default timeout
		timeout = defaultTimeout
	}

	// Return the HTTP client with the calculated timeout
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   timeout,
	}
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

func downloadDataHTTP(ctx context.Context, uri string) ([]byte, error) {
	clog.V(common.VERBOSE).Infof(ctx, "Downloading uri=%s", uri)
	started := time.Now()
	resp, err := osHttpClient.Get(uri)
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
