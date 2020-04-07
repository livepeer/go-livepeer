package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuspender(t *testing.T) {
	assert := assert.New(t)
	s := newSuspender()

	s.suspend("foo", 5)
	assert.Equal(s.Suspended("foo"), int64(5))
	s.suspend("foo", 5)
	assert.Equal(s.Suspended("foo"), int64(10))
	s.count = 11
	assert.Equal(s.Suspended("foo"), int64(0))
	_, ok := s.list["foo"]
	assert.False(ok)

	s.signalRefresh()
	assert.Equal(s.count, int64(12))
}
