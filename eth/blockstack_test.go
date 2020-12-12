package eth

import (
	"errors"
	"sync"
	"testing"

	"github.com/0xProject/0x-mesh/ethereum/miniheader"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMiniHeaderStore struct {
	headers           []*miniheader.MiniHeader
	latestErr         error
	sortedByNumberErr error
	insertErr         error
	deleteErr         error
}

func (s *stubMiniHeaderStore) FindLatestMiniHeader() (*miniheader.MiniHeader, error) {
	if s.latestErr != nil {
		return nil, s.latestErr
	}

	if len(s.headers) == 0 {
		return nil, nil
	}

	return s.headers[len(s.headers)-1], nil
}

func (s *stubMiniHeaderStore) FindAllMiniHeadersSortedByNumber() ([]*miniheader.MiniHeader, error) {
	if s.sortedByNumberErr != nil {
		return nil, s.sortedByNumberErr
	}

	return s.headers, nil
}

func (s *stubMiniHeaderStore) InsertMiniHeader(header *miniheader.MiniHeader) error {
	if s.insertErr != nil {
		return s.insertErr
	}

	s.headers = append(s.headers, header)

	return nil
}

func (s *stubMiniHeaderStore) DeleteMiniHeader(hash ethcommon.Hash) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	for i, header := range s.headers {
		if header.Hash == hash {
			copy(s.headers[i:], s.headers[i+1:])
			s.headers[len(s.headers)-1] = nil
			s.headers = s.headers[:len(s.headers)-1]
			return nil
		}
	}

	return errors.New("MiniHeader not found")
}

func TestPop(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 10)

	assert := assert.New(t)
	require := require.New(t)

	// Test when store.FindLatestMiniHeader() returns error
	store.latestErr = errors.New("FindLatestMiniHeader error")
	_, err := stack.Pop()
	assert.EqualError(err, store.latestErr.Error())

	// Test when store returns nil MiniHeader
	store.latestErr = nil
	header, err := stack.Pop()
	assert.Nil(header)
	assert.Nil(err)

	h0 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h0"))}
	require.Nil(store.InsertMiniHeader(h0))

	// Test when store.DeleteMiniHeader() returns error
	store.deleteErr = errors.New("DeleteMiniHeader error")
	_, err = stack.Pop()
	assert.EqualError(err, store.deleteErr.Error())

	// Test header popped when size = 1
	store.deleteErr = nil
	header, err = stack.Pop()
	assert.Equal(h0, header)
	assert.Nil(err)
	assert.Equal(0, len(store.headers))

	// Test header popped when size > 1
	h1 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h1"))}
	require.Nil(store.InsertMiniHeader(h0))
	require.Nil(store.InsertMiniHeader(h1))

	header, err = stack.Pop()
	assert.Equal(h1, header)
	assert.Nil(err)
	assert.Equal(1, len(store.headers))
}

func TestPopConcurrent(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 10)

	assert := assert.New(t)
	require := require.New(t)

	// Insert headers into store
	headerMap := make(map[ethcommon.Hash]bool)
	for i := 0; i < 10; i++ {
		hash := ethcommon.BytesToHash([]byte(string(rune(i))))
		headerMap[hash] = true

		require.Nil(store.InsertMiniHeader(&miniheader.MiniHeader{Hash: hash}))
	}

	// Ensure that headerMap starts off with the correct # of entries
	assert.Equal(10, len(headerMap))

	popCh := make(chan *miniheader.MiniHeader)

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			header, err := stack.Pop()
			require.Nil(err)

			popCh <- header

			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(popCh)
	}()

	// Mark each header as popped in headerMap
	for h := range popCh {
		_, ok := headerMap[h.Hash]
		assert.True(ok)

		delete(headerMap, h.Hash)
	}

	// headerMap should have 0 entries
	assert.Equal(0, len(headerMap))
	assert.Equal(0, len(store.headers))
}

func TestPush(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 2)

	assert := assert.New(t)
	require := require.New(t)

	// Test when store.FindAllMiniHeadersSortedByNumber() returns error
	store.sortedByNumberErr = errors.New("FindAllMiniHeadersSortedByNumber error")
	h0 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h0"))}
	err := stack.Push(h0)
	assert.EqualError(err, store.sortedByNumberErr.Error())

	// Test stack at limit and store.DeleteMiniHeader() returns error
	h1 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h1"))}
	require.Nil(store.InsertMiniHeader(h0))
	require.Nil(store.InsertMiniHeader(h1))

	h2 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h2"))}
	store.sortedByNumberErr = nil
	store.deleteErr = errors.New("DeleteMiniHeader error")
	err = stack.Push(h2)
	assert.EqualError(err, store.deleteErr.Error())

	// Test stack at limit, deleted bottom and store.InsertMiniHeader() returns error
	store.deleteErr = nil
	store.insertErr = errors.New("InsertMiniHeader error")
	err = stack.Push(h2)
	assert.EqualError(err, store.insertErr.Error())
	assert.Equal(h1, store.headers[0])
	assert.Equal(1, len(store.headers))

	// Test stack at limit, deleted bottom and header inserted
	store.insertErr = nil
	require.Nil(store.InsertMiniHeader(h2))

	h3 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h3"))}
	err = stack.Push(h3)
	assert.Nil(err)
	assert.Equal(h3, store.headers[len(store.headers)-1])
	assert.Equal(h2, store.headers[0])
	assert.Equal(2, len(store.headers))

	// Test stack not at limit and store.InsertMiniHeader() returns error
	require.Nil(store.DeleteMiniHeader(h2.Hash))

	h4 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h4"))}
	store.insertErr = errors.New("InsertMiniHeader error")
	err = stack.Push(h4)
	assert.EqualError(err, store.insertErr.Error())

	// Test not at limit and header inserted
	store.insertErr = nil
	err = stack.Push(h4)
	assert.Nil(err)
	assert.Equal(h4, store.headers[len(store.headers)-1])
	assert.Equal(2, len(store.headers))
}

func TestPushConcurrent(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 10)

	assert := assert.New(t)
	require := require.New(t)

	headerHash := func(i int) ethcommon.Hash {
		return ethcommon.BytesToHash([]byte(string(rune(i))))
	}

	// Insert headers into store
	for i := 0; i < 10; i++ {
		require.Nil(store.InsertMiniHeader(&miniheader.MiniHeader{Hash: headerHash(i)}))
	}

	// Create headerMap with expected entries after all stack pushes are complete
	headerMap := make(map[ethcommon.Hash]bool)
	for i := 10; i < 20; i++ {
		headerMap[headerHash(i)] = true
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 10; i < 20; i++ {
		go func(i int) {
			err := stack.Push(&miniheader.MiniHeader{Hash: headerHash(i)})
			require.Nil(err)

			wg.Done()
		}(i)
	}

	wg.Wait()

	headers, err := stack.PeekAll()
	require.Nil(err)

	for _, h := range headers {
		_, ok := headerMap[h.Hash]
		assert.True(ok)

		delete(headerMap, h.Hash)
	}

	// headerMap should have 0 entries
	assert.Equal(0, len(headerMap))
	assert.Equal(10, len(store.headers))
}

func TestPeek(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 10)

	assert := assert.New(t)

	// Test store.FindLatestMiniHeader() returns error
	store.latestErr = errors.New("FindLatestMiniHeader error")
	_, err := stack.Peek()
	assert.EqualError(err, store.latestErr.Error())

	// Test header returned
	h := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h"))}
	require.Nil(t, store.InsertMiniHeader(h))

	store.latestErr = nil
	header, err := stack.Peek()
	assert.Nil(err)
	assert.Equal(h, header)
}

func TestInspect(t *testing.T) {
	store := &stubMiniHeaderStore{}
	stack := NewBlockStack(store, 10)

	assert := assert.New(t)

	// Test store.FindAllMiniHeadersSortedByNumber() returns error
	store.sortedByNumberErr = errors.New("FindAllMiniHeadersSortedByNumber error")
	_, err := stack.PeekAll()
	assert.EqualError(err, store.sortedByNumberErr.Error())

	// Test headers returned
	h0 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h0"))}
	h1 := &miniheader.MiniHeader{Hash: ethcommon.BytesToHash([]byte("h1"))}
	require.Nil(t, store.InsertMiniHeader(h0))
	require.Nil(t, store.InsertMiniHeader(h1))

	store.sortedByNumberErr = nil
	headers, err := stack.PeekAll()
	assert.Nil(err)
	assert.Equal(2, len(headers))
	assert.Equal(h0, headers[0])
	assert.Equal(h1, headers[1])
}
