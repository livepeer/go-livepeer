package core

import (
	"errors"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
)

func TestAcceptErr(t *testing.T) {
	sender := pm.RandAddress()
	em := NewErrorMonitor(2, make(chan struct{}))

	ok := em.AcceptErr(sender)
	assert.True(t, ok)
	assert.Equal(t, em.errCount[sender], 1)

	ok = em.AcceptErr(sender)
	assert.True(t, ok)
	assert.Equal(t, em.errCount[sender], 2)

	ok = em.AcceptErr(sender)
	assert.False(t, ok)
	assert.Equal(t, em.errCount[sender], 2)
}

func TestClearErrCount(t *testing.T) {
	sender := pm.RandAddress()
	em := NewErrorMonitor(3, make(chan struct{}))

	em.AcceptErr(sender)
	em.AcceptErr(sender)
	assert.Equal(t, em.errCount[sender], 2)

	em.ClearErrCount(sender)
	assert.Equal(t, em.errCount[sender], 0)
}

func TestResetErrCounts(t *testing.T) {
	sender := pm.RandAddress()
	senderB := pm.RandAddress()
	assert.NotEqual(t, sender, senderB)
	em := NewErrorMonitor(3, make(chan struct{}))

	em.AcceptErr(sender)
	em.AcceptErr(sender)
	em.AcceptErr(senderB)
	assert.Equal(t, em.errCount[sender], 2)
	assert.Equal(t, em.errCount[senderB], 1)

	em.resetErrCounts()
	assert.Equal(t, em.errCount[sender], 0)
	assert.Equal(t, em.errCount[senderB], 0)

}

func TestGasPriceUpdateLoop(t *testing.T) {
	em := NewErrorMonitor(3, make(chan struct{}))
	go em.StartGasPriceUpdateLoop()
	assert := assert.New(t)

	// add some counts for senders
	sender := pm.RandAddress()
	senderB := pm.RandAddress()
	em.AcceptErr(sender)
	em.AcceptErr(sender)
	em.AcceptErr(senderB)
	assert.Equal(em.errCount[sender], 2)
	assert.Equal(em.errCount[senderB], 1)

	// Send a gasPriceUpdate
	em.gasPriceUpdate <- struct{}{}

	time.Sleep(1 * time.Second)

	// Map should be reinitialized
	count, ok := em.errCount[sender]
	assert.False(ok)
	assert.Equal(count, 0)

	count, ok = em.errCount[senderB]
	assert.False(ok)
	assert.Equal(count, 0)
	close(em.gasPriceUpdate)
}

func TestAcceptableError(t *testing.T) {
	expectedErr := &acceptableError{
		err:        errors.New("hello error"),
		acceptable: true,
	}
	assert := assert.New(t)
	// test constructor
	acceptableErr := newAcceptableError(errors.New("hello error"), true)
	assert.Equal(expectedErr, acceptableErr)

	assert.Equal(expectedErr.acceptable, acceptableErr.Acceptable())
	assert.Equal("hello error", acceptableErr.Error())
}
