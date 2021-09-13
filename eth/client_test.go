package eth

import (
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
)

func copyTranscoders(transcoders []*lpTypes.Transcoder) []*lpTypes.Transcoder {
	cp := make([]*lpTypes.Transcoder, 0)
	for _, tr := range transcoders {
		trCp := new(lpTypes.Transcoder)
		trCp.Address = tr.Address
		trCp.DelegatedStake = new(big.Int)
		*trCp.DelegatedStake = *tr.DelegatedStake
		cp = append(cp, trCp)
	}
	return cp
}

func TestSimulateTranscoderPool(t *testing.T) {
	assert := assert.New(t)

	// use copyTranscoders() to avoid mutating the orignal slice
	transcoders := []*lpTypes.Transcoder{
		{
			Address:        ethcommon.HexToAddress("aaa"),
			DelegatedStake: big.NewInt(5),
		},
		{
			Address:        ethcommon.HexToAddress("bbb"),
			DelegatedStake: big.NewInt(4),
		},
		{
			Address:        ethcommon.HexToAddress("ccc"),
			DelegatedStake: big.NewInt(3),
		},
		{
			Address:        ethcommon.HexToAddress("ddd"),
			DelegatedStake: big.NewInt(2),
		},
		{
			Address:        ethcommon.HexToAddress("eee"),
			DelegatedStake: big.NewInt(1),
		},
	}

	// transcoder is not in pool and doesn't have enough stake to join the pool
	hints := simulateTranscoderPoolUpdate(ethcommon.HexToAddress("fff"), big.NewInt(1), copyTranscoders(transcoders), true)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{})

	// transcoder is not in pool and pool is full, transcoder joins the list, current tail is evicted
	pool := copyTranscoders(transcoders)
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("fff"), big.NewInt(2), pool, true)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("ddd"),
	})
	assert.NotEqual(pool[len(pool)-1].Address, ethcommon.HexToAddress("eee"))

	// transcoder is not in pool and pool is not full
	// transcoder takes last spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("fff"), big.NewInt(1), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("eee"),
	})

	// transcoder is not in pool and pool is not full
	// transcoder takes first spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("fff"), big.NewInt(100), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosNext: ethcommon.HexToAddress("aaa"),
	})

	// transcoder is not in pool and pool is not full
	// transcoder takes second spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("fff"), big.NewInt(5), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("aaa"),
		PosNext: ethcommon.HexToAddress("bbb"),
	})

	// last transcoder's stake increases but remains in the last spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("eee"), big.NewInt(2), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("ddd"),
	})

	// transcoder is in pool and moves up one from the last spot
	pool = copyTranscoders(transcoders)
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("eee"), big.NewInt(3), pool, false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("ccc"),
		PosNext: ethcommon.HexToAddress("ddd"),
	})
	assert.Equal(ethcommon.HexToAddress("ddd"), pool[len(pool)-1].Address)

	// transcoder is in pool and takes the first spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("ccc"), big.NewInt(6), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosNext: ethcommon.HexToAddress("aaa"),
	})

	// transcoder decreases stake to be equal of that of the last transcoder, transcoder takes the last spot instead
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("aaa"), big.NewInt(1), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("eee"),
	})

	// transcoder decreases stake to be lower than that of the last transcoder, transcoder takes the last spot
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("aaa"), big.NewInt(0), copyTranscoders(transcoders), false)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{
		PosPrev: ethcommon.HexToAddress("eee"),
	})

	// when new stake is 0 and transcoder pool is full return empty hints
	hints = simulateTranscoderPoolUpdate(ethcommon.HexToAddress("aaa"), big.NewInt(0), copyTranscoders(transcoders), true)
	assert.Equal(hints, lpTypes.TranscoderPoolHints{})
}

func TestFindTranscoderHints(t *testing.T) {
	assert := assert.New(t)

	transcoders := []*lpTypes.Transcoder{
		{
			Address:        ethcommon.HexToAddress("aaa"),
			DelegatedStake: big.NewInt(5),
		},
		{
			Address:        ethcommon.HexToAddress("bbb"),
			DelegatedStake: big.NewInt(4),
		},
		{
			Address:        ethcommon.HexToAddress("ccc"),
			DelegatedStake: big.NewInt(3),
		},
		{
			Address:        ethcommon.HexToAddress("ddd"),
			DelegatedStake: big.NewInt(2),
		},
		{
			Address:        ethcommon.HexToAddress("eee"),
			DelegatedStake: big.NewInt(1),
		},
	}

	// del == 'aaa' == head
	hints := findTranscoderHints(ethcommon.HexToAddress("aaa"), transcoders)
	assert.Equal(hints.PosPrev, ethcommon.Address{})
	assert.Equal(hints.PosNext, ethcommon.HexToAddress("bbb"))

	// del == 'eee' == tail
	hints = findTranscoderHints(ethcommon.HexToAddress("eee"), transcoders)
	assert.Equal(hints.PosPrev, ethcommon.HexToAddress("ddd"))
	assert.Equal(hints.PosNext, ethcommon.Address{})

	// del == 'ccc'
	hints = findTranscoderHints(ethcommon.HexToAddress("ccc"), transcoders)
	assert.Equal(hints.PosPrev, ethcommon.HexToAddress("bbb"))
	assert.Equal(hints.PosNext, ethcommon.HexToAddress("ddd"))
}
