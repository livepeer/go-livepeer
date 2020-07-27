package drivers

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

type (
	gsKeyJSON struct {
		Type                    string `json:"type,omitempty"`
		ProjectID               string `json:"project_id,omitempty"`
		PrivateKeyID            string `json:"private_key_id,omitempty"`
		PrivateKey              string `json:"private_key,omitempty"`
		ClientEmail             string `json:"client_email,omitempty"`
		ClientID                string `json:"client_id,omitempty"`
		AuthURI                 string `json:"auth_uri,omitempty"`
		TokenURI                string `json:"token_uri,omitempty"`
		AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url,omitempty"`
		ClientX509CertURL       string `json:"client_x509_cert_url,omitempty"`
	}

	gsSigner struct {
		jsKey     *gsKeyJSON
		parsedKey *rsa.PrivateKey
	}

	gsOS struct {
		s3OS
		gsSigner *gsSigner
		keyData  []byte
	}

	gsSession struct {
		s3Session
		gos        *gsOS
		client     *storage.Client
		keyData    []byte
		useFullAPI bool
	}
)

func gsHost(bucket string) string {
	return fmt.Sprintf("https://%s.storage.googleapis.com", bucket)
}

func gsParseKey(key []byte) (*rsa.PrivateKey, error) {
	if block, _ := pem.Decode(key); block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, err
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("oauth2: private key is invalid")
	}
	return parsed, nil
}

func NewGoogleDriver(bucket, keyData string, useFullAPI bool) (OSDriver, error) {
	os := &gsOS{
		s3OS: s3OS{
			host:       gsHost(bucket),
			bucket:     bucket,
			useFullAPI: useFullAPI,
		},
		keyData: []byte(keyData),
	}

	var gsKey gsKeyJSON
	if err := json.Unmarshal([]byte(keyData), &gsKey); err != nil {
		return nil, err
	}
	parsedKey, err := gsParseKey([]byte(gsKey.PrivateKey))
	if err != nil {
		return nil, err
	}
	os.gsSigner = &gsSigner{
		jsKey:     &gsKey,
		parsedKey: parsedKey,
	}
	if useFullAPI {
		client, err := storage.NewClient(context.Background(), option.WithCredentialsJSON(os.keyData))
		if err != nil {
			return nil, err
		}
		client.Close()
	}
	return os, nil
}

func (os *gsOS) NewSession(path string) OSSession {
	var policy, signature = gsCreatePolicy(os.gsSigner, os.bucket, os.region, path)
	sess := &s3Session{
		host:        gsHost(os.bucket),
		bucket:      os.bucket,
		key:         path,
		policy:      policy,
		signature:   signature,
		credential:  os.gsSigner.clientEmail(),
		storageType: net.OSInfo_GOOGLE,
	}
	sess.fields = gsGetFields(sess)
	gs := &gsSession{
		s3Session:  *sess,
		gos:        os,
		useFullAPI: os.useFullAPI,
		keyData:    os.keyData,
	}
	return gs
}

func newGSSession(info *net.S3OSInfo) OSSession {
	sess := &s3Session{
		host:        info.Host,
		key:         info.Key,
		policy:      info.Policy,
		signature:   info.Signature,
		credential:  info.Credential,
		storageType: net.OSInfo_GOOGLE,
	}
	sess.fields = gsGetFields(sess)
	return sess
}

func (os *gsSession) OS() OSDriver {
	return os.gos
}

func (os *gsSession) createClient() error {
	client, err := storage.NewClient(context.Background(), option.WithCredentialsJSON(os.keyData))
	if err != nil {
		glog.Errorf("Error creating GCP client err=%v", err)
		return err
	}
	os.client = client
	return nil
}

func (os *gsSession) SaveData(name string, data []byte, meta map[string]string) (string, error) {
	if os.useFullAPI {
		if os.client == nil {
			if err := os.createClient(); err != nil {
				return "", err
			}
		}
		keyname := os.key + "/" + name
		objh := os.client.Bucket(os.bucket).Object(keyname)
		glog.V(common.VERBOSE).Infof("Saving to GS %s/%s", os.bucket, keyname)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		defer cancel()
		wr := objh.NewWriter(ctx)
		if len(meta) > 0 && wr.Metadata == nil {
			wr.Metadata = make(map[string]string, len(meta))
		}
		for k, v := range meta {
			wr.Metadata[k] = v
		}
		wr.ContentType = os.getContentType(name, data)
		_, err := io.Copy(wr, bytes.NewReader(data))
		err2 := wr.Close()
		if err != nil {
			return "", err
		}
		if err2 != nil {
			return "", err2
		}
		uri := os.getAbsURL(keyname)
		glog.V(common.VERBOSE).Infof("Saved to GS %s", uri)
		return uri, err
	}
	return os.s3Session.SaveData(name, data, meta)
}

type gsPageInfo struct {
	s3pageInfo
	bucket string
	client *storage.Client
	query  *storage.Query
	it     *storage.ObjectIterator
}

func (gspi *gsPageInfo) NextPage() (PageInfo, error) {
	if gspi.nextMarker == "" {
		return nil, ErrNoNextPage
	}
	next := &gsPageInfo{
		s3pageInfo: s3pageInfo{
			ctx:        gspi.ctx,
			nextMarker: gspi.nextMarker,
		},
		query:  gspi.query,
		bucket: gspi.bucket,
		client: gspi.client,
		it:     gspi.it,
	}
	if err := next.listFiles(); err != nil {
		return nil, err
	}
	return next, nil
}

func (gspi *gsPageInfo) listFiles() error {
	it := gspi.it
	if gspi.it == nil {
		it = gspi.client.Bucket(gspi.bucket).Objects(gspi.ctx, gspi.query)
	}
	it.PageInfo().Token = gspi.nextMarker
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if attrs.Name == "" {
			gspi.directories = append(gspi.directories, attrs.Prefix)
		} else {
			fi := FileInfo{
				Name:         attrs.Name,
				ETag:         attrs.Etag,
				LastModified: attrs.Updated,
				Size:         attrs.Size,
			}
			gspi.files = append(gspi.files, fi)
		}
		if it.PageInfo().Remaining() == 0 {
			break
		}
	}
	gspi.nextMarker = it.PageInfo().Token
	return nil
}

func (os *gsSession) ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error) {
	if !os.useFullAPI {
		return nil, errors.New("Not implemented")
	}
	if os.client == nil {
		if err := os.createClient(); err != nil {
			return nil, err
		}
	}
	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: delim,
	}
	pi := &gsPageInfo{
		s3pageInfo: s3pageInfo{
			ctx: ctx,
		},
		query:  query,
		bucket: os.bucket,
		client: os.client,
	}
	if err := pi.listFiles(); err != nil {
		return nil, err
	}
	return pi, nil
}

func (os *gsSession) EndSession() {
	if os.client != nil {
		os.client.Close()
		os.client = nil
	}
}

func (os *gsSession) ReadData(ctx context.Context, name string) (*FileInfoReader, error) {
	if !os.useFullAPI {
		return nil, errors.New("Not implemented")
	}
	if os.client == nil {
		if err := os.createClient(); err != nil {
			return nil, err
		}
	}

	objh := os.client.Bucket(os.bucket).Object(name)
	attrs, err := objh.Attrs(ctx)
	if err != nil {
		return nil, err
	}
	res := &FileInfoReader{}
	res.Name = name
	res.Size = attrs.Size
	res.ETag = attrs.Etag
	res.LastModified = attrs.Updated
	if len(attrs.Metadata) > 0 {
		for k, v := range attrs.Metadata {
			res.Metadata = make(map[string]string, len(attrs.Metadata))
			res.Metadata[k] = v
		}
	}
	rc, err := objh.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	res.Body = rc
	return res, nil
}

func gsGetFields(sess *s3Session) map[string]string {
	return map[string]string{
		"GoogleAccessId": sess.credential,
		"signature":      sess.signature,
	}
}

// gsCreatePolicy returns policy, signature
func gsCreatePolicy(signer *gsSigner, bucket, region, path string) (string, string) {
	const timeFormat = "2006-01-02T15:04:05.999Z"
	const shortTimeFormat = "20060102"

	expireAt := time.Now().Add(S3_POLICY_EXPIRE_IN_HOURS * time.Hour)
	expireFmt := expireAt.UTC().Format(timeFormat)
	src := fmt.Sprintf(`{ "expiration": "%s",
    "conditions": [
	  {"bucket": "%s"},
      {"acl": "public-read"},
      ["starts-with", "$Content-Type", ""],
      ["starts-with", "$key", "%s"]
    ]
  }`, expireFmt, bucket, path)
	policy := base64.StdEncoding.EncodeToString([]byte(src))
	sign := signer.sign(policy)
	return policy, sign
}

func (s *gsSigner) sign(mes string) string {
	h := sha256.New()
	h.Write([]byte(mes))
	d := h.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.parsedKey, crypto.SHA256, d)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(signature)
}

func (s *gsSigner) clientEmail() string {
	return s.jsKey.ClientEmail
}
