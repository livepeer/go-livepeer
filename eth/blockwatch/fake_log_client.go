package blockwatch

import (
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type filterLogsResponse struct {
	Logs []types.Log
	Err  error
}

// fakeLogClient is a fake Client for testing code calling the `FilterLogs` method.
// It allows the instatiator to specify `FilterLogs` responses for several block ranges.
type fakeLogClient struct {
	count           int64
	rangeToResponse map[string]filterLogsResponse
}

// newFakeLogClient instantiates a fakeLogClient for testing log fetching
func newFakeLogClient(rangeToResponse map[string]filterLogsResponse) (*fakeLogClient, error) {
	return &fakeLogClient{count: 0, rangeToResponse: rangeToResponse}, nil
}

// HeaderByNumber fetches a block header by its number
func (fc *fakeLogClient) HeaderByNumber(number *big.Int) (*MiniHeader, error) {
	return nil, errors.New("NOT_IMPLEMENTED")
}

// HeaderByHash fetches a block header by its block hash
func (fc *fakeLogClient) HeaderByHash(hash common.Hash) (*MiniHeader, error) {
	return nil, errors.New("NOT_IMPLEMENTED")
}

// FilterLogs returns the logs that satisfy the supplied filter query
func (fc *fakeLogClient) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	// Add a slight delay to simulate an actual network request. This also gives
	// BlockWatcher.getLogsInBlockRange multi-requests to hit the concurrent request
	// limit semaphore and simulate more realistic conditions.
	time.Sleep(5 * time.Millisecond)
	r := toRange(q.FromBlock, q.ToBlock)
	res, ok := fc.rangeToResponse[r]
	if !ok {
		return nil, fmt.Errorf("Didn't find response for range %s but  was expecting it to exist", r)
	}
	atomic.AddInt64(&fc.count, 1)
	return res.Logs, res.Err
}

// Count returns the number of times FilterLogs was called
func (fc *fakeLogClient) Count() int {
	return int(fc.count)
}

func toRange(from, to *big.Int) string {
	r := fmt.Sprintf("%s-%s", from, to)
	return r
}
