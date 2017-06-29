package eth

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	keyStore        = keystore.NewKeyStore("/Users/yondonfu/.lpTest/keystore", keystore.StandardScryptN, keystore.StandardScryptP)
	defaultPassword = ""
	rpcTimeout      = 10 * time.Second
)

func NewTransactorForAccount(account accounts.Account) (*bind.TransactOpts, error) {
	keyjson, err := keyStore.Export(account, defaultPassword, defaultPassword)

	if err != nil {
		return nil, err
	}

	transactOpts, err := bind.NewTransactor(bytes.NewReader(keyjson), defaultPassword)

	if err != nil {
		return nil, err
	}

	return transactOpts, err
}

func TestReward(t *testing.T) {
	var (
		tx           *types.Transaction
		err          error
		rpcTimeout   = 10 * time.Second
		eventTimeout = 30 * time.Second
	)

	backend, err := ethclient.Dial("/Users/yondonfu/.lpTest/geth.ipc")

	if err != nil {
		t.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	accounts := keyStore.Accounts()

	// SETUP ACCOUNTS

	transactOpts0, err := NewTransactorForAccount(accounts[0])

	if err != nil {
		t.Fatalf("Failed to create transactor 0: %v", err)
	}

	transactOpts1, err := NewTransactorForAccount(accounts[1])

	if err != nil {
		t.Fatalf("Failed to create transactor 1: %v", err)
	}

	transactOpts2, err := NewTransactorForAccount(accounts[2])

	if err != nil {
		t.Fatalf("Failed to create transactor 2: %v", err)
	}

	transactOpts3, err := NewTransactorForAccount(accounts[3])

	if err != nil {
		t.Fatalf("Failed to create transactor 3: %v", err)
	}

	// DEPLOY NODE

	nodeAddr, tx, err := DeployLibrary(transactOpts0, backend, "Node", nil)

	if err != nil {
		t.Fatalf("Failed to deploy Node: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined Node tx: %v", err)
	}

	fmt.Printf("Node deployed at: %v\n", nodeAddr.Hex())

	// DEPLOY MAXHEAP

	maxHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	maxHeapAddr, tx, err := DeployLibrary(transactOpts0, backend, "MaxHeap", maxHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MaxHeap: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MaxHeap tx: %v", err)
	}

	fmt.Printf("MaxHeap deployed at: %v\n", maxHeapAddr.Hex())

	// DEPLOY MINHEAP

	minHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	minHeapAddr, tx, err := DeployLibrary(transactOpts0, backend, "MinHeap", minHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MinHeap: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MinHeap tx: %v", err)
	}

	fmt.Printf("MinHeap deployed at: %v\n", minHeapAddr.Hex())

	// DEPLOY TRANSCODERPOOLS

	transcoderPoolsLibraries := map[string]common.Address{
		"MinHeap": minHeapAddr,
		"MaxHeap": maxHeapAddr,
	}
	transcoderPoolsAddr, tx, err := DeployLibrary(transactOpts0, backend, "TranscoderPools", transcoderPoolsLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy TranscoderPools: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined TranscoderPools tx: %v", err)
	}

	fmt.Printf("TranscoderPools deployed at: %v\n", transcoderPoolsAddr.Hex())

	// DEPLOY MERKLEPROOF

	merkleProofAddr, tx, err := DeployLibrary(transactOpts0, backend, "MerkleProof", nil)

	if err != nil {
		t.Fatalf("Failed to deploy MerkleProof: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MerkleProof tx: %v", err)
	}

	fmt.Printf("MerkleProof deployed at: %v\n", merkleProofAddr.Hex())

	// DEPLOY ECVERIFY

	ecVerifyAddr, tx, err := DeployLibrary(transactOpts0, backend, "ECVerify", nil)

	if err != nil {
		t.Fatalf("Failed to deploy ECVerify: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined ECVerify tx: %v", err)
	}

	fmt.Printf("ECVerify deployed at: %v\n", ecVerifyAddr.Hex())

	// DEPLOY TRANSCODEJOBS

	transcodeJobsLibraries := map[string]common.Address{
		"ECVerify":    ecVerifyAddr,
		"MerkleProof": merkleProofAddr,
	}
	transcodeJobsAddr, tx, err := DeployLibrary(transactOpts0, backend, "TranscodeJobs", transcodeJobsLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy TranscodeJobs: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined TranscodeJobs tx: %v", err)
	}

	fmt.Printf("TranscodeJobs deployed at: %v\n", transcodeJobsAddr.Hex())

	// DEPLOY SAFEMATH

	safeMathAddr, tx, err := DeployLibrary(transactOpts0, backend, "SafeMath", nil)

	if err != nil {
		t.Fatalf("Failed to deploy SafeMath: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined SafeMath tx: %v", err)
	}

	fmt.Printf("SafeMath deployed at: %v\n", safeMathAddr.Hex())

	// DEPLOY LIVEPEERPROTOCOL

	protocolLibraries := map[string]common.Address{
		"Node":            nodeAddr,
		"TranscodeJobs":   transcodeJobsAddr,
		"TranscoderPools": transcoderPoolsAddr,
		"SafeMath":        safeMathAddr,
	}
	protocolAddr, tx, err := DeployLivepeerProtocol(transactOpts0, backend, protocolLibraries, 1, big.NewInt(20), big.NewInt(2))

	if err != nil {
		t.Fatalf("Failed to deploy protocol: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined LivepeerProtocol tx: %v", err)
	}

	fmt.Printf("LivepeerProtocol deployed at: %v\n", protocolAddr.Hex())

	// SETUP CLIENTS

	client0, _ := NewClient(transactOpts0, backend, protocolAddr, rpcTimeout, eventTimeout)
	client1, _ := NewClient(transactOpts1, backend, protocolAddr, rpcTimeout, eventTimeout)
	client2, _ := NewClient(transactOpts2, backend, protocolAddr, rpcTimeout, eventTimeout)
	client3, _ := NewClient(transactOpts3, backend, protocolAddr, rpcTimeout, eventTimeout)

	// TRANSCODER REGISTRATION & BONDING

	tx, err = client0.Transcoder(10, 5, big.NewInt(100))

	if err != nil {
		t.Fatalf("Client failed to call transcoder: %v", err)
	}

	_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	_, err = client0.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 0 failed to bond: %v", err)
	}

	_, err = client1.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 1 failed to bond: %v", err)
	}

	_, err = client2.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 2 failed to bond: %v", err)
	}

	_, err = client3.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 3 failed to bond: %v", err)
	}

	// REWARD

	for i := 0; i < 30; i++ {
		fmt.Printf("Client 0 checking if it should call reward...\n")

		ok, err := client0.CurrentRoundInitialized()

		if err != nil {
			t.Fatalf("Client 0 failed CurrentRoundInitialized: %v", err)
		}

		if !ok {
			tx, err := client0.InitializeRound()

			if err != nil {
				t.Fatalf("Client 0 failed InitializeRound: %v", err)
			}

			_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

			fmt.Printf("Initialized round\n")
		}

		valid, err := client0.ValidRewardTimeWindow()

		if err != nil {
			t.Fatalf("Client 0 failed ValidRewardTimeWindow: %v", err)
		}

		if valid {
			tx, err = client0.Reward()

			if err != nil {
				t.Fatalf("Client 0 failed Reward: %v", err)
			}

			_, err = waitForMinedTx(backend, rpcTimeout, tx.Hash())

			if err != nil {
				t.Fatalf("%v", err)
			}

			cr, cn, crsb, err := client0.RoundInfo()

			if err != nil {
				t.Fatalf("Client 0 failed RoundInfo: %v", err)
			}

			fmt.Printf("Current Round: %v, Cycle #: %v, Current Round Block: %v\n", cr, cn, crsb)
			fmt.Printf("Client 0 called reward\n")
		}

		time.Sleep(2 * time.Second)
	}
}
