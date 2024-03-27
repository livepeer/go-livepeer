package eth

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/livepeer/go-livepeer/eth/contracts/chainlink"
)

type PriceData struct {
	RoundID   int64
	Price     *big.Rat
	UpdatedAt time.Time
}

// PriceFeedEthClient is an interface for fetching price data from a Chainlink
// PriceFeed contract.
type PriceFeedEthClient interface {
	Description() (string, error)
	FetchPriceData() (PriceData, error)
}

func NewPriceFeedEthClient(ethClient *ethclient.Client, priceFeedAddr string) (PriceFeedEthClient, error) {
	addr := common.HexToAddress(priceFeedAddr)
	priceFeed, err := chainlink.NewAggregatorV3Interface(addr, ethClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator proxy: %w", err)
	}

	return &priceFeedClient{
		client:    ethClient,
		priceFeed: priceFeed,
	}, nil
}

type priceFeedClient struct {
	client    *ethclient.Client
	priceFeed *chainlink.AggregatorV3Interface
}

func (c *priceFeedClient) Description() (string, error) {
	return c.priceFeed.Description(&bind.CallOpts{})
}

func (c *priceFeedClient) FetchPriceData() (PriceData, error) {
	data, err := c.priceFeed.LatestRoundData(&bind.CallOpts{})
	if err != nil {
		return PriceData{}, errors.New("failed to get latest round data: " + err.Error())
	}

	decimals, err := c.priceFeed.Decimals(&bind.CallOpts{})
	if err != nil {
		return PriceData{}, errors.New("failed to get decimals: " + err.Error())
	}

	return computePriceData(data.RoundId, data.UpdatedAt, data.Answer, decimals), nil
}

// computePriceData transforms the raw data from the PriceFeed into the higher
// level PriceData struct, more easily usable by the rest of the system.
func computePriceData(roundID, updatedAt, answer *big.Int, decimals uint8) PriceData {
	// Compute a big.int which is 10^decimals.
	divisor := new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(decimals)),
		nil)

	return PriceData{
		RoundID:   roundID.Int64(),
		Price:     new(big.Rat).SetFrac(answer, divisor),
		UpdatedAt: time.Unix(updatedAt.Int64(), 0),
	}
}
