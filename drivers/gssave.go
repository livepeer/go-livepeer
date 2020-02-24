package drivers

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"google.golang.org/api/option"
)

// const fullURL = "https://console.cloud.google.com/storage/browser/_details/lptest-fran/media_b452968_3.ts"
const fullURL = "https://console.cloud.google.com/storage/browser/_details/"

var credsJSON, bucketName string

// FailSaveEnabled returns true if segments that failed to transcode
// should be saved to GS
func FailSaveEnabled() bool {
	return credsJSON != ""
}

// SetCreds ...
func SetCreds(bucket, creds string) {
	bucketName = bucket

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
	data, err := ioutil.ReadFile(inpFileName)
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

	wc := client.Bucket(bucketName).Object(fileName).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}
	obj := client.Bucket(bucketName).Object(fileName)
	return fullURL + bucketName + "/" + obj.ObjectName(), nil
}
