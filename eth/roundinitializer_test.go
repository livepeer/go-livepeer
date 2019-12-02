package eth

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type stubBlockNumReader struct {
	blkNum *big.Int
	err    error
}

func (rdr *stubBlockNumReader) LastSeenBlock() (*big.Int, error) {
	if rdr.err != nil {
		return nil, rdr.err
	}

	return rdr.blkNum, nil
}

type stubBlockHashReader struct {
	blkHash [32]byte
}

func (rdr *stubBlockHashReader) LastInitializedBlockHash() [32]byte {
	return rdr.blkHash
}

func TestRoundInitializer_CurrentEpochSeed(t *testing.T) {
	client := &MockClient{}
	blkNumRdr := &stubBlockNumReader{}
	blkHashRdr := &stubBlockHashReader{}
	initializer := NewRoundInitializer(client, blkNumRdr, blkHashRdr, 1*time.Second)

	assert := assert.New(t)

	// Test error getting block num
	blkNumRdr.err = errors.New("LastSeenBlock error")

	_, err := initializer.currentEpochSeed(nil, [32]byte{})
	assert.EqualError(err, blkNumRdr.err.Error())

	// Test epochNum = 0
	blkNumRdr.err = nil
	blkNumRdr.blkNum = big.NewInt(5)
	blkHash := [32]byte{123}

	epochSeed, err := initializer.currentEpochSeed(blkNumRdr.blkNum, blkHash)
	assert.Nil(err)
	// epochNum = (5 - 5) / 5 = 0
	// epochSeed = keccak256(blkHash | 0) = 53205358842179480591542570540016728811976439286094436690881169143335261643310
	expEpochSeed, _ := new(big.Int).SetString("53205358842179480591542570540016728811976439286094436690881169143335261643310", 10)
	assert.Equal(expEpochSeed, epochSeed)

	// Test epochNum > 0
	blkNumRdr.blkNum = big.NewInt(20)

	epochSeed, err = initializer.currentEpochSeed(big.NewInt(5), blkHash)
	assert.Nil(err)
	// epochNum = (20 - 5) / 5 = 3
	// epochSeed = keccak256(blkHash | 3) = 42541119854153860846042329644941941146216657514071318786342840580076059276721
	expEpochSeed.SetString("42541119854153860846042329644941941146216657514071318786342840580076059276721", 10)
	assert.Equal(expEpochSeed, epochSeed)

	// Test epochNum > 0 with some # of blocks into the epoch
	epochSeed, err = initializer.currentEpochSeed(big.NewInt(4), blkHash)
	assert.Nil(err)
	// epochNum = (20 - 4) / 5 = 3.2 -> 3
	assert.Equal(expEpochSeed, epochSeed)
}

func TestRoundInitializer_ShouldInitialize(t *testing.T) {
	client := &MockClient{}
	blkNumRdr := &stubBlockNumReader{}
	blkHashRdr := &stubBlockHashReader{}
	initializer := NewRoundInitializer(client, blkNumRdr, blkHashRdr, 1*time.Second)

	assert := assert.New(t)

	// Test error getting transcoders
	expErr := errors.New("TranscoderPool error")
	client.On("TranscoderPool").Return(nil, expErr).Once()

	ok, err := initializer.shouldInitialize(nil)
	assert.EqualError(err, expErr.Error())
	assert.False(ok)

	// Test error getting max active set size
	expErr = errors.New("GetTranscoderPoolMaxSize error")
	client.On("TranscoderPool").Return(nil, nil).Once()
	client.On("GetTranscoderPoolMaxSize").Return(nil, expErr).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.EqualError(err, expErr.Error())
	assert.False(ok)

	// Test active set is empty because no registered transcoders
	client.On("TranscoderPool").Return([]*lpTypes.Transcoder{}, nil).Once()
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(2), nil).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test active set is empty because max active set size is 0
	registered := []*lpTypes.Transcoder{
		&lpTypes.Transcoder{},
	}
	client.On("TranscoderPool").Return(registered, nil).Once()
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(0), nil).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test that caller is not in active set because it is not registered
	caller := ethcommon.BytesToAddress([]byte("foo"))
	client.On("Account").Return(accounts.Account{Address: caller})

	registered = []*lpTypes.Transcoder{
		&lpTypes.Transcoder{Address: ethcommon.BytesToAddress([]byte("jar"))},
		&lpTypes.Transcoder{Address: ethcommon.BytesToAddress([]byte("bar"))},
	}
	client.On("TranscoderPool").Return(registered, nil).Once()
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(2), nil).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test that caller is not in active set but it is registered
	registered = append(registered, &lpTypes.Transcoder{Address: caller})
	client.On("TranscoderPool").Return(registered, nil)
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(2), nil).Once()

	ok, err = initializer.shouldInitialize(nil)
	assert.Nil(err)
	assert.False(ok)

	// Test caller not selected
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(3), nil)

	seed := big.NewInt(3)
	ok, err = initializer.shouldInitialize(seed)
	assert.Nil(err)
	assert.False(ok)

	// Test caller selected
	seed = big.NewInt(5)
	ok, err = initializer.shouldInitialize(seed)
	assert.Nil(err)
	assert.True(ok)
}

func TestRoundInitializer_TryInitialize(t *testing.T) {
	client := &MockClient{}
	blkNumRdr := &stubBlockNumReader{}
	blkHashRdr := &stubBlockHashReader{}
	initializer := NewRoundInitializer(client, blkNumRdr, blkHashRdr, 1*time.Second)

	assert := assert.New(t)

	// Test error checking round initialized
	expErr := errors.New("CurrentRoundInitialized error")
	client.On("CurrentRoundInitialized").Return(false, expErr).Once()

	err := initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test current round is initialized
	client.On("CurrentRoundInitialized").Return(true, nil).Once()

	err = initializer.tryInitialize()
	assert.Nil(err)

	// Test error getting current round start block
	expErr = errors.New("CurrentRoundStartBlock error")
	client.On("CurrentRoundStartBlock").Return(nil, expErr).Once()
	client.On("CurrentRoundInitialized").Return(false, nil)

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test error getting epoch seed
	expErr = errors.New("currentEpochSeed error")
	blkNumRdr.err = expErr
	startBlkNum := big.NewInt(5)
	client.On("CurrentRoundStartBlock").Return(startBlkNum, nil)

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test error checking should initialize
	expErr = errors.New("shouldInitialize error")
	client.On("TranscoderPool").Return(nil, expErr).Once()
	blkNumRdr.err = nil
	blkNumRdr.blkNum = big.NewInt(5)

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test should not initialize
	caller := ethcommon.BytesToAddress([]byte("foo"))
	client.On("Account").Return(accounts.Account{Address: caller})

	registered := []*lpTypes.Transcoder{
		&lpTypes.Transcoder{Address: ethcommon.BytesToAddress([]byte("jar"))},
	}
	client.On("TranscoderPool").Return(registered, nil).Once()
	client.On("GetTranscoderPoolMaxSize").Return(big.NewInt(2), nil)

	err = initializer.tryInitialize()
	assert.Nil(err)

	blkHashRdr.blkHash = [32]byte{123}

	// Test error getting current round
	expErr = errors.New("CurrentRound error")
	client.On("CurrentRound").Return(nil, expErr).Once()
	registered = append([]*lpTypes.Transcoder{&lpTypes.Transcoder{Address: caller}}, registered...)
	client.On("TranscoderPool").Return(registered, nil)

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test error when submitting initialization tx
	client.On("CurrentRound").Return(big.NewInt(5), nil)
	expErr = errors.New("InitializeRound error")
	client.On("InitializeRound").Return(nil, expErr).Once()

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test error checking initialization tx
	tx := &types.Transaction{}
	client.On("InitializeRound").Return(tx, nil)
	expErr = errors.New("CheckTx error")
	client.On("CheckTx", mock.Anything).Return(expErr).Once()

	err = initializer.tryInitialize()
	assert.EqualError(err, expErr.Error())

	// Test success
	client.On("CheckTx", mock.Anything).Return(nil)

	err = initializer.tryInitialize()
	assert.Nil(err)
}
