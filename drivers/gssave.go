package drivers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"google.golang.org/api/option"
)

const fullURL = "https://console.cloud.google.com/storage/browser/"

// FailSaveBucketName name of the bucket to save FV faild segments to
var FailSaveBucketName string
var credsJSON string

// should be saved to GS
func FailSaveEnabled() bool {
	return credsJSON != ""
}

// SetCreds ...
func SetCreds(bucket, creds string) {
	FailSaveBucketName = bucket

	info, err := os.Stat(creds)
	glog.Infof("bucket %s creds %s is not ex %v is dir %v", bucket, creds, os.IsNotExist(err), info != nil && info.IsDir())
	if err == nil && !info.IsDir() {
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

func SavePairData2GS(trusturi string, data1 []byte, untrusturi string, data2 []byte, suffix string, src []byte) error {

	fnames := make([]string, 0)
	datas := make([][]byte, 0)
	trustpurl, _ := url.Parse(trusturi)
	untrustpurl, _ := url.Parse(untrusturi)
	pairstr := strconv.Itoa(rand.Int()) + "-"
	fileName1 := pairstr + trustpurl.Host + "-trust-" + suffix
	fileName2 := pairstr + untrustpurl.Host + "-untrust-" + suffix
	fnames = append(fnames, fileName1)
	fnames = append(fnames, fileName2)
	datas = append(datas, data1)
	datas = append(datas, data2)
	if src != nil {
		fnames = append(fnames, pairstr+"-source.ts")
		datas = append(datas, src)
	}

	var wait sync.WaitGroup
	wait.Add(len(fnames))
	dlfunc := func(fname string, data []byte) {
		defer wait.Done()
		fu, err := Save2GS(fname, data)
		if err != nil {
			glog.Infof("Error saving to GS bucket=%s, err=%v", FailSaveBucketName, err)
		} else {
			glog.Infof("Segment name=%s saved to url=%s", fname, fu)
		}
	}
	for i, name := range fnames {
		go dlfunc(name, datas[i])
	}
	wait.Wait()
	return nil
}
