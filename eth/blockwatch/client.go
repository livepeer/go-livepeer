package blockwatch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// Client defines the methods needed to satisfy the client expected when
// instantiating a Watcher instance.
type Client interface {
	HeaderByNumber(number *big.Int) (*MiniHeader, error)
	HeaderByHash(hash common.Hash) (*MiniHeader, error)
	FilterLogs(q ethereum.FilterQuery) ([]types.Log, error)
}

// RPCClient is a Client for fetching Ethereum blocks from a specific JSON-RPC endpoint.
type RPCClient struct {
	rpcClient      *rpc.Client
	client         *ethclient.Client
	requestTimeout time.Duration
}

// NewRPCClient returns a new Client for fetching Ethereum blocks using the given
// ethclient.Client.
func NewRPCClient(rpcURL string, requestTimeout time.Duration) (*RPCClient, error) {
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	rpcClient, err := rpc.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	return &RPCClient{rpcClient: rpcClient, client: ethClient, requestTimeout: requestTimeout}, nil
}

type getHeaderResponse struct {
	Hash          common.Hash `json:"hash"`
	ParentHash    common.Hash `json:"parentHash"`
	Number        string      `json:"number"`
	L1BlockNumber string      `json:"l1BlockNumber"`
}

// HeaderByNumber fetches a block header by its number. If no `number` is supplied, it will return the latest
// block header. If no block exists with this number it will return a `ethereum.NotFound` error.
func (rc *RPCClient) HeaderByNumber(number *big.Int) (*MiniHeader, error) {
	var blockParam string
	if number == nil {
		blockParam = "latest"
	} else {
		blockParam = hexutil.EncodeBig(number)
	}

	return rc.callEth("eth_getBlockByNumber", blockParam)
}

// HeaderByHash fetches a block header by its block hash. If no block exists with this number it will return
// a `ethereum.NotFound` error.
func (rc *RPCClient) HeaderByHash(hash common.Hash) (*MiniHeader, error) {
	return rc.callEth("eth_getBlockByHash", hash)
}

func (rc *RPCClient) callEth(method string, arg interface{}) (*MiniHeader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rc.requestTimeout)
	defer cancel()

	var header getHeaderResponse
	err := rc.rpcClient.CallContext(ctx, &header, method, arg, false)

	if err != nil {
		return nil, err
	}
	// If it returned an empty struct
	if header.Number == "" {
		return nil, ethereum.NotFound
	}

	// If no L1BlockNumber, then Livepeer is running on L1, so L1BlockNumber is the same as BlockNumber
	if header.L1BlockNumber == "" {
		header.L1BlockNumber = header.Number
	}

	blockNum, ok := math.ParseBig256(header.Number)
	if !ok {
		return nil, errors.New("Failed to parse big.Int value from hex-encoded block number returned from eth_getBlockByNumber")
	}
	l1BlockNum, ok := math.ParseBig256(header.L1BlockNumber)
	if !ok {
		return nil, fmt.Errorf("Failed to parse big.Int value from hex-encoded L1 block number returned from %v", method)
	}
	miniHeader := &MiniHeader{
		Hash:          header.Hash,
		Parent:        header.ParentHash,
		Number:        blockNum,
		L1BlockNumber: l1BlockNum,
	}
	return miniHeader, nil
}

// FilterLogs returns the logs that satisfy the supplied filter query.
func (rc *RPCClient) FilterLogs(q ethereum.FilterQuery) ([]types.Log, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rc.requestTimeout)
	defer cancel()
	logs, err := rc.client.FilterLogs(ctx, q)
	if err != nil {
		return nil, err
	}
	return logs, nil
}
