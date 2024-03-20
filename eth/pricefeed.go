package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
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

type PriceFeedEthClient interface {
	Description() (string, error)
	FetchPriceData() (PriceData, error)
}

func NewPriceFeedEthClient(ctx context.Context, rpcUrl, priceFeedAddr string) (PriceFeedEthClient, error) {
	client, err := ethclient.DialContext(ctx, rpcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	ok := isContractAddress(priceFeedAddr, client)
	if !ok {
		return nil, fmt.Errorf("not a contract address: %s", priceFeedAddr)
	}

	addr := common.HexToAddress(priceFeedAddr)
	priceFeed, err := chainlink.NewAggregatorV3Interface(addr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create mock aggregator proxy: %w", err)
	}

	return &priceFeedClient{
		client:    client,
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

type ethClient interface {
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
}

func isContractAddress(addr string, client ethClient) bool {
	if len(addr) == 0 {
		return false
	}

	// Ensure it is an Ethereum address: 0x followed by 40 hexadecimal characters.
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	if !re.MatchString(addr) {
		return false
	}

	// Ensure it is a contract address.
	address := common.HexToAddress(addr)
	bytecode, err := client.CodeAt(context.Background(), address, nil) // nil is latest block
	if err != nil {
		return false
	}
	isContract := len(bytecode) > 0
	return isContract
}
