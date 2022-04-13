package watchers

import (
	"fmt"
	"github.com/livepeer/go-livepeer/eth/contracts"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAndGet_LastInitializedRound_LastInitializedBlockHash(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	round := big.NewInt(5)
	var hash [32]byte
	copy(hash[:], "hello world")
	var preHash [32]byte
	copy(preHash[:], "hey hey hey")
	num := big.NewInt(10)
	tw.setLastInitializedRound(round, hash, preHash, num)
	assert.Equal(tw.lastInitializedRound, round)
	assert.Equal(tw.lastInitializedL1BlockHash, hash)
	assert.Equal(tw.preLastInitializedL1BlockHash, preHash)

	r := tw.LastInitializedRound()
	assert.Equal(r, round)
	h := tw.LastInitializedL1BlockHash()
	assert.Equal(h, hash)
	ph := tw.PreLastInitializedL1BlockHash()
	assert.Equal(ph, preHash)
	assert.Equal(tw.CurrentRoundStartL1Block(), num)
}

func TestSetAndGet_TranscoderPoolSize(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	size := big.NewInt(50)
	tw.setTranscoderPoolSize(size)
	assert.Equal(size, tw.transcoderPoolSize)
	assert.Equal(size, tw.GetTranscoderPoolSize())

	// return big.Int(0) when nil
	tw.setTranscoderPoolSize(nil)
	assert.Nil(tw.transcoderPoolSize)
	assert.Equal(big.NewInt(0), tw.GetTranscoderPoolSize())
}

func TestTimeWatcher_NewTimeWatcher(t *testing.T) {
	assert := assert.New(t)
	size := big.NewInt(50)
	block := big.NewInt(10)
	round := big.NewInt(1)
	hash := ethcommon.HexToHash("foo")
	lpEth := &eth.StubClient{
		PoolSize:          size,
		BlockNum:          block,
		BlockHashToReturn: hash,
		Round:             round,
		Errors:            make(map[string]error),
	}
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{L1BlockNumber: block},
	}

	testErr := fmt.Errorf("error")

	// Last InitializedRound error
	lpEth.Errors["LastInitializedRound"] = testErr
	expErr := fmt.Sprintf("error fetching initial lastInitializedRound value err=%q", testErr)
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(tw)
	assert.EqualError(err, expErr)
	lpEth.Errors["LastInitializedRound"] = nil

	// BlockHashForRound error
	lpEth.Errors["BlockHashForRound"] = testErr
	expErr = fmt.Sprintf("error fetching initial lastInitializedL1BlockHash value err=%q", testErr)
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(tw)
	assert.EqualError(err, expErr)
	lpEth.Errors["BlockHashForRound"] = nil

	// CurrentRoundStartL1Block error
	lpEth.Errors["CurrentRoundStartBlock"] = testErr
	expErr = fmt.Sprintf("error fetching current round start block err=%q", testErr)
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(tw)
	assert.EqualError(err, expErr)
	lpEth.Errors["CurrentRoundStartBlock"] = nil

	// GetLastestBlock error
	watcher.err = fmt.Errorf("GetLatestBlock error")
	expErr = fmt.Sprintf("error fetching last seen block err=%q", watcher.err)
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(tw)
	assert.EqualError(err, expErr)
	watcher.err = nil

	// TranscoderPoolSize error
	lpEth.Errors["GetTranscoderPoolSize"] = testErr
	expErr = fmt.Sprintf("error fetching initial transcoderPoolSize err=%q", testErr)
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(tw)
	assert.EqualError(err, expErr)
	lpEth.Errors["GetTranscoderPoolSize"] = nil

	preHash := ethcommon.HexToHash("bar")
	lpEth.BlockHashForRoundMap = map[int64]common.Hash{0: preHash, 1: hash}
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(err)
	bh := tw.LastInitializedL1BlockHash()
	assert.Equal(hash, common.BytesToHash(bh[:]))
	pbh := tw.PreLastInitializedL1BlockHash()
	assert.Equal(preHash, common.BytesToHash(pbh[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(block, tw.LastSeenL1Block())

	// if watcher.GetLatestBlock() == nil, initialise lastSeenL1Block to big.NewInt(0)
	watcher.latestHeader = nil
	tw, err = NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	assert.Nil(err)
	bh = tw.LastInitializedL1BlockHash()
	assert.Equal(hash, common.BytesToHash(bh[:]))
	pbh = tw.PreLastInitializedL1BlockHash()
	assert.Equal(preHash, common.BytesToHash(pbh[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(big.NewInt(0), tw.LastSeenL1Block())
}

func TestTimeWatcher_WatchAndStop(t *testing.T) {
	assert := assert.New(t)
	size := big.NewInt(50)
	block := big.NewInt(10)
	round := big.NewInt(1)
	hash := ethcommon.HexToHash("foo")
	lpEth := &eth.StubClient{
		PoolSize:          size,
		BlockNum:          block,
		BlockHashToReturn: hash,
		Round:             round,
		Errors:            make(map[string]error),
	}
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{L1BlockNumber: block},
	}
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	require.Nil(t, err)

	header := defaultMiniHeader()
	newRoundEvent := newStubNewRoundLog()

	header.Logs = append(header.Logs, newRoundEvent)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	// New Round event
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound := tw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound := tw.LastInitializedL1BlockHash()
	var expectedHashForRound [32]byte
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(header.L1BlockNumber, tw.LastSeenL1Block())

	// Test no NewRound events, values on rw remain the same
	tw.setTranscoderPoolSize(big.NewInt(10))
	blockEvent.BlockHeader.Logs = header.Logs[:1]
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	lastRound = tw.LastInitializedRound()
	assert.Zero(lastRound.Cmp(big.NewInt(8)))
	bhForRound = tw.LastInitializedL1BlockHash()
	copy(expectedHashForRound[:], newRoundEvent.Data[:])
	assert.Equal(bhForRound, expectedHashForRound)
	assert.Equal(big.NewInt(10), tw.GetTranscoderPoolSize())
	assert.Equal(header.L1BlockNumber, tw.LastSeenL1Block())

	// Test RPC paths (event removed)
	blockEvent.BlockHeader.Logs = append(blockEvent.BlockHeader.Logs, newRoundEvent)
	blockEvent.Type = blockwatch.Removed
	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	bhForRound = tw.LastInitializedL1BlockHash()
	assert.Equal(hash, common.BytesToHash(bhForRound[:]))
	assert.Equal(round, tw.LastInitializedRound())
	assert.Equal(size, tw.GetTranscoderPoolSize())
	assert.Equal(header.L1BlockNumber, tw.LastSeenL1Block())

	// Test Stop
	tw.Stop()
	time.Sleep(2 * time.Millisecond)
	assert.True(watcher.sub.unsubscribed)
}

func TestTimeWatcher_HandleLog(t *testing.T) {
	lpEth := &eth.StubClient{Round: big.NewInt(2)}
	watcher := &stubBlockWatcher{}
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	require.Nil(t, err)

	assert := assert.New(t)

	// Test unknown event
	log := newStubBaseLog()
	log.Topics = []ethcommon.Hash{ethcommon.BytesToHash([]byte("foo"))}

	err = tw.handleLog(log)
	assert.Nil(err)
	assert.Equal(big.NewInt(2), tw.LastInitializedRound())
	assert.Equal([32]byte{}, tw.LastInitializedL1BlockHash())
}

func TestTimeWatcher_HandleDecodedLog(t *testing.T) {
	assert := assert.New(t)

	lpEth := &eth.StubClient{
		Round: big.NewInt(2),
		BlockHashForRoundMap: map[int64]common.Hash{
			1: ethcommon.BytesToHash([]byte("one")),
			2: ethcommon.BytesToHash([]byte("two")),
			3: ethcommon.BytesToHash([]byte("three")),
			4: ethcommon.BytesToHash([]byte("four")),
			5: ethcommon.BytesToHash([]byte("five")),
			6: ethcommon.BytesToHash([]byte("six")),
		},
	}
	watcher := &stubBlockWatcher{}
	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, lpEth)
	require.Nil(t, err)

	var hash [32]byte
	var preHash [32]byte
	var nr contracts.RoundsManagerNewRound

	// Test remove log, data refreshed from eth client
	log := newStubBaseLog()
	log.Removed = true
	lpEth.Round = big.NewInt(3)

	err = tw.handleDecodedLog(log, nr)

	assert.Nil(err)
	assert.Equal(big.NewInt(3), tw.LastInitializedRound())
	copy(hash[:], lpEth.BlockHashForRoundMap[3].Bytes())
	assert.Equal(hash, tw.LastInitializedL1BlockHash())
	copy(preHash[:], lpEth.BlockHashForRoundMap[2].Bytes())
	assert.Equal(preHash, tw.PreLastInitializedL1BlockHash())

	// Test add log with subsequent round
	log.Removed = false
	currentHash := ethcommon.BytesToHash([]byte("last"))
	tw.lastInitializedL1BlockHash = currentHash
	nr.Round = big.NewInt(4)
	nr.BlockHash = lpEth.BlockHashForRoundMap[4]

	err = tw.handleDecodedLog(log, nr)

	assert.Nil(err)
	assert.Equal(big.NewInt(4), tw.LastInitializedRound())
	copy(hash[:], lpEth.BlockHashForRoundMap[4].Bytes())
	assert.Equal(hash, tw.LastInitializedL1BlockHash())
	copy(preHash[:], currentHash.Bytes())
	assert.Equal(preHash, tw.PreLastInitializedL1BlockHash())

	// Test add log with not subsequent round
	nr.Round = big.NewInt(6)
	nr.BlockHash = lpEth.BlockHashForRoundMap[6]

	err = tw.handleDecodedLog(log, nr)

	assert.Nil(err)
	assert.Equal(big.NewInt(6), tw.LastInitializedRound())
	copy(hash[:], lpEth.BlockHashForRoundMap[6].Bytes())
	assert.Equal(hash, tw.LastInitializedL1BlockHash())
	copy(preHash[:], lpEth.BlockHashForRoundMap[5].Bytes())
	assert.Equal(preHash, tw.PreLastInitializedL1BlockHash())
}

func TestLastSeenBlock(t *testing.T) {
	assert := assert.New(t)
	tw := &TimeWatcher{}
	block := big.NewInt(5)

	tw.setLastSeenL1Block(block)
	tw.LastSeenL1Block()
	assert.Equal(big.NewInt(5), tw.LastSeenL1Block())
}

func TestHandleBlockNum(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{L1BlockNumber: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{Round: big.NewInt(2)})
	assert.Nil(err)
	header := defaultMiniHeader()
	header.L1BlockNumber = big.NewInt(10)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	assert.Equal(tw.LastSeenL1Block(), header.L1BlockNumber)
}

func TestSubscribeBlocks(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{L1BlockNumber: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{Round: big.NewInt(2)})
	assert.Nil(err)
	header := defaultMiniHeader()
	header.L1BlockNumber = big.NewInt(10)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	events := make(chan *big.Int, 10)
	sub := tw.SubscribeL1Blocks(events)
	defer sub.Unsubscribe()

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	update := <-events
	assert.Equal(update, header.L1BlockNumber)
}

func TestSubscribeRounds(t *testing.T) {
	assert := assert.New(t)
	watcher := &stubBlockWatcher{
		latestHeader: &blockwatch.MiniHeader{L1BlockNumber: big.NewInt(1)},
	}

	tw, err := NewTimeWatcher(stubRoundsManagerAddr, watcher, &eth.StubClient{Round: big.NewInt(2)})
	assert.Nil(err)
	header := defaultMiniHeader()
	newRoundEvent := newStubNewRoundLog()

	header.Logs = append(header.Logs, newRoundEvent)
	blockEvent := &blockwatch.Event{
		Type:        blockwatch.Added,
		BlockHeader: header,
	}

	events := make(chan types.Log, 10)
	sub := tw.SubscribeRounds(events)
	defer sub.Unsubscribe()

	go tw.Watch()
	time.Sleep(2 * time.Millisecond)

	watcher.sink <- []*blockwatch.Event{blockEvent}
	time.Sleep(2 * time.Millisecond)
	update := <-events
	assert.Equal(newRoundEvent, update)
}
