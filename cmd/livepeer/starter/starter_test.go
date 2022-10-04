package starter

import (
	"errors"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupOrchestrator(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dbh, dbraw, err := common.TempDB(t)
	require.Nil(err)

	defer dbh.Close()
	defer dbraw.Close()

	orch := pm.RandAddress()

	stubEthClient := &eth.StubClient{
		Orch: &lpTypes.Transcoder{
			Address:           orch,
			ActivationRound:   big.NewInt(5),
			DeactivationRound: big.NewInt(10),
		},
		TranscoderAddress: orch,
	}

	n, err := core.NewLivepeerNode(stubEthClient, "", dbh)
	require.Nil(err)

	err = setupOrchestrator(n, orch)
	assert.Nil(err)

	orchs, err := dbh.SelectOrchs(&common.DBOrchFilter{
		Addresses: []ethcommon.Address{orch},
	})
	assert.Nil(err)
	assert.Len(orchs, 1)
	assert.Equal(orchs[0].ActivationRound, int64(5))
	assert.Equal(orchs[0].DeactivationRound, int64(10))

	// test eth.GetTranscoder error
	stubEthClient.Err = errors.New("GetTranscoder error")
	err = setupOrchestrator(n, orch)
	assert.EqualError(err, "GetTranscoder error")
}

func TestIsLocalURL(t *testing.T) {
	assert := assert.New(t)

	// Test url.ParseRequestURI error
	_, err := isLocalURL("127.0.0.1:8935")
	assert.NotNil(err)

	// Test loopback URLs
	isLocal, err := isLocalURL("https://127.0.0.1:8935")
	assert.Nil(err)
	assert.True(isLocal)
	isLocal, err = isLocalURL("https://127.0.0.2:8935")
	assert.Nil(err)
	assert.True(isLocal)

	// Test localhost URL
	isLocal, err = isLocalURL("https://localhost:8935")
	assert.Nil(err)
	assert.True(isLocal)

	// Test non-local URL
	isLocal, err = isLocalURL("https://0.0.0.0:8935")
	assert.Nil(err)
	assert.False(isLocal)
	isLocal, err = isLocalURL("https://7.7.7.7:8935")
	assert.Nil(err)
	assert.False(isLocal)
}

func TestParseGetBroadcasterPrices(t *testing.T) {
	assert := assert.New(t)

	j := `{"broadcasters":[{"ethaddress":"0x0000000000000000000000000000000000000000","priceperunit":1000,"pixelsperunit":1}, {"ethaddress":"0x1000000000000000000000000000000000000000","priceperunit":2000,"pixelsperunit":3}]}`
	
	prices := getBroadcasterPrices(j)
	assert.NotNil(prices)
	assert.Equal(2,len(prices))

	price1 := big.NewRat(prices[0].PricePerUnit, prices[0].PixelsPerUnit)
	price2 := big.NewRat(prices[1].PricePerUnit, prices[1].PixelsPerUnit)
	assert.Equal(big.NewRat(1000,1), price1)
	assert.Equal(big.NewRat(2000,3), price2)
}
