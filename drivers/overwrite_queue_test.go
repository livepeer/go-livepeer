package drivers

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// waitForQueueToClear used in tests
func (oq *OverwriteQueue) waitForQueueToClear(timeout time.Duration) {
	// wait for work to be queued
	time.Sleep(5 * time.Millisecond)
	start := time.Now()
	for {
		if len(oq.queue) == 0 {
			return
		}
		if time.Since(start) > timeout {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestOverwriteQueueShouldCallSave(t *testing.T) {
	timeout := 500 * time.Millisecond
	mos := NewMockOSSession()
	oq := NewOverwriteQueue(mos, "f1", "save", 2, timeout, 15*time.Second)

	data1 := []byte("data01")

	var meta map[string]string
	mos.On("SaveData", "f1", dataReader(data1), meta, timeout).Return("not used", nil).Once()

	oq.Save(data1)
	oq.waitForQueueToClear(5 * time.Second)
	mos.AssertExpectations(t)
	oq.StopAfter(0)
	time.Sleep(10 * time.Millisecond)
}

func TestOverwriteQueueShouldRetry(t *testing.T) {
	timeout := 500 * time.Millisecond
	mos := NewMockOSSession()
	oq := NewOverwriteQueue(mos, "f1", "retry", 2, timeout, 15*time.Second)

	data1 := []byte("data01")

	var meta map[string]string
	mos.On("SaveData", "f1", dataReader(data1), meta, timeout).Return("not used", errors.New("no1")).Once()
	timeout = time.Duration(float64(timeout) * timeoutMultiplier)
	mos.On("SaveData", "f1", dataReader(data1), meta, timeout).Return("not used", nil).Once()

	oq.Save(data1)
	oq.waitForQueueToClear(5 * time.Second)
	mos.AssertExpectations(t)
	oq.StopAfter(0)
	time.Sleep(10 * time.Millisecond)
}

func TestOverwriteQueueShouldUseLastValue(t *testing.T) {
	timeout := 300 * time.Millisecond
	mos := NewMockOSSession()
	oq := NewOverwriteQueue(mos, "f1", "use last", 2, timeout, 15*time.Second)

	dataw1 := []byte("dataw01")
	data2 := []byte("data02")
	data3 := []byte("data03")

	var meta map[string]string
	mos.On("SaveData", "f1", dataReader(dataw1), meta, timeout).Return("not used", nil).Once()
	mos.On("SaveData", "f1", dataReader(data3), meta, timeout).Return("not used", nil).Once()

	mos.waitForCh = true
	oq.Save(dataw1)
	<-mos.back
	oq.Save(data2)
	oq.Save(data3)
	mos.waitCh <- struct{}{}

	oq.waitForQueueToClear(5 * time.Second)
	mos.AssertExpectations(t)
	mos.AssertNotCalled(t, "SaveData", "f1", data2, meta, timeout)
	oq.StopAfter(0)
	time.Sleep(10 * time.Millisecond)
}

func dataReader(expected []byte) interface{} {
	return mock.MatchedBy(func(r io.Reader) bool {
		data, err := ioutil.ReadAll(r)
		_, seekErr := r.(*bytes.Reader).Seek(0, 0)
		return err == nil && bytes.Equal(data, expected) && seekErr == nil
	})
}
