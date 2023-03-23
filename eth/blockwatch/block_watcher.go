package blockwatch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/glog"
)

// maxBlocksInGetLogsQuery is the max number of blocks to fetch logs for in a single query. There is
// a hard limit of 10,000 logs returned by a single `eth_getLogs` query by Infura's Ethereum nodes so
// we need to try and stay below it. Parity, Geth and Alchemy all have much higher limits (if any) on
// the number of logs returned so Infura is by far the limiting factor.
var maxBlocksInGetLogsQuery = 1000

// EventType describes the types of events emitted by blockwatch.Watcher. A block can be discovered
// and added to our representation of the chain. During a block re-org, a block previously stored
// can be removed from the list.
type EventType int

const (
	// Added indicates that an event was added to our representation of the chain
	Added EventType = iota
	// Removed indicates that an event was removed from our representation of the chain
	Removed
)

// Event describes a block event emitted by a Watcher
type Event struct {
	Type        EventType
	BlockHeader *MiniHeader
}

// Config holds some configuration options for an instance of BlockWatcher.
type Config struct {
	Store               MiniHeaderStore
	PollingInterval     time.Duration
	StartBlockDepth     rpc.BlockNumber
	BlockRetentionLimit int
	WithLogs            bool
	Topics              []common.Hash
	Client              Client
}

// Watcher maintains a consistent representation of the latest `blockRetentionLimit` blocks,
// handling block re-orgs and network disruptions gracefully. It can be started from any arbitrary
// block height, and will emit both block added and removed events.
type Watcher struct {
	blockRetentionLimit int
	startBlockDepth     rpc.BlockNumber
	stack               *Stack
	client              Client
	blockFeed           event.Feed
	blockScope          event.SubscriptionScope // Subscription scope tracking current live listeners
	wasStartedOnce      bool                    // Whether the block watcher has previously been started
	pollingInterval     time.Duration
	ticker              *time.Ticker
	withLogs            bool
	topics              []common.Hash
	sync.RWMutex
}

// New creates a new Watcher instance.
func New(config Config) *Watcher {
	stack := NewStack(config.Store, config.BlockRetentionLimit)

	bs := &Watcher{
		pollingInterval:     config.PollingInterval,
		blockRetentionLimit: config.BlockRetentionLimit,
		startBlockDepth:     config.StartBlockDepth,
		stack:               stack,
		client:              config.Client,
		withLogs:            config.WithLogs,
		topics:              config.Topics,
	}
	return bs
}

// BackfillEvents finds missed events and sends them to event subscribers.
// It blocks until it is done backfilling or the given context is canceled.
// Note that the latest block is never backfilled here from logs. It will be polled separately in syncToLatestBlock().
// The reason for that is that we always need to propagate events from the latest block even if it does not contain
// events which are filtered out during the backfilling process.
func (w *Watcher) BackfillEvents(ctx context.Context, chainHead *MiniHeader) error {
	var events, err = w.getMissedEventsToBackfill(ctx, chainHead)
	if err != nil {
		return err
	}
	if len(events) > 0 {
		w.blockFeed.Send(w.enrichWithL1(events))
	}
	return nil
}

// Watch starts the Watcher. It will continuously look for new blocks and blocks
// until the given context is canceled. Typically, you want to call Watch inside a goroutine.
func (w *Watcher) Watch(ctx context.Context) error {
	w.Lock()
	if w.wasStartedOnce {
		w.Unlock()
		return errors.New("Can only start Watcher once per instance")
	}
	w.wasStartedOnce = true
	w.Unlock()

	ticker := time.NewTicker(w.pollingInterval)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return nil
		case <-ticker.C:
			if err := w.syncToLatestBlock(ctx); err != nil {
				glog.Errorf("blockwatch.Watcher error encountered - trying again on next polling interval err=%q", err)
			}
		}
	}
}

// Subscribe allows one to subscribe to the block events emitted by the Watcher.
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other subscribers.
// Slow subscribers are not dropped.
func (w *Watcher) Subscribe(sink chan<- []*Event) event.Subscription {
	return w.blockScope.Track(w.blockFeed.Subscribe(sink))
}

// GetLatestBlock returns the latest block processed
func (w *Watcher) GetLatestBlock() (*MiniHeader, error) {
	h, err := w.stack.Peek()
	if err != nil {
		return nil, err
	}
	return w.enrichWithL1BlockNumber(h)
}

// InspectRetainedBlocks returns the blocks retained in-memory by the Watcher instance. It is not
// particularly performant and therefore should only be used for debugging and testing purposes.
func (w *Watcher) InspectRetainedBlocks() ([]*MiniHeader, error) {
	return w.stack.Inspect()
}

func (w *Watcher) syncToLatestBlock(ctx context.Context) error {
	w.Lock()
	defer w.Unlock()

	newestHeader, err := w.client.HeaderByNumber(nil)
	if err != nil {
		return err
	}

	return w.BackfillEvents(ctx, newestHeader)
}

func (w *Watcher) addLogs(header *MiniHeader) (*MiniHeader, error) {
	if !w.withLogs {
		return header, nil
	}
	logs, err := w.client.FilterLogs(ethereum.FilterQuery{
		BlockHash: &header.Hash,
		Topics:    [][]common.Hash{w.topics},
	})
	if err != nil {
		return header, err
	}
	header.Logs = logs
	return header, nil
}

// getMissedEventsToBackfill finds missed events that might have occurred since the last block polling.
// It does this by comparing the last block stored with the latest block discoverable via RPC.
// If the stored block is older than the latest block, it batch-fetches the events for missing blocks,
// re-sets the stored blocks and returns the block events found.
// Note that the latest block is never backfilled, and will be polled separately during in syncToLatestBlock().
func (w *Watcher) getMissedEventsToBackfill(ctx context.Context, chainHead *MiniHeader) ([]*Event, error) {
	events := []*Event{}

	var (
		startBlockNum          int
		latestRetainedBlockNum int
		blocksElapsed          int
	)

	latestRetainedBlock, err := w.stack.Peek()
	if err != nil {
		return events, err
	}

	if chainHead == nil {
		var err error
		chainHead, err = w.client.HeaderByNumber(nil)
		if err != nil {
			return events, err
		}
	}

	latestBlockNum := int(chainHead.Number.Int64())

	if latestRetainedBlock != nil {
		latestRetainedBlockNum = int(latestRetainedBlock.Number.Int64())
		// Events for latestRetainedBlock already processed, start at latestRetainedBlock + 1
		startBlockNum = latestRetainedBlockNum + 1
	} else {
		err = w.stack.Push(chainHead)
		if err != nil {
			return events, err
		}

		events = append(events, &Event{
			Type:        Added,
			BlockHeader: chainHead,
		})
		return events, nil
	}

	if blocksElapsed = latestBlockNum - startBlockNum; blocksElapsed <= 0 {
		return events, nil
	}

	logs, furthestBlockProcessed := w.getLogsInBlockRange(ctx, startBlockNum, latestBlockNum)
	if furthestBlockProcessed > latestRetainedBlockNum {
		// If we have processed blocks further then the latestRetainedBlock in the DB, we
		// want to remove all blocks from the DB and insert the furthestBlockProcessed
		// Doing so will cause the BlockWatcher to start from that furthestBlockProcessed.
		headers, err := w.InspectRetainedBlocks()
		if err != nil {
			return events, err
		}
		for i := 0; i < len(headers); i++ {
			_, err := w.stack.Pop()
			if err != nil {
				return events, err
			}
		}
		// Add furthest block processed into the DB
		var latestHeader *MiniHeader
		if chainHead.Number.Int64() == int64(furthestBlockProcessed) {
			latestHeader = chainHead
		} else {
			latestHeader, err = w.client.HeaderByNumber(big.NewInt(int64(furthestBlockProcessed)))
		}
		if err != nil {
			return events, err
		}
		err = w.stack.Push(latestHeader)
		if err != nil {
			return events, err
		}

		events = append(events, &Event{
			Type:        Added,
			BlockHeader: latestHeader,
		})

		// If no logs found, noop
		if len(logs) == 0 {
			return events, nil
		}

		// Create the block events from all the logs found by grouping
		// them into blockHeaders
		hashToBlockHeader := map[common.Hash]*MiniHeader{}
		for _, log := range logs {
			blockHeader, ok := hashToBlockHeader[log.BlockHash]
			if !ok {
				// TODO: Find a way to include the parent hash for the block as well.
				// It's currently not an issue to omit it since we don't use the parent hash
				// when processing block events in event watcher services
				blockHeader = &MiniHeader{
					Hash:   log.BlockHash,
					Number: big.NewInt(0).SetUint64(log.BlockNumber),
					Logs:   []types.Log{},
				}
				hashToBlockHeader[log.BlockHash] = blockHeader
			}
			blockHeader.Logs = append(blockHeader.Logs, log)
		}
		for _, blockHeader := range hashToBlockHeader {
			events = append(events, &Event{
				Type:        Added,
				BlockHeader: blockHeader,
			})
		}
		return events, nil
	}
	return events, nil
}

type logRequestResult struct {
	From int
	To   int
	Logs []types.Log
	Err  error
}

// getLogsRequestChunkSize is the number of `eth_getLogs` JSON RPC to send concurrently in each batch fetch
const getLogsRequestChunkSize = 3

// getLogsInBlockRange attempts to fetch all logs in the block range supplied. It implements a
// limited-concurrency batch fetch, where all requests in the previous batch must complete for
// the next batch of requests to be sent. If an error is encountered in a batch, all subsequent
// batch requests are not sent. Instead, it returns all the logs it found up until the error was
// encountered, along with the block number after which no further logs were retrieved.
func (w *Watcher) getLogsInBlockRange(ctx context.Context, from, to int) ([]types.Log, int) {
	blockRanges := w.getSubBlockRanges(from, to, maxBlocksInGetLogsQuery)

	numChunks := 0
	chunkChan := make(chan []*blockRange, 1000000)
	for len(blockRanges) != 0 {
		var chunk []*blockRange
		if len(blockRanges) < getLogsRequestChunkSize {
			chunk = blockRanges[:len(blockRanges)]
		} else {
			chunk = blockRanges[:getLogsRequestChunkSize]
		}
		chunkChan <- chunk
		blockRanges = blockRanges[len(chunk):]
		numChunks++
	}

	semaphoreChan := make(chan struct{}, 1)
	defer close(semaphoreChan)

	didAPreviousRequestFail := false
	furthestBlockProcessed := from - 1
	allLogs := []types.Log{}

	for i := 0; i < numChunks; i++ {
		// Add one to the semaphore chan. If it already has a value, the chunk blocks here until one frees up.
		// We deliberately process the chunks sequentially, since if any request results in an error, we
		// do not want to send any further requests.
		semaphoreChan <- struct{}{}

		// If a previous request failed, we stop processing newer requests
		if didAPreviousRequestFail {
			<-semaphoreChan
			continue // Noop
		}

		mu := sync.Mutex{}
		indexToLogResult := map[int]logRequestResult{}
		chunk := <-chunkChan

		wg := &sync.WaitGroup{}
		for i, aBlockRange := range chunk {
			wg.Add(1)
			go func(index int, b *blockRange) {
				defer wg.Done()

				select {
				case <-ctx.Done():
					indexToLogResult[index] = logRequestResult{
						From: b.FromBlock,
						To:   b.ToBlock,
						Err:  errors.New("context was canceled"),
						Logs: []types.Log{},
					}
					return
				default:
				}

				logs, err := w.filterLogsRecursively(b.FromBlock, b.ToBlock, []types.Log{})
				if err != nil {
					glog.Errorf("failed to fetch logs for range error=%v fromBlock=%v toBlock=%v", err, b.FromBlock, b.ToBlock)
				}
				mu.Lock()
				indexToLogResult[index] = logRequestResult{
					From: b.FromBlock,
					To:   b.ToBlock,
					Err:  err,
					Logs: logs,
				}
				mu.Unlock()
			}(i, aBlockRange)
		}

		// Wait for all log requests to complete
		wg.Wait()

		for i, aBlockRange := range chunk {
			logRequestResult := indexToLogResult[i]
			// Break at first error encountered
			if logRequestResult.Err != nil {
				didAPreviousRequestFail = true
				furthestBlockProcessed = logRequestResult.From - 1
				break
			}
			allLogs = append(allLogs, logRequestResult.Logs...)
			furthestBlockProcessed = aBlockRange.ToBlock
		}
		<-semaphoreChan
	}

	return allLogs, furthestBlockProcessed
}

type blockRange struct {
	FromBlock int
	ToBlock   int
}

// getSubBlockRanges breaks up the block range into smaller block ranges of rangeSize.
// `eth_getLogs` requests are inclusive to both the start and end blocks specified and
// so we need to make the ranges exclusive of one another to avoid fetching the same
// blocks' logs twice.
func (w *Watcher) getSubBlockRanges(from, to, rangeSize int) []*blockRange {
	chunks := []*blockRange{}
	numBlocksLeft := to - from
	if numBlocksLeft < rangeSize {
		chunks = append(chunks, &blockRange{
			FromBlock: from,
			ToBlock:   to,
		})
	} else {
		blocks := []int{}
		for i := 0; i <= numBlocksLeft; i++ {
			blocks = append(blocks, from+i)
		}
		numChunks := len(blocks) / rangeSize
		remainder := len(blocks) % rangeSize
		if remainder > 0 {
			numChunks = numChunks + 1
		}

		for i := 0; i < numChunks; i = i + 1 {
			fromIndex := i * rangeSize
			toIndex := fromIndex + rangeSize
			if toIndex > len(blocks) {
				toIndex = len(blocks)
			}
			bs := blocks[fromIndex:toIndex]
			blockRange := &blockRange{
				FromBlock: bs[0],
				ToBlock:   bs[len(bs)-1],
			}
			chunks = append(chunks, blockRange)
		}
	}
	return chunks
}

const infuraTooManyResultsErrMsg = "query returned more than 10000 results"

func (w *Watcher) filterLogsRecursively(from, to int, allLogs []types.Log) ([]types.Log, error) {
	glog.V(6).Infof("Polling blocks from=%v to=%v", from, to)
	numBlocks := to - from
	topics := [][]common.Hash{}
	if len(w.topics) > 0 {
		topics = append(topics, w.topics)
	}
	logs, err := w.client.FilterLogs(ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(to)),
		Topics:    topics,
	})
	if err != nil {
		// Infura caps the logs returned to 10,000 per request, if our request exceeds this limit, split it
		// into two requests. Parity, Geth and Alchemy all have much higher limits (if any at all), so no need
		// to expect any similar errors of this nature from them.
		if err.Error() == infuraTooManyResultsErrMsg {
			// HACK(fabio): Infura limits the returned results to 10,000 logs, BUT some single
			// blocks contain more then 10,000 logs. This has supposedly been fixed but we keep
			// this logic here just in case. It helps us avoid infinite recursion.
			// Source: https://community.infura.io/t/getlogs-error-query-returned-more-than-1000-results/358/10
			if from == to {
				return allLogs, fmt.Errorf("Unable to get the logs for block #%d, because it contains too many logs", from)
			}

			r := numBlocks % 2
			firstBatchSize := numBlocks / 2
			secondBatchSize := firstBatchSize
			if r == 1 {
				secondBatchSize = secondBatchSize + 1
			}

			endFirstHalf := from + firstBatchSize
			startSecondHalf := endFirstHalf + 1
			allLogs, err := w.filterLogsRecursively(from, endFirstHalf, allLogs)
			if err != nil {
				return nil, err
			}
			allLogs, err = w.filterLogsRecursively(startSecondHalf, to, allLogs)
			if err != nil {
				return nil, err
			}
			return allLogs, nil
		} else {
			return nil, err
		}
	}
	allLogs = append(allLogs, logs...)
	return allLogs, nil
}

// enrichWithL1 adds L1 block number to the event with the highest block number.
//
// Adding L1 block number to an event requires an RPC call and the only code using L1 block number is TimeWatcher that
// is interested only in the highest L1 block number, not in all of them.
func (w *Watcher) enrichWithL1(events []*Event) []*Event {
	if len(events) == 0 {
		return events
	}

	max := 0
	for i, e := range events {
		if events[max].BlockHeader.Number.Cmp(e.BlockHeader.Number) <= 0 {
			max = i
		}
	}

	block, err := w.enrichWithL1BlockNumber(events[max].BlockHeader)
	if err != nil {
		glog.Errorf("Cannot fetch L1 block number for block number %d err=%q", events[max].BlockHeader.Number, err)
	}

	events[max].BlockHeader = block
	return events
}

func (w *Watcher) enrichWithL1BlockNumber(header *MiniHeader) (*MiniHeader, error) {
	if header == nil || header.L1BlockNumber != nil {
		return header, nil
	}
	fetchedBlock, err := w.client.HeaderByHash(header.Hash)
	if err != nil {
		return header, err
	}
	header.L1BlockNumber = fetchedBlock.L1BlockNumber
	return header, nil
}

func isUnknownBlockErr(err error) bool {
	// Geth error
	if err.Error() == "unknown block" {
		return true
	}
	// Parity error
	if err.Error() == "One of the blocks specified in filter (fromBlock, toBlock or blockHash) cannot be found" {
		return true
	}
	return false
}
