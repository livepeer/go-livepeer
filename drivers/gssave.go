package drivers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"google.golang.org/api/option"
)

// const fullURL = "https://console.cloud.google.com/storage/browser/_details/lptest-fran/media_b452968_3.ts"
const fullURL = "https://console.cloud.google.com/storage/browser/_details/"

// FailSaveBucketName name of the bucket to save bad segments to
var FailSaveBucketName string
var credsJSON string

// FailSaveEnabled returns true if segments that failed to transcode
// should be saved to GS
func FailSaveEnabled() bool {
	return credsJSON != ""
}

// SetCreds ...
func SetCreds(bucket, creds string) {
	FailSaveBucketName = bucket

	info, err := os.Stat(creds)
	glog.Infof("bucket %s creds %s is not ex %v is dir %v", bucket, creds, os.IsNotExist(err), info != nil && info.IsDir())
	if !os.IsNotExist(err) && !info.IsDir() {
		t, _ := ioutil.ReadFile(creds)
		credsJSON = string(t)
		return
	}
	credsJSON = creds
}

// SaveFile2GS saves file to Google Cloud Storage
func SaveFile2GS(inpFileName, targetFileName string) (string, error) {
	var data []byte
	purl, err := url.Parse(inpFileName)
	if err == nil && purl.Scheme != "" {
		resp, err := httpc.Get(inpFileName)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Error reading from url=%s, err=%v", inpFileName, err)
		}
		data, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		targetFileName = purl.Path
		if targetFileName[0] == '/' {
			targetFileName = targetFileName[1:]
		}
	} else {
		data, err = ioutil.ReadFile(inpFileName)
	}
	if err != nil {
		return "", err
	}
	return Save2GS(targetFileName, data)
}

// Save2GS saves data to Google Cloud Storage
func Save2GS(fileName string, data []byte) (string, error) {
	if credsJSON == "" {
		return "", nil
	}
	ctx := context.Background()

	// Creates a client.
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(credsJSON)))
	if err != nil {
		glog.Errorf("Failed to create client: %v", err)
		return "", err
	}

	f := bytes.NewReader(data)

	wc := client.Bucket(FailSaveBucketName).Object(fileName).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}
	obj := client.Bucket(FailSaveBucketName).Object(fileName)
	return fullURL + FailSaveBucketName + "/" + obj.ObjectName(), nil
}
