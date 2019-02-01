package eth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRemoteNonceReader struct {
	mock.Mock
}

func (m *mockRemoteNonceReader) PendingNonceAt(ctx context.Context, addr ethcommon.Address) (uint64, error) {
	args := m.Called(ctx, addr)

	return args.Get(0).(uint64), args.Error(1)
}

func TestNext_PendingNonceAtError(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), errors.New("PendingNonceAt error"))

	assert := assert.New(t)

	_, err := nm.Next(addr)
	assert.NotNil(err)
	assert.Equal("PendingNonceAt error", err.Error())
}

func TestNext_ZeroNonce(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), nil)

	assert := assert.New(t)

	nonce, err := nm.Next(addr)
	assert.Nil(err)
	assert.Equal(uint64(0), nonce)
}

func TestNext_IncrementLocal(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), nil)

	assert := assert.New(t)
	require := require.New(t)

	nonce, err := nm.Next(addr)
	require.Nil(err)
	require.Equal(uint64(0), nonce)

	nm.Update(addr, nonce)

	nonce, err = nm.Next(addr)
	assert.Nil(err)
	assert.Equal(uint64(1), nonce)
}

func TestNext_LocalLowerThanRemote(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(10), nil)

	assert := assert.New(t)

	nonce, err := nm.Next(addr)
	assert.Nil(err)
	assert.Equal(uint64(10), nonce)
}

func TestUpdate_LastNonceEqualsLocalNonce(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), nil)

	nm.Update(addr, uint64(0))

	nonce, err := nm.Next(addr)
	require.Nil(t, err)

	assert.Equal(t, uint64(1), nonce)
}

func TestUpdate_LastNonceNotEqualsLocalNonce(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), nil)

	nm.Update(addr, uint64(10))

	nonce, err := nm.Next(addr)
	require.Nil(t, err)

	assert.Equal(t, uint64(11), nonce)
}

func TestNextAndUpdate_ConcurrentMultipleAddrs(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)

	r.On("PendingNonceAt", mock.Anything, mock.Anything).Return(uint64(0), nil)

	var wg sync.WaitGroup
	var errCount uint64
	var addrs [50]ethcommon.Address

	for i := 0; i < 50; i++ {
		wg.Add(1)

		addrs[i] = pm.RandAddress()

		go func(addr ethcommon.Address) {
			defer wg.Done()

			nm.Lock(addr)
			defer nm.Unlock(addr)

			nonce, err := nm.Next(addr)
			if err != nil {
				atomic.AddUint64(&errCount, 1)
			}

			if err == nil {
				nm.Update(addr, nonce)
			}
		}(addrs[i])
	}

	wg.Wait()

	assert := assert.New(t)

	assert.Equal(uint64(0), errCount)

	for _, addr := range addrs {
		nonce, err := nm.Next(addr)
		assert.Nil(err)
		assert.Equal(uint64(1), nonce)
	}
}

func TestNextAndUpdate_ConcurrentSingleAddr(t *testing.T) {
	r := &mockRemoteNonceReader{}
	nm := NewNonceManager(r)
	addr := pm.RandAddress()

	r.On("PendingNonceAt", mock.Anything, addr).Return(uint64(0), nil)

	var wg sync.WaitGroup
	var errCount uint64

	var usedNoncesLock sync.Mutex
	usedNonces := make(map[uint64]bool)

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			nm.Lock(addr)
			defer nm.Unlock(addr)

			nonce, err := nm.Next(addr)
			if err != nil {
				atomic.AddUint64(&errCount, 1)
			}

			usedNoncesLock.Lock()
			usedNonces[nonce] = true
			usedNoncesLock.Unlock()

			if err == nil {
				nm.Update(addr, nonce)
			}
		}()
	}

	wg.Wait()

	assert := assert.New(t)

	assert.Equal(uint64(0), errCount)

	nonce, err := nm.Next(addr)
	assert.Nil(err)
	assert.Equal(uint64(50), nonce)

	for i := 0; i < 50; i++ {
		assert.True(usedNonces[uint64(i)])
	}
}
