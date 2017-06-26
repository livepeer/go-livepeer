package eth

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/libp2p-livepeer/eth/contracts"
)

var (
	key0, _ = crypto.GenerateKey()
	key1, _ = crypto.GenerateKey()
	key2, _ = crypto.GenerateKey()
	key3, _ = crypto.GenerateKey()
	addr0   = crypto.PubkeyToAddress(key0.PublicKey)
	addr1   = crypto.PubkeyToAddress(key1.PublicKey)
	addr2   = crypto.PubkeyToAddress(key2.PublicKey)
	addr3   = crypto.PubkeyToAddress(key3.PublicKey)
)

func newTestBackend() *backends.SimulatedBackend {
	return backends.NewSimulatedBackend(core.GenesisAlloc{
		addr0: {Balance: big.NewInt(10000000000)},
		addr1: {Balance: big.NewInt(10000000000)},
		addr2: {Balance: big.NewInt(10000000000)},
		addr3: {Balance: big.NewInt(10000000000)},
	})
}

func deployLivepeerProtocol(transactOpts *bind.TransactOpts, backend *backends.SimulatedBackend) (common.Address, error) {
	nodeAddr, _, _, err := contracts.DeployNode(transactOpts, backend, nil)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	maxHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	maxHeapAddr, _, _, err := contracts.DeployMaxHeap(transactOpts, backend, maxHeapLibraries)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	minHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	minHeapAddr, _, _, err := contracts.DeployMinHeap(transactOpts, backend, minHeapLibraries)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	transcoderPoolsLibraries := map[string]common.Address{
		"MinHeap": minHeapAddr,
		"MaxHeap": maxHeapAddr,
	}
	transcoderPoolsAddr, _, _, err := contracts.DeployTranscoderPools(transactOpts, backend, transcoderPoolsLibraries)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	merkleProofAddr, _, _, err := contracts.DeployMerkleProof(transactOpts, backend, nil)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	ecVerifyAddr, _, _, err := contracts.DeployECVerify(transactOpts, backend, nil)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	transcodeJobsLibraries := map[string]common.Address{
		"ECVerify":    ecVerifyAddr,
		"MerkleProof": merkleProofAddr,
	}
	transcodeJobsAddr, _, _, err := contracts.DeployTranscodeJobs(transactOpts, backend, transcodeJobsLibraries)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	safeMathAddr, _, _, err := contracts.DeploySafeMath(transactOpts, backend, nil)

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	protocolLibraries := map[string]common.Address{
		"Node":            nodeAddr,
		"TranscodeJobs":   transcodeJobsAddr,
		"TranscoderPools": transcoderPoolsAddr,
		"SafeMath":        safeMathAddr,
	}
	protocolAddr, _, _, err := contracts.DeployLivepeerProtocol(transactOpts, backend, protocolLibraries, 1, big.NewInt(1), big.NewInt(1))

	if err != nil {
		return common.Address{}, err
	}

	backend.Commit()

	return protocolAddr, nil
}

func TestLivepeerProtocol(t *testing.T) {
	// DEPLOY

	backend := newTestBackend()

	transactOpts0 := bind.NewKeyedTransactor(key0)
	transactOpts1 := bind.NewKeyedTransactor(key1)
	transactOpts2 := bind.NewKeyedTransactor(key2)
	transactOpts3 := bind.NewKeyedTransactor(key3)

	protocolAddr, err := deployLivepeerProtocol(transactOpts0, backend)

	if err != nil {
		t.Fatalf("Failed to deploy LivepeerProtocol contract: %v", err)
	}

	// SETUP CLIENTS

	client0, err := NewClient(transactOpts0, backend, protocolAddr)

	if err != nil {
		t.Fatalf("Failed to instantiate client 0: %v", err)
	}

	client1, err := NewClient(transactOpts1, backend, protocolAddr)

	if err != nil {
		t.Fatalf("Failed to instantiate client 1: %v", err)
	}

	client2, err := NewClient(transactOpts2, backend, protocolAddr)

	if err != nil {
		t.Fatalf("Failed to instantiate client 2: %v", err)
	}

	client3, err := NewClient(transactOpts3, backend, protocolAddr)

	if err != nil {
		t.Fatalf("Failed to instantiate client 3: %v", err)
	}

	// DISTRIBUTE TOKEN

	_, err = client0.TransferLPT(addr1, big.NewInt(10000))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer LPT to address 1")
	}

	backend.Commit()

	_, err = client0.TransferLPT(addr2, big.NewInt(10000))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer LPT to address 2")
	}

	backend.Commit()

	_, err = client0.TransferLPT(addr3, big.NewInt(10000))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer LPT to address 3")
	}

	backend.Commit()

	// TRANSCODER REGISTRATION & BONDING

	_, err = client0.InitializeRound()

	if err != nil {
		t.Fatalf("Client failed to initialize the round: %v", err)
	}

	backend.Commit()

	_, err = client0.Transcoder(10, 5, big.NewInt(100))

	if err != nil {
		t.Fatalf("Client failed to call transcoder: %v", err)
	}

	backend.Commit()

	_, err = client0.Approve(protocolAddr, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to approve LPT amount: %v", err)
	}

	backend.Commit()

	_, err = client0.Bond(addr0, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to bond LPT to address 0: %v", err)
	}

	backend.Commit()

	_, err = client1.Approve(protocolAddr, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 1 failed to approve LPT amount: %v", err)
	}

	backend.Commit()

	_, err = client1.Bond(addr0, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 1 failed to bond LPT to address 0: %v", err)
	}

	backend.Commit()

	_, err = client2.Approve(protocolAddr, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 2 failed to approve LPT amount: %v", err)
	}

	backend.Commit()

	_, err = client2.Bond(addr0, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 2 failed to bond LPT to address 0: %v", err)
	}

	backend.Commit()

	_, err = client3.Approve(protocolAddr, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 3 failed to approve LPT amount: %v", err)
	}

	backend.Commit()

	_, err = client3.Bond(addr0, big.NewInt(200))

	if err != nil {
		t.Fatalf("Client 3 failed to bond LPT to address 0: %v", err)
	}

	backend.Commit()
}
