package drivers

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func testFileInfoReader(fn, body string) *FileInfoReader {
	fi := &FileInfoReader{
		FileInfo: FileInfo{
			Name: fn,
		},
		Body: ioutil.NopCloser(bytes.NewReader([]byte(body))),
	}
	return fi
}

func TestReaderPoolShouldReturnError(t *testing.T) {
	assert := assert.New(t)

	mos := &MockOSSession{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mos.On("ReadData", ctx, "f1").Return(testFileInfoReader("f1", "body 1"), nil)
	mos.On("ReadData", ctx, "f2").Return(nil, errors.New("ReadData error"))
	filesNames := []string{"f1", "f2"}

	fis, data, err := ParallelReadFiles(ctx, mos, filesNames, 2)
	assert.Len(fis, 2)
	assert.Len(data, 2)
	assert.Equal(data[0], []byte("body 1"))
	assert.Equal(fis[0].Name, "f1")
	assert.Nil(fis[1])
	if assert.Error(err) {
		assert.Equal(err.Error(), "ReadData error")
	}
}

func TestReaderPoolShouldReadInParallel(t *testing.T) {
	assert := assert.New(t)

	mos := &MockOSSession{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f1e := make(chan time.Time)
	mos.On("ReadData", ctx, "f1").WaitUntil(f1e).Return(testFileInfoReader("f1", "body 1"), nil)
	mos.On("ReadData", ctx, "f2").Run(func(args mock.Arguments) {
		close(f1e)
	}).Return(testFileInfoReader("f2", "body 2"), nil)

	filesNames := []string{"f1", "f2"}

	fis, data, err := ParallelReadFiles(ctx, mos, filesNames, 2)
	assert.Len(fis, 2)
	assert.Len(data, 2)
	assert.Equal(data[0], []byte("body 1"))
	assert.Equal(fis[0].Name, "f1")
	assert.Equal(data[1], []byte("body 2"))
	assert.Equal(fis[1].Name, "f2")
	assert.Nil(err)
}
