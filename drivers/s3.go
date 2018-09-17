package drivers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// region "eu-central-1"
type s3OS struct {
	region             string
	bucket             string
	awsAccessKeyID     string
	awsSecretAccessKey string
	sessions           map[uint64]OSSession
	sessionsLock       sync.Mutex
	policy             string
	xAmzCredential     string
	xAmzDate           string
	signature          string
}

type s3OSSession struct {
	osd        *s3OS
	created    time.Time
	creds      *credentials.Credentials
	jobID      int64
	s3svc      *s3.S3
	manifestID string
	nonce      uint64
}

// S3BUCKET s3 bucket owned by this node
var S3BUCKET string

// S3REGION region of s3 bucket owned by this node
var S3REGION string

// IsOwnStorageS3 returns true if turi points to S3 bucket owned by this node
func IsOwnStorageS3(turi *net.TypedURI) bool {
	if turi.Storage != "s3" || S3BUCKET == "" || S3REGION == "" {
		return false
	}
	// XXX compare to bucket, region configured for this node
	// parse url?
	return strings.Contains(turi.Uri, S3BUCKET) && strings.Contains(turi.Uri, S3REGION)
}

func newS3Driver(info *net.S3OSInfo) OSDriver {
	od := &s3OS{
		region:             info.Region,
		bucket:             info.Bucket,
		sessions:           make(map[uint64]OSSession),
		sessionsLock:       sync.Mutex{},
		awsAccessKeyID:     info.AccessKey,
		awsSecretAccessKey: info.AccessKeySecret,
		policy:             info.Policy,
		signature:          info.Signature,
		xAmzDate:           info.XAmzDate,
		xAmzCredential:     info.XAmzCredential,
	}
	return od
}

func (os *s3OS) IsExternal() bool {
	return true
}

func (os *s3OS) StartSession(jobID int64, manifestID string, nonce uint64) OSSession {
	glog.V(common.DEBUG).Infof("S3 Starting session for job %d man %s nonce %d", jobID, manifestID, nonce)
	os.sessionsLock.Lock()
	defer os.sessionsLock.Unlock()
	if es, ok := os.sessions[nonce]; ok {
		return es
	}
	s := s3OSSession{
		osd:        os,
		created:    time.Now(),
		jobID:      jobID,
		manifestID: manifestID,
		nonce:      nonce,
	}
	if os.awsAccessKeyID != "" {
		s.creds = credentials.NewStaticCredentials(os.awsAccessKeyID, os.awsSecretAccessKey, "")
		cfg := aws.NewConfig().WithRegion(s.osd.region).WithCredentials(s.creds)
		s.s3svc = s3.New(session.New(), cfg)
	}
	os.sessions[nonce] = &s
	return &s
}

func (session *s3OSSession) EndSession() {

}

func (session *s3OSSession) IsOwnStorage(turi *net.TypedURI) bool {
	return IsOwnStorageS3(turi)
}

func (session *s3OSSession) GetInfo() net.OSInfo {
	info := net.OSInfo{
		Storage: "s3",
		S3Info: &net.S3OSInfo{
			Region: session.osd.region,
			Bucket: session.osd.bucket,
		},
	}
	if session.osd.awsAccessKeyID != "" {
		policy, signature, xAmzCredential, xAmzDate := createPolicy(session.osd.awsAccessKeyID,
			session.osd.bucket, session.osd.region, session.osd.awsSecretAccessKey)
		info.S3Info.Policy = policy
		info.S3Info.XAmzCredential = xAmzCredential
		info.S3Info.XAmzDate = xAmzDate
		info.S3Info.Signature = signature
	}
	return info
}

func (session *s3OSSession) getAbsURL(path string) string {
	return fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", session.osd.region, session.osd.bucket, path)
}

func (session *s3OSSession) SaveData(streamID, name string, data []byte) (*net.TypedURI, string, error) {
	glog.V(common.VERBOSE).Infof("saving data to s3 %s", name)
	path, err := session.putData(name, data)
	if err != nil {
		// handle error
		glog.Errorf("put file error: %v", err)
	}
	abs := session.getAbsURL(path)

	turl := &net.TypedURI{
		Storage:       "s3",
		Uri:           abs,
		StreamID:      streamID,
		UriInManifest: name,
	}
	glog.V(common.VERBOSE).Infof("data saved to s3: %s", turl.Uri)
	return turl, abs, err
}

func (session *s3OSSession) putData(name string, data []byte) (string, error) {
	glog.V(common.VERBOSE).Infof("S3 saving %s into manifest %s with nonce %d", name, session.manifestID, session.nonce)
	path := fmt.Sprintf("%s_%d", session.manifestID, session.nonce)
	key := path + "/" + name
	if session.s3svc == nil {
		return session.postData(path, name, data)
	}

	reader := bytes.NewReader(data)
	size := int64(len(data))
	fileType := http.DetectContentType(data)

	params := &s3.PutObjectInput{
		Bucket:        aws.String(session.osd.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
		ACL:           aws.String("public-read"),
	}

	resp, err := session.s3svc.PutObject(params)
	glog.V(common.VERBOSE).Infof("response %s", awsutil.StringValue(resp))
	return key, err
}

func (session *s3OSSession) postData(path, fileName string, buffer []byte) (string, error) {
	glog.V(common.VERBOSE).Infof("saving to s3 with post %s / %s", path, fileName)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)
	key := path + "/${filename}"
	fields := map[string]string{
		"acl":              "public-read",
		"Content-Type":     fileType,
		"key":              key,
		"policy":           session.osd.policy,
		"x-amz-algorithm":  "AWS4-HMAC-SHA256",
		"x-amz-credential": session.osd.xAmzCredential,
		"x-amz-date":       session.osd.xAmzDate,
		"x-amz-signature":  session.osd.signature,
	}
	postURL := "https://" + session.osd.bucket + ".s3.amazonaws.com/"
	req, err := newfileUploadRequest(postURL, fields, fileBytes, fileName)
	if err != nil {
		glog.Error(err)
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Error(err)
	} else {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			glog.Error(err)
		}
		resp.Body.Close()
	}
	return path + "/" + fileName, err
}

func makeHmac(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func signString(stringToSign, sregion, amzDate, secret string) string {
	date := makeHmac([]byte("AWS4"+secret), []byte(amzDate))
	region := makeHmac(date, []byte(sregion))
	service := makeHmac(region, []byte("s3"))
	credentials := makeHmac(service, []byte("aws4_request"))
	signature := makeHmac(credentials, []byte(stringToSign))
	sSignature := hex.EncodeToString(signature)
	return sSignature
}

// createPolicy returns policy, signature, xAmzCredentail and xAmzDate
func createPolicy(key, bucket, region, secret string) (string, string, string, string) {
	const timeFormat = "2006-01-02T15:04:05.999Z"
	const shortTimeFormat = "20060102"

	expireAt := time.Now().Add(24 * time.Hour)
	expireFmt := expireAt.UTC().Format(timeFormat)
	xAmzDate := time.Now().UTC().Format(shortTimeFormat)
	xAmzCredential := fmt.Sprintf("%s/%s/%s/s3/aws4_request", key, xAmzDate, region)
	src := fmt.Sprintf(`{ "expiration": "%s",
    "conditions": [
      {"bucket": "%s"},
      {"acl": "public-read"},
      ["starts-with", "$Content-Type", ""],
      ["starts-with", "$key", ""],
      {"x-amz-algorithm": "AWS4-HMAC-SHA256"},
      {"x-amz-credential": "%s"},
      {"x-amz-date": "%sT000000Z" }
    ]
  }`, expireFmt, bucket, xAmzCredential, xAmzDate)
	policy := base64.StdEncoding.EncodeToString([]byte(src))
	return policy, signString(policy, region, xAmzDate, secret), xAmzCredential, xAmzDate + "T000000Z"
}

func newfileUploadRequest(uri string, params map[string]string, fData io.Reader, fileName string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, val := range params {
		err := writer.WriteField(key, val)
		if err != nil {
			glog.Error(err)
		}
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, fData)

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}
