package drivers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestS3URL(t *testing.T) {
	assert := assert.New(t)
	os, err := ParseOSURL("s3://user:xxxxxxxxx%2Bxxxxxxxxx%2Fxxxxxxxx%2Bxxxxxxxxxxxxx@us-west-2/example-bucket")
	assert.Equal(nil, err)
	s3, ok := os.(*s3OS)
	assert.Equal(true, ok)
	assert.Equal("user", s3.awsAccessKeyID)
	assert.Equal("xxxxxxxxx+xxxxxxxxx/xxxxxxxx+xxxxxxxxxxxxx", s3.awsSecretAccessKey)
	assert.Equal("us-west-2", s3.region)
	assert.Equal("example-bucket", s3.bucket)
}
