package drivers

import (
	"io/ioutil"
	"os"
	"strings"

	skynet "github.com/NebulousLabs/go-skynet"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
)

type (
	// gsKeyJSON struct {
	// 	Type                    string `json:"type,omitempty"`
	// 	ProjectID               string `json:"project_id,omitempty"`
	// 	PrivateKeyID            string `json:"private_key_id,omitempty"`
	// 	PrivateKey              string `json:"private_key,omitempty"`
	// 	ClientEmail             string `json:"client_email,omitempty"`
	// 	ClientID                string `json:"client_id,omitempty"`
	// 	AuthURI                 string `json:"auth_uri,omitempty"`
	// 	TokenURI                string `json:"token_uri,omitempty"`
	// 	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url,omitempty"`
	// 	ClientX509CertURL       string `json:"client_x509_cert_url,omitempty"`
	// }

	// gsSigner struct {
	// 	jsKey     *gsKeyJSON
	// 	parsedKey *rsa.PrivateKey
	// }

	siaOS struct {
		// 	s3OS
		// 	gsSigner *gsSigner
	}

	siaSession struct {
		Name string
	}
)

// IsOwnStorageSia returns true if uri points to Sia location owned by this node
func IsOwnStorageSia(uri string) bool {
	return true // for now, Sia is sponsoring
}

func NewSiaDriver() (OSDriver, error) {
	os := &siaOS{}
	return os, nil
}

func (os *siaOS) NewSession(path string) OSSession {
	// var policy, signature = gsCreatePolicy(os.gsSigner, os.bucket, os.region, path)
	sess := &siaSession{Name: "SiaSession"}
	// sess.fields = gsGetFields(sess)
	return sess
}

func (sess *siaSession) SaveData(name string, data []byte) (string, error) {
	fname := "./tmp_skynet" + name
	glog.V(common.DEBUG).Infof("fname: %s", fname)
	defer func(fn string) {
		os.Remove(fn)
	}(fname)
	ioutil.WriteFile(fname, data, 0644)
	skyhash, err := skynet.UploadFile(fname, skynet.DefaultUploadOptions)
	if err != nil {
		glog.V(common.DEBUG).Infof("Uploaded to Skynet: %s", skyhash)
		// fmt.Println("Uploaded to Skynet: " + skyhash)
	}
	return strings.Replace(skyhash, "sia://", "https://siasky.net/", -1), nil
}

func (sess *siaSession) EndSession() {

}

func (sess *siaSession) IsExternal() bool {
	return true
}

func (sess *siaSession) GetInfo() *net.OSInfo {
	return nil
}

// func newGSSession(info *net.S3OSInfo) OSSession {
// 	sess := &s3Session{
// 		host:        info.Host,
// 		key:         info.Key,
// 		policy:      info.Policy,
// 		signature:   info.Signature,
// 		credential:  info.Credential,
// 		storageType: net.OSInfo_GOOGLE,
// 	}
// 	sess.fields = gsGetFields(sess)
// 	return sess
// }

// func gsGetFields(sess *s3Session) map[string]string {
// 	return map[string]string{
// 		"GoogleAccessId": sess.credential,
// 		"signature":      sess.signature,
// 	}
// }

// // gsCreatePolicy returns policy, signature
// func gsCreatePolicy(signer *gsSigner, bucket, region, path string) (string, string) {
// 	const timeFormat = "2006-01-02T15:04:05.999Z"
// 	const shortTimeFormat = "20060102"

// 	expireAt := time.Now().Add(S3_POLICY_EXPIRE_IN_HOURS * time.Hour)
// 	expireFmt := expireAt.UTC().Format(timeFormat)
// 	src := fmt.Sprintf(`{ "expiration": "%s",
//     "conditions": [
//       {"bucket": "%s"},
//       {"acl": "public-read"},
//       ["starts-with", "$Content-Type", ""],
//       ["starts-with", "$key", "%s"]
//     ]
//   }`, expireFmt, bucket, path)
// 	policy := base64.StdEncoding.EncodeToString([]byte(src))
// 	sign := signer.sign(policy)
// 	return policy, sign
// }

// func (s *gsSigner) sign(mes string) string {
// 	h := sha256.New()
// 	h.Write([]byte(mes))
// 	d := h.Sum(nil)

// 	signature, err := rsa.SignPKCS1v15(rand.Reader, s.parsedKey, crypto.SHA256, d)
// 	if err != nil {
// 		panic(err)
// 	}

// 	return base64.StdEncoding.EncodeToString(signature)
// }

// func (s *gsSigner) clientEmail() string {
// 	return s.jsKey.ClientEmail
// }
