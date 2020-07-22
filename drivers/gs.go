package drivers

import (
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
	"io/ioutil"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

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
		gsSigner    *gsSigner
		keyFileName string
	}

	gsSession struct {
		s3Session
		keyFileName string
	}
)

var GSBUCKET string

// IsOwnStorageGS returns true if uri points to Google Cloud Storage bucket owned by this node
func IsOwnStorageGS(uri string) bool {
	return strings.HasPrefix(uri, gsHost(GSBUCKET))
}

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

// listFilesWithPrefix lists objects using prefix and delimeter.
func listFilesWithPrefix(ctx context.Context, bucket, prefix, delim, keyFileName string) ([]string, error) {
	// bucket := "bucket-name"
	// prefix := "/foo"
	// delim := "_"
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(keyFileName))
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	// Prefixes and delimiters can be used to emulate directory listings.
	// Prefixes can be used to filter objects starting with prefix.
	// The delimiter argument can be used to restrict the results to only the
	// objects in the given "directory". Without the delimiter, the entire tree
	// under the prefix is returned.
	//
	// For example, given these blobs:
	//   /a/1.txt
	//   /a/b/2.txt
	//
	// If you just specify prefix="a/", you'll get back:
	//   /a/1.txt
	//   /a/b/2.txt
	//
	// However, if you specify prefix="a/" and delim="/", you'll get back:
	//   /a/1.txt
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	it := client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: delim,
	})
	var res []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Bucket(%q).Objects(): %v", bucket, err)
		}
		// fmt.Fprintln(w, attrs.Name)
		// glog.Infof("Got %+v", attrs)
		res = append(res, attrs.Name)
	}
	return res, nil
}

func NewGoogleDriver(bucket, keyFileName string) (OSDriver, error) {
	os := &gsOS{
		s3OS: s3OS{
			host:   gsHost(bucket),
			bucket: bucket,
		},
		keyFileName: keyFileName,
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
	return os, nil
}

func (os *gsOS) NewSession(path string) OSSession {
	var policy, signature = gsCreatePolicy(os.gsSigner, os.bucket, os.region, path)
	sess := &s3Session{
		host:        gsHost(os.bucket),
		key:         path,
		policy:      policy,
		signature:   signature,
		credential:  os.gsSigner.clientEmail(),
		storageType: net.OSInfo_GOOGLE,
	}
	sess.fields = gsGetFields(sess)
	return &gsSession{
		s3Session:   *sess,
		keyFileName: os.keyFileName,
	}
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

func (os *gsSession) ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error) {
	return nil, errors.New("Not implemented")
	// res, err := listFilesWithPrefix(ctx, GSBUCKET, prefix, delim, os.keyFileName)
	// return res, err
}

func (os *gsSession) ReadData(ctx context.Context, name string) (io.ReadCloser, map[string]string, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(os.keyFileName))
	if err != nil {
		return nil, nil, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	objh := client.Bucket(GSBUCKET).Object(name)
	rc, err := objh.NewReader(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Object(%q).NewReader: %v", name, err)
	}
	return rc, nil, nil
	/*
		defer rc.Close()

		data, err := ioutil.ReadAll(rc)
		if err != nil {
			return nil, nil, fmt.Errorf("ioutil.ReadAll: %v", err)
		}

		return data, nil, nil
	*/
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
	//   ["starts-with", "$x-goog-meta-duration", ""],

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
