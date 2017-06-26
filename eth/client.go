package eth

// This is meant to be the integration point with the Ethereum smart contract.  It's currently stubbed for now.
//
// We can do the implementation following this link:
// https://github.com/ethereum/go-ethereum/wiki/Native-DApps:-Go-bindings-to-Ethereum-contracts

//go:generate abigen --abi protocol/abi/LivepeerProtocol.abi --pkg contracts --type LivepeerProtocol --out contracts/livepeerProtocol.go --bin protocol/bin/LivepeerProtocol.bin
//go:generate abigen --abi protocol/abi/LivepeerToken.abi --pkg contracts --type LivepeerToken --out contracts/livepeerToken.go --bin protocol/bin/LivepeerToken.bin
//go:generate abigen --abi protocol/abi/TranscoderPools.abi --pkg contracts --type TranscoderPools --out contracts/transcoderPools.go --bin protocol/bin/TranscoderPools.bin
//go:generate abigen --abi protocol/abi/TranscodeJobs.abi --pkg contracts --type TranscodeJobs --out contracts/transcodeJobs.go --bin protocol/bin/TranscodeJobs.bin
//go:generate abigen --abi protocol/abi/MaxHeap.abi --pkg contracts --type MaxHeap --out contracts/maxHeap.go --bin protocol/bin/MaxHeap.bin
//go:generate abigen --abi protocol/abi/MinHeap.abi --pkg contracts --type MinHeap --out contracts/minHeap.go --bin protocol/bin/MinHeap.bin
//go:generate abigen --abi protocol/abi/Node.abi --pkg contracts --type Node --out contracts/node.go --bin protocol/bin/Node.bin
//go:generate abigen --abi protocol/abi/SafeMath.abi --pkg contracts --type SafeMath --out contracts/safeMath.go --bin protocol/bin/SafeMath.bin
//go:generate abigen --abi protocol/abi/ECVerify.abi --pkg contracts --type ECVerify --out contracts/ecVerify.go --bin protocol/bin/ECVerify.bin
//go:generate abigen --abi protocol/abi/MerkleProof.abi --pkg contracts --type MerkleProof --out contracts/merkleProof.go --bin protocol/bin/MerkleProof.bin

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/libp2p-livepeer/eth/contracts"
)

type Client struct {
	protocolSession *contracts.LivepeerProtocolSession
	tokenSession    *contracts.LivepeerTokenSession
	backend         bind.ContractBackend

	protocolContractAddr common.Address
	tokenContractAddr    common.Address
}

func NewClient(transactOpts *bind.TransactOpts, backend bind.ContractBackend, protocolContractAddr common.Address) (*Client, error) {
	protocol, err := contracts.NewLivepeerProtocol(protocolContractAddr, backend)

	if err != nil {
		return nil, err
	}

	tokenContractAddr, err := protocol.Token(nil)

	if err != nil {
		return nil, err
	}

	token, err := contracts.NewLivepeerToken(tokenContractAddr, backend)

	if err != nil {
		return nil, err
	}

	return &Client{
		&contracts.LivepeerProtocolSession{
			Contract:     protocol,
			TransactOpts: *transactOpts,
		},
		&contracts.LivepeerTokenSession{
			Contract:     token,
			TransactOpts: *transactOpts,
		},
		backend,
		protocolContractAddr,
		tokenContractAddr,
	}, nil
}

func (c *Client) Transcoder(blockRewardCut uint8, feeShare uint8, pricePerSegment *big.Int) (*types.Transaction, error) {
	return c.protocolSession.Transcoder(blockRewardCut, feeShare, pricePerSegment)
}

func (c *Client) IsActiveTranscoder(addr common.Address) (bool, error) {
	return c.protocolSession.IsActiveTranscoder(addr)
}

func (c *Client) Approve(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return c.tokenSession.Approve(c.protocolContractAddr, amount)
}

func (c *Client) Bond(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	// _, err := c.tokenSession.Approve(c.protocolContractAddr, amount)

	// if err != nil {
	// 	return nil, err
	// }

	return c.protocolSession.Bond(amount, toAddr)
}

func (c *Client) InitializeRound() (*types.Transaction, error) {
	return c.protocolSession.InitializeRound()
}

func (c *Client) TransferLPT(toAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	return c.tokenSession.Transfer(toAddr, amount)
}

func (c *Client) BalanceOfLPT(addr common.Address) (*big.Int, error) {
	return c.tokenSession.BalanceOf(addr)
}
