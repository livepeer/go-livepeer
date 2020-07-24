package drivers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3_POLICY_EXPIRE_IN_HOURS how long access rights given to other node will be valid
const S3_POLICY_EXPIRE_IN_HOURS = 24

/* S3OS S# backed object storage driver. For own storage access key and access key secret
   should be specified. To give to other nodes access to own S3 storage so called 'POST' policy
   is created. This policy is valid for S3_POLICY_EXPIRE_IN_HOURS hours.
*/
type s3OS struct {
	host               string
	region             string
	bucket             string
	awsAccessKeyID     string
	awsSecretAccessKey string
	s3svc              *s3.S3
	useFullAPI         bool
}

type s3Session struct {
	host        string
	bucket      string
	key         string
	policy      string
	signature   string
	credential  string
	xAmzDate    string
	storageType net.OSInfo_StorageType
	fields      map[string]string
	s3svc       *s3.S3
}

// S3BUCKET s3 bucket owned by this node
var S3BUCKET string

func s3Host(bucket string) string {
	return fmt.Sprintf("https://%s.s3.amazonaws.com", bucket)
	// return fmt.Sprintf("https://%s.s3.us-west-002.backblazeb2.com", bucket)
	// return fmt.Sprintf("https://%s.storage.googleapis.com", bucket)
}

// IsOwnStorageS3 returns true if uri points to S3 bucket owned by this node
func IsOwnStorageS3(uri string) bool {
	return strings.HasPrefix(uri, s3Host(S3BUCKET))
}

func newS3Session(info *net.S3OSInfo) OSSession {
	sess := &s3Session{
		host:        info.Host,
		key:         info.Key,
		policy:      info.Policy,
		signature:   info.Signature,
		xAmzDate:    info.XAmzDate,
		credential:  info.Credential,
		storageType: net.OSInfo_S3,
	}
	sess.fields = s3GetFields(sess)
	return sess
}

func NewS3Driver(region, bucket, accessKey, accessKeySecret string, useFullAPI bool) OSDriver {
	glog.Infof("Creating S3 with region %s bucket %s", region, bucket)
	os := &s3OS{
		host:               s3Host(bucket),
		region:             region,
		bucket:             bucket,
		awsAccessKeyID:     accessKey,
		awsSecretAccessKey: accessKeySecret,
		useFullAPI:         useFullAPI,
	}
	if os.awsAccessKeyID != "" {
		creds := credentials.NewStaticCredentials(os.awsAccessKeyID, os.awsSecretAccessKey, "")
		cfg := aws.NewConfig().WithRegion(os.region).WithCredentials(creds)
		os.s3svc = s3.New(session.New(), cfg)
	}
	return os
}

// For creating S3-compatible stores other than S3 itself
func NewCustomS3Driver(host, bucket, accessKey, accessKeySecret string, useFullAPI bool) OSDriver {
	glog.Infof("using custom s3 with url: %s, bucket %s use full API %v", host, bucket, useFullAPI)
	os := &s3OS{
		host:               host,
		bucket:             bucket,
		awsAccessKeyID:     accessKey,
		awsSecretAccessKey: accessKeySecret,
		region:             "ignored",
		useFullAPI:         useFullAPI,
	}
	if !useFullAPI {
		os.host += "/" + bucket
	}
	if os.awsAccessKeyID != "" {
		creds := credentials.NewStaticCredentials(os.awsAccessKeyID, os.awsSecretAccessKey, "")
		cfg := aws.NewConfig().WithRegion(os.region).WithCredentials(creds)
		// cfg := aws.NewConfig().WithCredentials(creds)
		cfg = cfg.WithEndpoint(host)
		cfg = cfg.WithS3ForcePathStyle(true)
		// cfg = cfg.WithEndpointResolver(os)
		// cfg = cfg.WithDisableEndpointHostPrefix(true)
		// cfg = cfg.WithEndpoint("https://s3.us-west-002.backblazeb2.com")
		// cfg = cfg.WithEndpoint("https://storage.googleapis.com")
		os.s3svc = s3.New(session.New(), cfg)
	}
	return os
}

/*
func (os *s3OS) EndpointFor(service, region string, opts ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	glog.Infof("Got endpoint request for service %s region '%s' opts: %+v", service, region, opts)
	res := endpoints.ResolvedEndpoint{
		URL:           os.host,
		SigningRegion: os.region,
	}
	glog.Infof("Returning: %+v", res)
	return res, nil
}
*/

func (os *s3OS) NewSession(path string) OSSession {
	policy, signature, credential, xAmzDate := createPolicy(os.awsAccessKeyID,
		os.bucket, os.region, os.awsSecretAccessKey, path)
	sess := &s3Session{
		host:        os.host,
		bucket:      os.bucket,
		key:         path,
		policy:      policy,
		signature:   signature,
		credential:  credential,
		xAmzDate:    xAmzDate,
		storageType: net.OSInfo_S3,
	}
	if os.useFullAPI {
		sess.s3svc = os.s3svc
	}
	sess.fields = s3GetFields(sess)
	return sess
}

func s3GetFields(sess *s3Session) map[string]string {
	return map[string]string{
		"x-amz-algorithm":  "AWS4-HMAC-SHA256",
		"x-amz-credential": sess.credential,
		"x-amz-date":       sess.xAmzDate,
		"x-amz-signature":  sess.signature,
	}
}

func (os *s3Session) IsExternal() bool {
	return true
}

func (os *s3Session) EndSession() {
}

type s3pageInfo struct {
	files       []FileInfo
	directories []string
	hasNextPage bool
	ctx         context.Context
	s3svc       *s3.S3
	params      *s3.ListObjectsInput
	nextMarker  string
}

func (s3pi *s3pageInfo) Files() []FileInfo {
	return s3pi.files
}
func (s3pi *s3pageInfo) Directories() []string {
	return s3pi.directories
}
func (s3pi *s3pageInfo) HasNextPage() bool {
	return s3pi.hasNextPage
}
func (s3pi *s3pageInfo) NextPage() (PageInfo, error) {
	if !s3pi.hasNextPage {
		return nil, ErrNoNextPage
	}
	next := &s3pageInfo{
		s3svc:  s3pi.s3svc,
		params: s3pi.params,
		ctx:    s3pi.ctx,
	}
	next.params.Marker = &s3pi.nextMarker
	if err := next.listFiles(); err != nil {
		return nil, err
	}
	return next, nil
}

func (s3pi *s3pageInfo) listFiles() error {
	resp, err := s3pi.s3svc.ListObjectsWithContext(s3pi.ctx, s3pi.params)
	// glog.Infof("list resp: %s", resp)
	if err != nil {
		return err
	}
	for _, cont := range resp.CommonPrefixes {
		s3pi.directories = append(s3pi.directories, *cont.Prefix)
	}
	s3pi.hasNextPage = *resp.IsTruncated
	for _, cont := range resp.Contents {
		fi := FileInfo{
			Name:         *cont.Key,
			ETag:         *cont.ETag,
			LastModified: *cont.LastModified,
			Size:         *cont.Size,
		}
		s3pi.files = append(s3pi.files, fi)
	}
	if resp.NextMarker != nil {
		s3pi.nextMarker = *resp.NextMarker
	} else if *resp.IsTruncated && len(resp.Contents) > 0 {
		s3pi.nextMarker = *resp.Contents[len(resp.Contents)-1].Key
	}
	return nil
}

func (os *s3Session) ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error) {
	if os.s3svc != nil {
		bucket := aws.String(os.bucket)
		params := &s3.ListObjectsInput{
			Bucket: bucket,
		}
		if prefix != "" {
			params.Prefix = aws.String(prefix)
		}
		if delim != "" {
			params.Delimiter = aws.String(delim)
		}
		pi := &s3pageInfo{
			ctx:    ctx,
			s3svc:  os.s3svc,
			params: params,
		}
		if err := pi.listFiles(); err != nil {
			return nil, err
		}
		return pi, nil
	}

	return nil, fmt.Errorf("Not implemented")
}

func (os *s3Session) ReadData(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {
	if os.s3svc != nil {
		params := &s3.GetObjectInput{
			Bucket: aws.String(os.bucket),
			Key:    aws.String(name),
		}
		resp, err := os.s3svc.GetObjectWithContext(ctx, params)
		if err != nil {
			return nil, nil, err
		}
		meta := map[string]string{
			"ETag":         *resp.ETag,
			"LastModified": (*resp.LastModified).String(),
		}
		for k, v := range resp.Metadata {
			meta[k] = *v
		}
		return resp.Body, meta, nil
	}
	return nil, nil, fmt.Errorf("Not implemented")
}

func (os *s3Session) saveDataPut(name string, data []byte, meta map[string]string) (string, error) {
	bucket := aws.String(os.bucket)
	keyname := aws.String(os.key + "/" + name)
	var metadata map[string]*string
	if len(meta) > 0 {
		metadata = make(map[string]*string)
		for k, v := range meta {
			metadata[k] = aws.String(v)
		}
	}
	contentType := aws.String(os.getContentType(name, data))

	params := &s3.PutObjectInput{
		Bucket:   bucket,  // Required
		Key:      keyname, // Required
		Metadata: metadata,
		// ACL:    aws.String("bucket-owner-full-control"),
		Body:          bytes.NewReader(data),
		ContentType:   contentType,
		ContentLength: aws.Int64(int64(len(data))),
	}
	resp, err := os.s3svc.PutObject(params)
	if err != nil {
		return "", err
	}
	glog.Infof("resp: %s", resp.String())
	// var uri string
	uri := os.getAbsURL(*keyname)
	// if strings.Contains(os.host, os.bucket) {
	// 	uri = os.getAbsURL(*keyname)
	// } else {
	// 	uri = os.host + "/" + os.bucket + "/" + *keyname
	// }

	glog.V(common.VERBOSE).Infof("Saved to S3 %s", uri)

	return uri, err
}

func (os *s3Session) SaveData(name string, data []byte, meta map[string]string) (string, error) {
	if os.s3svc != nil {
		return os.saveDataPut(name, data, meta)
	}
	// tentativeUrl just used for logging
	tentativeURL := path.Join(os.host, os.key, name)
	glog.V(common.VERBOSE).Infof("Saving to S3 %s", tentativeURL)
	path, err := os.postData(name, data, meta)
	if err != nil {
		// handle error
		glog.Errorf("Save S3 error: %v", err)
		return "", err
	}
	url := os.getAbsURL(path)

	glog.V(common.VERBOSE).Infof("Saved to S3 %s", tentativeURL)

	return url, err
}

func (os *s3Session) getAbsURL(path string) string {
	if strings.Contains(os.host, os.bucket) {
		return os.host + "/" + path
	}
	return os.host + "/" + os.bucket + "/" + path
}

func (os *s3Session) GetInfo() *net.OSInfo {
	oi := &net.OSInfo{
		S3Info: &net.S3OSInfo{
			Host:       os.host,
			Key:        os.key,
			Policy:     os.policy,
			Signature:  os.signature,
			Credential: os.credential,
			XAmzDate:   os.xAmzDate,
		},
		StorageType: os.storageType,
	}
	return oi
}

func (os *s3Session) getContentType(fileName string, buffer []byte) string {
	ext := path.Ext(fileName)
	fileType, err := common.TypeByExtension(ext)
	if err != nil {
		fileType = http.DetectContentType(buffer)
	}
	return fileType
}

// if s3 storage is not our own, we are saving data into it using POST request
func (os *s3Session) postData(fileName string, buffer []byte, meta map[string]string) (string, error) {
	fileBytes := bytes.NewReader(buffer)
	fileType := os.getContentType(fileName, buffer)
	path, fileName := path.Split(path.Join(os.key, fileName))
	fields := map[string]string{
		"acl":          "public-read",
		"Content-Type": fileType,
		"key":          path + "${filename}",
		"policy":       os.policy,
	}
	for k, v := range os.fields {
		fields[k] = v
	}
	postURL := os.host
	if !strings.Contains(postURL, os.bucket) {
		postURL += "/" + os.bucket
	}
	req, err := newfileUploadRequest(postURL, fields, fileBytes, fileName)
	if err != nil {
		glog.Error(err)
		return "", err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Error(err)
		return "", err
	}
	body := &bytes.Buffer{}
	sz, err := body.ReadFrom(resp.Body)
	if err != nil {
		glog.Error(err)
		return "", err
	}
	resp.Body.Close()
	if sz > 0 {
		// usually there's an error at this point, so log
		glog.Error("Got response from from S3: ", body)
		return "", fmt.Errorf(body.String()) // sorta bad
	}
	return path + fileName, err
}

func (os *s3Session) IsOwn(url string) bool {
	return strings.HasPrefix(url, os.host)
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
func createPolicy(key, bucket, region, secret, path string) (string, string, string, string) {
	const timeFormat = "2006-01-02T15:04:05.999Z"
	const shortTimeFormat = "20060102"

	expireAt := time.Now().Add(S3_POLICY_EXPIRE_IN_HOURS * time.Hour)
	expireFmt := expireAt.UTC().Format(timeFormat)
	xAmzDate := time.Now().UTC().Format(shortTimeFormat)
	xAmzCredential := fmt.Sprintf("%s/%s/%s/s3/aws4_request", key, xAmzDate, region)
	src := fmt.Sprintf(`{ "expiration": "%s",
    "conditions": [
      {"bucket": "%s"},
      {"acl": "public-read"},
      ["starts-with", "$Content-Type", ""],
      ["starts-with", "$key", "%s"],
      {"x-amz-algorithm": "AWS4-HMAC-SHA256"},
      {"x-amz-credential": "%s"},
      {"x-amz-date": "%sT000000Z" }
    ]
  }`, expireFmt, bucket, path, xAmzCredential, xAmzDate)
	policy := base64.StdEncoding.EncodeToString([]byte(src))
	return policy, signString(policy, region, xAmzDate, secret), xAmzCredential, xAmzDate + "T000000Z"
}

func newfileUploadRequest(uri string, params map[string]string, fData io.Reader, fileName string) (*http.Request, error) {
	glog.Infof("Posting data to %s (params %+v)", uri, params)
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
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}
