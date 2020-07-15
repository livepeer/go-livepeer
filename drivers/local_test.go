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
	path, err := sess.SaveData("name1/1.ts", copyBytes(tempData1), nil)
	glog.Info(path)
	fmt.Println(path)
	assert.Equal("fake.com/url/stream/sesspath/name1/1.ts", path)
	data := sess.GetData("sesspath/name1/1.ts")
	fmt.Printf("got Data: '%s'\n", data)
	assert.Equal(tempData1, string(data))
	path, err = sess.SaveData("name1/1.ts", copyBytes(tempData2), nil)
	data = sess.GetData("sesspath/name1/1.ts")
	assert.Equal(tempData2, string(data))
	path, err = sess.SaveData("name1/2.ts", copyBytes(tempData3), nil)
	data = sess.GetData("sesspath/name1/2.ts")
	assert.Equal(tempData3, string(data))
	// Test trim prefix when baseURI != nil
	data = sess.GetData(path)
	assert.Equal(tempData3, string(data))
	data = sess.GetData("sesspath/name1/1.ts")
	assert.Nil(data)
	sess.EndSession()
	data = sess.GetData("sesspath/name1/2.ts")
	assert.Nil(data)

	// Test trim prefix when baseURI = nil
	os = NewMemoryDriver(nil)
	sess = os.NewSession("sesspath").(*MemorySession)
	path, err = sess.SaveData("name1/1.ts", copyBytes(tempData1), nil)
	assert.Nil(err)
	assert.Equal("/stream/sesspath/name1/1.ts", path)

	data = sess.GetData(path)
	assert.Equal(tempData1, string(data))
}
