package drivers

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
)

func copyBytes(src string) []byte {
	srcb := []byte(src)
	dst := make([]byte, len(srcb))
	copy(dst, srcb)
	return dst
}

func TestLocalOS(t *testing.T) {
	tempData1 := "dataitselftempdata1"
	tempData2 := "dataitselftempdata2"
	tempData3 := "dataitselftempdata3"
	oldDataCacheLen := dataCacheLen
	dataCacheLen = 1
	defer func() {
		dataCacheLen = oldDataCacheLen
	}()
	assert := assert.New(t)
	u, err := url.Parse("fake.com/url")
	assert.NoError((err))
	os := NewMemoryDriver(u)
	sess := os.NewSession(("sesspath")).(*MemorySession)
	path, err := sess.SaveData("name1/1.ts", copyBytes(tempData1))
	glog.Info(path)
	fmt.Println(path)
	assert.Equal("name1/1.ts", path)
	data, err := os.GetData("sesspath/name1/1.ts")
	assert.Nil(err)
	assert.Equal(tempData1, string(data))
	data, err = sess.GetData("name1/1.ts")
	assert.Nil(err)
	assert.Equal(tempData1, string(data))
	path, err = sess.SaveData("name1/1.ts", copyBytes(tempData2))
	assert.Nil(err)
	data, err = os.GetData("sesspath/name1/1.ts")
	assert.Nil(err)
	assert.Equal(tempData2, string(data))
	data, err = sess.GetData("name1/1.ts")
	assert.Nil(err)
	assert.Equal(tempData2, string(data))
	path, err = sess.SaveData("name1/2.ts", copyBytes(tempData3))
	assert.Nil(err)
	data, err = os.GetData("sesspath/name1/2.ts")
	assert.Nil(err)
	assert.Equal(tempData3, string(data))
	data, err = sess.GetData("name1/2.ts")
	assert.Nil(err)
	assert.Equal(tempData3, string(data))

	// Test end session
	sess.EndSession()
	data, err = os.GetData("sesspath/name1/2.ts")
	assert.Equal("memory os: invalid session", err.Error())
	assert.Nil(data)
	data, err = sess.GetData("name1/2.ts")
	assert.Equal("memory os: object does not exist", err.Error())
	assert.Nil(data)
}
