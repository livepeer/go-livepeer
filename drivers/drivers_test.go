package drivers

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestS3URL(t *testing.T) {
	assert := assert.New(t)
	os, err := ParseOSURL("s3://user:xxxxxxxxx%2Bxxxxxxxxx%2Fxxxxxxxx%2Bxxxxxxxxxxxxx@us-west-2/example-bucket", true)
	assert.Equal(nil, err)
	s3, iss3 := os.(*s3OS)
	assert.Equal(true, iss3)
	assert.Equal("user", s3.awsAccessKeyID)
	assert.Equal("xxxxxxxxx+xxxxxxxxx/xxxxxxxx+xxxxxxxxxxxxx", s3.awsSecretAccessKey)
	assert.Equal("https://example-bucket.s3.amazonaws.com", s3.host)
	assert.Equal("us-west-2", s3.region)
	assert.Equal("example-bucket", s3.bucket)
}

func TestCustomS3URL(t *testing.T) {
	assert := assert.New(t)
	os, err := ParseOSURL("s3+http://user:password@example.com:9000/path/to/bucket-name", true)
	s3, iss3 := os.(*s3OS)
	assert.Equal(true, iss3)
	assert.Equal(nil, err)
	assert.Equal("http://example.com:9000/path/to/bucket-name", s3.host)
	assert.Equal("bucket-name", s3.bucket)
	assert.Equal("user", s3.awsAccessKeyID)
	assert.Equal("password", s3.awsSecretAccessKey)
}

func TestGSURL(t *testing.T) {
	assert := assert.New(t)
	// Don't worry, I invalidated this
	testGSToken := `{
		"type": "service_account",
		"project_id": "livepeerjs-231617",
		"private_key_id": "835d25ed984195fab1e551d9c9c351921b9512cb",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCdhlk4y6a/U5qu\ngGQ5vqxfwcJ6qJKNtMRTa5DEOny0XKwsbfUEF6YMbsMx1ZGhgiAH2w+KMukpdPMd\nAU6DxNDrractn2i+rSTL8mcRXALGlBLkpHWfQZvSojcqo5CVktpUsv31PJirGEuC\n0HuSVcSuM/RFG4B3tcMtxNB8yh9AOTYqe3H9YNywLig+vTJMh9tGkKA5FFwN0gAh\nSDfuDpkmewbsysgQmNqytvjP5yRtSfX8G5URDt/6Ge5Dbb763LhLPtmxg9hTizYo\nSbvytj/tHtIZBeYvPMZq78m7B8rNemiVDHMxE1+WoBjbixIkjGz5GTXX0NRv222q\nqqyGb5/NAgMBAAECggEACDq1TQwIl0y3E0A2XDTrnOoKrq1BUMFdk00WiEXU73g6\n72xEJVVV8abUsDUL0V/yq+5kCrB7sVSAgeaoUyZ0UqelCPNfvbxeZIAypbvEklq4\nfPThhzMegJvEXYgjfMjp+oxKS6ZBhIi1oy0gk4XDC2W/8F9OMBLREkJKsQY/KTPu\n9n5QkkRA1bhI2d1sj+HHOwKwfyxzhwLXvni4F1X2V8C1YLQ/9kb+kpul5+j5QRMr\nwW8rNsihXXcR5U0q8X4OEqGGsVb3/2e/lAw5KAvJ4MPAgwrjmLbPViuU01KQXS1J\ndgix5KPeOZ8ejWfvxXP0EoqzGCq7K+61xy2Zg1JCGQKBgQDLICfxNWn/xf8QIvIC\ndxSCnWZWydyy5sStfManJhaJcm2JdL95uLI+b3oCE6dLiOxtu4lzrxKELjyJNBV1\nbxnc/JfWG0FURl7c8G+VTGJiSaRH5WhBvziD3kphgKpjL6cHXSJR/tT+ElBBjK9o\nOIvTeMi6uh710mMD+1crUSpwaQKBgQDGh3Rqn7Z6EYYRMx7hczZci8ktKYiyyEBH\niL/1O6MR39Pn81VWHhr/0EwfbPG3k9xDsoKRGGwE54Q9Ymlff4k9d/5qsEl7VM4P\n/pK1vipMXyVtyPYRfAsPdonaotRQvK0uA1tlTpmc1JKGRa4+zQLlnooLh29o1lfa\nR+g/u4pHxQKBgAk4gXeqpBAvTb/OxkukWjL/sCiaa0FXxm/VrTLjQLymjCBkQ1jk\nMHszFkfH2p1MLudgTwIIXX/QlYDo81xsWbE1ajMW86U+uImxBG+zkvfBPgrheBUb\n+BXMXnYEoDd2b0+fQ7KTLdoGvMvs9f12K6rC3eHUFxmznjkNDMzzl0iZAoGANfdm\nTwGhYedXkV9bGp/t/BRHmI48yZSj3I4w2CHg/x/gA6Ji5SkD39wohTZhMqzv6Dsj\nQPvpiR/CE8mnqT0K+nme4DORlgQEi9aA3QSXjPEkRIanVTNp8kcfzB4NJvFTBjoF\nYzGNklM6jWNtrUafbfm9vsqPH2l8sipv2LtLKJ0CgYB//iLmjLBS/Qqsiqz7Y5GK\n24jrL6ZPPpjJ6ms7oPHiHPhRqnnxPaoZ/eakUFdr1diWk6vxZUv/6Fnhr313i14A\nMQwn/d3abKyHpj5DI6/5c4OjLoLHFcBZgWkv+/cZcNhkau92pMQUUWZqJdPjK2EQ\nYJE8pVOLiszikCAO1IL8mA==\n-----END PRIVATE KEY-----\n",
		"client_email": "dummy-service-account@livepeerjs-231617.iam.gserviceaccount.com",
		"client_id": "112693155080511457268",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/dummy-service-account%40livepeerjs-231617.iam.gserviceaccount.com"
	}`

	// Set up test file
	file, err := ioutil.TempFile("", "gs-code")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())
	file.WriteString(testGSToken)

	// Set up test URL from filesystem when trusted
	u, _ := url.Parse(fmt.Sprintf("gs://bucket-name?keyfile=%s", file.Name()))
	prepared, err := PrepareOSURL(u.String())
	assert.Equal(nil, err)
	os, err := ParseOSURL(prepared, true)
	assert.Equal(nil, err)
	gs, ok := os.(*gsOS)
	assert.Equal(true, ok)
	assert.Equal("https://bucket-name.storage.googleapis.com", gs.s3OS.host)
	assert.Equal("bucket-name", gs.s3OS.bucket)

	// Also test embedding the thing in the URL itself
	u = &url.URL{
		Scheme: "gs",
		Host:   "bucket-name",
		User:   url.User(testGSToken),
	}
	os, err = ParseOSURL(u.String(), false)
	assert.Equal(nil, err)
	gs, ok = os.(*gsOS)
	assert.Equal(true, ok)
	assert.Equal("https://bucket-name.storage.googleapis.com", gs.s3OS.host)
	assert.Equal("bucket-name", gs.s3OS.bucket)
}
