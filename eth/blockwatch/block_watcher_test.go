package blockwatch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var config = Config{
	PollingInterval:     1 * time.Second,
	BlockRetentionLimit: 10,
	StartBlockDepth:     rpc.LatestBlockNumber,
	WithLogs:            false,
	Topics:              []common.Hash{},
}

var basicFakeClientFixture = "testdata/fake_client_basic_fixture.json"

func TestWatcher(t *testing.T) {
	fakeClient, err := newFakeClient("testdata/fake_client_block_poller_fixtures.json")
	require.NoError(t, err)

	// Polling interval unused because we hijack the ticker for this test
	config.Store = &stubMiniHeaderStore{}
	config.Client = fakeClient
	watcher := New(config)

	// Having a buffer of 1 unblocks the below for-loop without resorting to a goroutine
	events := make(chan []*Event, 1)
	sub := watcher.Subscribe(events)

	for i := 0; i < fakeClient.NumberOfTimesteps(); i++ {
		scenarioLabel := fakeClient.GetScenarioLabel()

		retainedBlocks, err := watcher.InspectRetainedBlocks()
		require.NoError(t, err)
		expectedRetainedBlocks := fakeClient.ExpectedRetainedBlocks()
		assert.Equal(t, expectedRetainedBlocks, retainedBlocks, scenarioLabel)

		expectedEvents := fakeClient.GetEvents()
		if len(expectedEvents) != 0 {
			select {
			case gotEvents := <-events:
				assert.Equal(t, expectedEvents, gotEvents, scenarioLabel)

			case <-time.After(3 * time.Second):
				t.Fatal("Timed out waiting for Events channel to deliver expected events")
			}
		}

		fakeClient.IncrementTimestep()

		if i == fakeClient.NumberOfTimesteps()-1 {
			sub.Unsubscribe()
		}
	}
}

func TestWatcherStartStop(t *testing.T) {
	fakeClient, err := newFakeClient(basicFakeClientFixture)
	require.NoError(t, err)

	config.Store = &stubMiniHeaderStore{}
	config.Client = fakeClient
	watcher := New(config)

	// Start the watcher in a goroutine. We use the done channel to signal when
	// watcher.Watch returns.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer cancel()
	go func() {
		require.NoError(t, watcher.Watch(ctx))
		done <- struct{}{}
	}()

	// Wait a bit and then stop the watcher by calling cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Make sure that the watcher actually stops.
	select {
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for watcher to stop")
	case <-done:
		break
	}
}

type blockRangeChunksTestCase struct {
	from                int
	to                  int
	expectedBlockRanges []*blockRange
}

func TestGetSubBlockRanges(t *testing.T) {
	rangeSize := 6
	testCases := []blockRangeChunksTestCase{
		blockRangeChunksTestCase{
			from: 10,
			to:   10,
			expectedBlockRanges: []*blockRange{
				&blockRange{
					FromBlock: 10,
					ToBlock:   10,
				},
			},
		},
		blockRangeChunksTestCase{
			from: 10,
			to:   16,
			expectedBlockRanges: []*blockRange{
				&blockRange{
					FromBlock: 10,
					ToBlock:   15,
				},
				&blockRange{
					FromBlock: 16,
					ToBlock:   16,
				},
			},
		},
		blockRangeChunksTestCase{
			from: 10,
			to:   22,
			expectedBlockRanges: []*blockRange{
				&blockRange{
					FromBlock: 10,
					ToBlock:   15,
				},
				&blockRange{
					FromBlock: 16,
					ToBlock:   21,
				},
				&blockRange{
					FromBlock: 22,
					ToBlock:   22,
				},
			},
		},
		blockRangeChunksTestCase{
			from: 10,
			to:   24,
			expectedBlockRanges: []*blockRange{
				&blockRange{
					FromBlock: 10,
					ToBlock:   15,
				},
				&blockRange{
					FromBlock: 16,
					ToBlock:   21,
				},
				&blockRange{
					FromBlock: 22,
					ToBlock:   24,
				},
			},
		},
	}

	fakeClient, err := newFakeClient(basicFakeClientFixture)
	require.NoError(t, err)
	config.Store = &stubMiniHeaderStore{}
	config.Client = fakeClient
	watcher := New(config)

	for _, testCase := range testCases {
		blockRanges := watcher.getSubBlockRanges(testCase.from, testCase.to, rangeSize)
		assert.Equal(t, testCase.expectedBlockRanges, blockRanges)
	}
}

func TestGetMissedEventsToBackfillSomeMissed(t *testing.T) {
	// Fixture will return block 30 as the tip of the chain
	fakeClient, err := newFakeClient("testdata/fake_client_fast_sync_fixture.json")
	require.NoError(t, err)

	store := &stubMiniHeaderStore{}
	// Add block number 5 as the last block seen by BlockWatcher
	preLastBlockSeen := &MiniHeader{
		Number: big.NewInt(5),
		Hash:   common.HexToHash("0x293b9ea024055a3e9eddbf9b9383dc7731744111894af6aa038594dc1b61f87f"),
		Parent: common.HexToHash("0x26b13ac89500f7fcdd141b7d1b30f3a82178431eca325d1cf10998f9d68ff5ba"),
	}
	err = store.InsertMiniHeader(preLastBlockSeen)

	require.NoError(t, err)

	config.Store = store
	config.Client = fakeClient
	watcher := New(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := watcher.getMissedEventsToBackfill(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, events, 2)

	// Check that block 30 is now in the DB, and block 5 was removed.
	headers, err := store.FindAllMiniHeadersSortedByNumber()
	require.NoError(t, err)
	require.Len(t, headers, 1)
	assert.Equal(t, big.NewInt(30), headers[0].Number)
}

func TestGetMissedEventsToBackfillNoneMissed(t *testing.T) {
	// Fixture will return block 5 as the tip of the chain
	fakeClient, err := newFakeClient("testdata/fake_client_basic_fixture.json")
	require.NoError(t, err)

	store := &stubMiniHeaderStore{}
	// Add block number 5 as the last block seen by BlockWatcher
	lastBlockSeen := &MiniHeader{
		Number: big.NewInt(5),
		Hash:   common.HexToHash("0x293b9ea024055a3e9eddbf9b9383dc7731744111894af6aa038594dc1b61f87f"),
		Parent: common.HexToHash("0x26b13ac89500f7fcdd141b7d1b30f3a82178431eca325d1cf10998f9d68ff5ba"),
	}
	err = store.InsertMiniHeader(lastBlockSeen)
	require.NoError(t, err)

	config.Store = store
	config.Client = fakeClient
	watcher := New(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := watcher.getMissedEventsToBackfill(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, events, 0)

	// Check that block 5 is still in the DB
	headers, err := store.FindAllMiniHeadersSortedByNumber()
	require.NoError(t, err)
	require.Len(t, headers, 1)
	assert.Equal(t, big.NewInt(5), headers[0].Number)
}

func TestGetMissedEventsToBackfill_NOOP(t *testing.T) {
	// No last retained block
	// No config.BackfillStartBlock

	fakeClient, err := newFakeClient("testdata/fake_client_basic_fixture.json")
	require.NoError(t, err)

	store := &stubMiniHeaderStore{}

	config.Store = store
	config.Client = fakeClient
	watcher := New(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := watcher.getMissedEventsToBackfill(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	headers, err := store.FindAllMiniHeadersSortedByNumber()
	require.NoError(t, err)
	require.Len(t, headers, 1)
}

var logStub = types.Log{
	Address: common.HexToAddress("0x21ab6c9fac80c59d401b37cb43f81ea9dde7fe34"),
	Topics: []common.Hash{
		common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
		common.HexToHash("0x0000000000000000000000004d8a4aa1f304f9632cf3877473445d85c577fe5d"),
		common.HexToHash("0x0000000000000000000000004bdd0d16cfa18e33860470fc4d65c6f5cee60959"),
	},
	Data:        common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000337ad34c0"),
	BlockNumber: uint64(30),
	TxHash:      common.HexToHash("0xd9bb5f9e888ee6f74bedcda811c2461230f247c205849d6f83cb6c3925e54586"),
	TxIndex:     uint(0),
	BlockHash:   common.HexToHash("0x6bbf9b6e836207ab25379c20e517a89090cbbaf8877746f6ed7fb6820770816b"),
	Index:       uint(0),
	Removed:     false,
}

var errUnexpected = errors.New("Something unexpected")

type filterLogsRecursivelyTestCase struct {
	Label                     string
	rangeToFilterLogsResponse map[string]filterLogsResponse
	Err                       error
	Logs                      []types.Log
}

func TestFilterLogsRecursively(t *testing.T) {
	from := 10
	to := 20
	testCases := []filterLogsRecursivelyTestCase{
		filterLogsRecursivelyTestCase{
			Label: "HAPPY_PATH",
			rangeToFilterLogsResponse: map[string]filterLogsResponse{
				"10-20": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs: []types.Log{logStub},
		},
		filterLogsRecursivelyTestCase{
			Label: "TOO_MANY_RESULTS_INFURA_ERROR",
			rangeToFilterLogsResponse: map[string]filterLogsResponse{
				"10-20": filterLogsResponse{
					Err: errors.New(infuraTooManyResultsErrMsg),
				},
				"10-15": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				"16-20": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs: []types.Log{logStub, logStub},
		},
		filterLogsRecursivelyTestCase{
			Label: "TOO_MANY_RESULTS_INFURA_ERROR_DEEPER_RECURSION",
			rangeToFilterLogsResponse: map[string]filterLogsResponse{
				"10-20": filterLogsResponse{
					Err: errors.New(infuraTooManyResultsErrMsg),
				},
				"10-15": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				"16-20": filterLogsResponse{
					Err: errors.New(infuraTooManyResultsErrMsg),
				},
				"16-18": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				"19-20": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs: []types.Log{logStub, logStub, logStub},
		},
		filterLogsRecursivelyTestCase{
			Label: "TOO_MANY_RESULTS_INFURA_ERROR_DEEPER_RECURSION_FAILURE",
			rangeToFilterLogsResponse: map[string]filterLogsResponse{
				"10-20": filterLogsResponse{
					Err: errors.New(infuraTooManyResultsErrMsg),
				},
				"10-15": filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				"16-20": filterLogsResponse{
					Err: errUnexpected,
				},
			},
			Err: errUnexpected,
		},
		filterLogsRecursivelyTestCase{
			Label: "UNEXPECTED_ERROR",
			rangeToFilterLogsResponse: map[string]filterLogsResponse{
				"10-20": filterLogsResponse{
					Err: errUnexpected,
				},
			},
			Err: errUnexpected,
		},
	}

	config.Store = &stubMiniHeaderStore{}

	for _, testCase := range testCases {
		fakeLogClient, err := newFakeLogClient(testCase.rangeToFilterLogsResponse)
		require.NoError(t, err)
		config.Client = fakeLogClient
		watcher := New(config)

		logs, err := watcher.filterLogsRecursively(from, to, []types.Log{})
		require.Equal(t, testCase.Err, err, testCase.Label)
		require.Equal(t, testCase.Logs, logs, testCase.Label)
		assert.Equal(t, len(testCase.rangeToFilterLogsResponse), fakeLogClient.Count())
	}
}

type logsInBlockRangeTestCase struct {
	Label                     string
	From                      int
	To                        int
	RangeToFilterLogsResponse map[string]filterLogsResponse
	Logs                      []types.Log
	FurthestBlockProcessed    int
}

func TestGetLogsInBlockRange(t *testing.T) {
	from := 10
	to := 20
	testCases := []logsInBlockRangeTestCase{
		logsInBlockRangeTestCase{
			Label: "HAPPY_PATH",
			From:  from,
			To:    to,
			RangeToFilterLogsResponse: map[string]filterLogsResponse{
				aRange(from, to): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs:                   []types.Log{logStub},
			FurthestBlockProcessed: to,
		},
		logsInBlockRangeTestCase{
			Label: "SPLIT_REQUEST_BY_MAX_BLOCKS_IN_QUERY",
			From:  from,
			To:    from + maxBlocksInGetLogsQuery + 10,
			RangeToFilterLogsResponse: map[string]filterLogsResponse{
				aRange(from, from+maxBlocksInGetLogsQuery-1): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				aRange(from+maxBlocksInGetLogsQuery, from+maxBlocksInGetLogsQuery+10): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs:                   []types.Log{logStub, logStub},
			FurthestBlockProcessed: from + maxBlocksInGetLogsQuery + 10,
		},
		logsInBlockRangeTestCase{
			Label: "SHORT_CIRCUIT_SEMAPHORE_BLOCKED_REQUESTS_ON_ERROR",
			From:  from,
			To:    from + (maxBlocksInGetLogsQuery * (getLogsRequestChunkSize + 1)),
			RangeToFilterLogsResponse: map[string]filterLogsResponse{
				// Same number of responses as the getLogsRequestChunkSize since the
				// error response will stop any further requests.
				aRange(from, from+maxBlocksInGetLogsQuery-1): filterLogsResponse{
					Err: errUnexpected,
				},
				aRange(from+maxBlocksInGetLogsQuery, from+(maxBlocksInGetLogsQuery*2)-1): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				aRange(from+(maxBlocksInGetLogsQuery*2), from+(maxBlocksInGetLogsQuery*3)-1): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
			},
			Logs:                   []types.Log{},
			FurthestBlockProcessed: from - 1,
		},
		logsInBlockRangeTestCase{
			Label: "CORRECT_FURTHEST_BLOCK_PROCESSED_ON_ERROR",
			From:  from,
			To:    from + maxBlocksInGetLogsQuery + 10,
			RangeToFilterLogsResponse: map[string]filterLogsResponse{
				aRange(from, from+maxBlocksInGetLogsQuery-1): filterLogsResponse{
					Logs: []types.Log{
						logStub,
					},
				},
				aRange(from+maxBlocksInGetLogsQuery, from+maxBlocksInGetLogsQuery+10): filterLogsResponse{
					Err: errUnexpected,
				}},
			Logs:                   []types.Log{logStub},
			FurthestBlockProcessed: from + maxBlocksInGetLogsQuery - 1,
		},
	}

	config.Store = &stubMiniHeaderStore{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, testCase := range testCases {
		fakeLogClient, err := newFakeLogClient(testCase.RangeToFilterLogsResponse)
		require.NoError(t, err)
		config.Client = fakeLogClient
		watcher := New(config)

		logs, furthestBlockProcessed := watcher.getLogsInBlockRange(ctx, testCase.From, testCase.To)
		require.Equal(t, testCase.FurthestBlockProcessed, furthestBlockProcessed, testCase.Label)
		require.Equal(t, testCase.Logs, logs, testCase.Label)
		assert.Equal(t, len(testCase.RangeToFilterLogsResponse), fakeLogClient.Count())
	}
}

func aRange(from, to int) string {
	r := fmt.Sprintf("%d-%d", from, to)
	return r
}

func TestEnrichWithL1_Empty(t *testing.T) {
	assert := assert.New(t)

	w := New(config)
	res := w.enrichWithL1([]*Event{})
	assert.NotNil(res)
	assert.Empty(res)
}

func TestEnrichWithL1_One(t *testing.T) {
	assert := assert.New(t)

	config.Client = &fakeClient{}
	w := New(config)
	header := &MiniHeader{
		Number: big.NewInt(FakeBlockNumber),
		Hash:   common.HexToHash(FakeHash),
		Logs:   []types.Log{logStub},
	}
	events := []*Event{{BlockHeader: header}}

	res := w.enrichWithL1(events)

	assert.Len(res, 1)
	assert.Equal(common.HexToHash(FakeHash), res[0].BlockHeader.Hash)
	assert.Equal(big.NewInt(FakeBlockNumber), res[0].BlockHeader.Number)
	assert.Equal(big.NewInt(FakeL1BlockNumber), res[0].BlockHeader.L1BlockNumber)
	assert.Contains(res[0].BlockHeader.Logs, logStub)
}

func TestEnrichWithL1_Multiple(t *testing.T) {
	assert := assert.New(t)

	config.Client = &fakeClient{}
	w := New(config)
	highestHeader := &MiniHeader{
		Number: big.NewInt(FakeBlockNumber),
		Hash:   common.HexToHash(FakeHash),
	}
	events := []*Event{
		{BlockHeader: &MiniHeader{Number: big.NewInt(1)}},
		{BlockHeader: highestHeader},
		{BlockHeader: &MiniHeader{Number: big.NewInt(2)}},
	}

	res := w.enrichWithL1(events)

	assert.Len(res, 3)
	// highest block number event has L1 block number
	assert.Equal(common.HexToHash(FakeHash), res[1].BlockHeader.Hash)
	assert.Equal(big.NewInt(FakeBlockNumber), res[1].BlockHeader.Number)
	assert.Equal(big.NewInt(FakeL1BlockNumber), res[1].BlockHeader.L1BlockNumber)
	// other events do not have L1 block numbers
	assert.Nil(res[0].BlockHeader.L1BlockNumber)
	assert.Nil(res[2].BlockHeader.L1BlockNumber)
}

func TestEnrichWithL1BlockNumber(t *testing.T) {
	assert := assert.New(t)

	config.Client = &fakeClient{}
	w := New(config)
	header := &MiniHeader{
		Number: big.NewInt(FakeBlockNumber),
		Hash:   common.HexToHash(FakeHash),
	}

	res, err := w.enrichWithL1BlockNumber(header)

	assert.NoError(err)
	assert.Equal(common.HexToHash(FakeHash), res.Hash)
	assert.Equal(big.NewInt(FakeBlockNumber), res.Number)
	assert.Equal(big.NewInt(FakeL1BlockNumber), res.L1BlockNumber)
}
