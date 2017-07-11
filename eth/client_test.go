package eth

import (
	"fmt"
	"math/big"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	ethTypes "github.com/livepeer/golp/eth/types"
)

var (
	usr, _           = user.Current()
	dir              = usr.HomeDir
	datadir          = filepath.Join(dir, ".lpTest")
	keyStore         = keystore.NewKeyStore(filepath.Join(datadir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)
	defaultPassword  = ""
	rpcTimeout       = 10 * time.Second
	eventTimeout     = 30 * time.Second
	minedTxTimeout   = 60 * time.Second
	testRewardLength = 60
)

func deployContracts(t *testing.T, transactOpts *bind.TransactOpts, backend *ethclient.Client) common.Address {
	var (
		tx  *types.Transaction
		err error
	)

	// DEPLOY NODE

	nodeAddr, tx, err := deployLibrary(transactOpts, backend, Node, nil)

	if err != nil {
		t.Fatalf("Failed to deploy Node: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined Node tx: %v", err)
	}

	// DEPLOY MAXHEAP

	maxHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	maxHeapAddr, tx, err := deployLibrary(transactOpts, backend, MaxHeap, maxHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MaxHeap: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MaxHeap tx: %v", err)
	}

	// DEPLOY MINHEAP

	minHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	minHeapAddr, tx, err := deployLibrary(transactOpts, backend, MinHeap, minHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MinHeap: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MinHeap tx: %v", err)
	}

	// DEPLOY TRANSCODERPOOLS

	transcoderPoolsLibraries := map[string]common.Address{
		"MinHeap": minHeapAddr,
		"MaxHeap": maxHeapAddr,
	}
	transcoderPoolsAddr, tx, err := deployLibrary(transactOpts, backend, TranscoderPools, transcoderPoolsLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy TranscoderPools: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined TranscoderPools tx: %v", err)
	}

	// DEPLOY MERKLEPROOF

	merkleProofAddr, tx, err := deployLibrary(transactOpts, backend, MerkleProof, nil)

	if err != nil {
		t.Fatalf("Failed to deploy MerkleProof: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined MerkleProof tx: %v", err)
	}

	// DEPLOY ECVERIFY

	ecVerifyAddr, tx, err := deployLibrary(transactOpts, backend, ECVerify, nil)

	if err != nil {
		t.Fatalf("Failed to deploy ECVerify: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined ECVerify tx: %v", err)
	}

	// DEPLOY TRANSCODEJOBS

	transcodeJobsLibraries := map[string]common.Address{
		"ECVerify":    ecVerifyAddr,
		"MerkleProof": merkleProofAddr,
	}
	transcodeJobsAddr, tx, err := deployLibrary(transactOpts, backend, TranscodeJobs, transcodeJobsLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy TranscodeJobs: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined TranscodeJobs tx: %v", err)
	}

	// DEPLOY SAFEMATH

	safeMathAddr, tx, err := deployLibrary(transactOpts, backend, SafeMath, nil)

	if err != nil {
		t.Fatalf("Failed to deploy SafeMath: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined SafeMath tx: %v", err)
	}

	// DEPLOY LIVEPEERPROTOCOL

	protocolLibraries := map[string]common.Address{
		"Node":            nodeAddr,
		"TranscodeJobs":   transcodeJobsAddr,
		"TranscoderPools": transcoderPoolsAddr,
		"SafeMath":        safeMathAddr,
	}
	// protocolAddr, tx, err := deployLivepeerProtocol(transactOpts, backend, protocolLibraries, 1, big.NewInt(40), big.NewInt(2))
	protocolAddr, tx, err := deployLivepeerProtocol(transactOpts, backend, protocolLibraries, 1, big.NewInt(20), big.NewInt(2))

	if err != nil {
		t.Fatalf("Failed to deploy protocol: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("Failed to wait for mined LivepeerProtocol tx: %v", err)
	}

	return protocolAddr
}

func TestReward(t *testing.T) {
	var (
		tx  *types.Transaction
		err error
	)

	backend, err := ethclient.Dial(filepath.Join(datadir, "geth.ipc"))

	if err != nil {
		t.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	accounts := keyStore.Accounts()

	transactOpts, err := NewTransactOptsForAccount(accounts[0], defaultPassword, keyStore)

	if err != nil {
		t.Fatalf("Failed to create transact opts: %v", err)
	}

	// DEPLOY

	protocolAddr := deployContracts(t, transactOpts, backend)
	fmt.Printf("Contract addr: %x\n", protocolAddr.String())

	// SETUP CLIENTS

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Failed to create client 0: %v", err)
	}

	client1, err := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Failed to create client 1: %v", err)
	}

	client2, err := NewClient(accounts[2], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Failed to create client 2: %v", err)
	}

	client3, err := NewClient(accounts[3], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Faild to create client 3: %v", err)
	}

	// DISTRIBUTE LPT

	tx, err = client0.Transfer(accounts[1].Address, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer tokens: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	tx, err = client0.Transfer(accounts[2].Address, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer tokens: %v", err)
	}
	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	tx, err = client0.Transfer(accounts[3].Address, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer tokens: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	// TRANSCODER REGISTRATION & BONDING

	// Start at the beginning of a round to avoid timing edge cases in tests
	err = WaitUntilNextRound(backend, rpcTimeout, big.NewInt(40))

	if err != nil {
		t.Fatalf("Failed to wait until next round: %v", err)
	}

	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err = client0.Transcoder(10, 5, big.NewInt(100))

	if err != nil {
		t.Fatalf("Client 0 failed to call transcoder: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err = client0.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 0 failed to bond: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(client1, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	_, err = client1.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 1 failed to bond: %v", err)
	}

	if err := CheckRoundAndInit(client2, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	_, err = client2.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 2 failed to bond: %v", err)
	}

	if err := CheckRoundAndInit(client3, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	_, err = client3.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 3 failed to bond: %v", err)
	}

	// REWARD

	for i := 0; i < testRewardLength; i++ {
		if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
			t.Fatalf("%v", err)
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

			_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

			if err != nil {
				t.Fatalf("%v", err)
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func TestJobClaimVerify(t *testing.T) {
	var (
		tx  *types.Transaction
		err error
	)

	backend, err := ethclient.Dial(filepath.Join(datadir, "geth.ipc"))

	if err != nil {
		t.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	accounts := keyStore.Accounts()

	transactOpts, err := NewTransactOptsForAccount(accounts[0], defaultPassword, keyStore)

	if err != nil {
		t.Fatalf("Failed to create transact opts: %v", err)
	}

	// DEPLOY

	protocolAddr := deployContracts(t, transactOpts, backend)

	// SETUP CLIENTS

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Failed to create client 0: %v", err)
	}

	client1, _ := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)

	if err != nil {
		t.Fatalf("Failed to create client 1: %v", err)
	}

	// DISTRIBUTE LPT

	tx, err = client0.Transfer(accounts[1].Address, big.NewInt(500))

	if err != nil {
		t.Fatalf("Client 0 failed to transfer tokens: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	// TRANSCODER REGISTRATION & BONDING

	// Start at the beginning of a round to avoid timing edge cases in tests
	err = WaitUntilNextRound(backend, rpcTimeout, big.NewInt(40))

	if err != nil {
		t.Fatalf("Failed to wait until next round: %v", err)
	}
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err = client0.Transcoder(10, 5, big.NewInt(100))

	if err != nil {
		t.Fatalf("Client 0 failed to call transcoder: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err = client0.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 0 failed to bond: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	// SUBSCRIBE TO JOB EVENT

	logsSub, logsCh, err := client0.SubscribeToJobEvent(func(l types.Log) error {
		glog.Infof("Got log: %v", l)
		return nil
	})

	if err != nil {
		t.Fatalf("Client 0 failed to subscribe to job event: %v", err)
	}

	// CREATE JOB

	// Start at the beginning of a round
	err = WaitUntilNextRound(backend, rpcTimeout, big.NewInt(40))
	if err := CheckRoundAndInit(client1, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}

	// Stream ID
	streamID := "1"

	dummyTranscodingOptions := common.BytesToHash([]byte{5})
	tx, err = client1.Job(streamID, dummyTranscodingOptions, big.NewInt(150))
	if err != nil {
		t.Fatalf("Client 1 failed to create a job: %v", err)
	}
	receipt, err := WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())
	if err != nil {
		glog.Errorf("%v", err)
		return
	}
	if tx.Gas().Cmp(receipt.GasUsed) == 0 {
		glog.Errorf("Job Creation Failed")
		return
	}

	// time.Sleep(10 * time.Second)

	// CLAIM WORK

	// Segment data hashes
	d0 := common.BytesToHash(common.FromHex("80084bf2fba02475726feb2cab2d8215eab14bc6bdd8bfb2c8151257032ecd8b"))
	d1 := common.BytesToHash(common.FromHex("b039179a8a4ce2c252aa6f2f25798251c19b75fc1508d9d511a191e0487d64a7"))
	d2 := common.BytesToHash(common.FromHex("263ab762270d3b73d3e2cddf9acc893bb6bd41110347e5d5e4bd1d3c128ea90a"))
	d3 := common.BytesToHash(common.FromHex("4ce8765e720c576f6f5a34ca380b3de5f0912e6e3cc5355542c363891e54594b"))

	// Segment hashes
	s0 := &ethTypes.Segment{
		streamID,
		big.NewInt(0),
		d0,
	}

	s1 := &ethTypes.Segment{
		streamID,
		big.NewInt(1),
		d1,
	}

	s2 := &ethTypes.Segment{
		streamID,
		big.NewInt(2),
		d2,
	}

	s3 := &ethTypes.Segment{
		streamID,
		big.NewInt(3),
		d3,
	}

	bSig0, err := client1.SignSegmentHash(defaultPassword, s0.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig1, err := client1.SignSegmentHash(defaultPassword, s1.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig2, err := client1.SignSegmentHash(defaultPassword, s2.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig3, err := client1.SignSegmentHash(defaultPassword, s3.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	// Transcoded data hashes
	tD0 := common.BytesToHash(common.FromHex("42538602949f370aa331d2c07a1ee7ff26caac9cc676288f94b82eb2188b8465"))
	tD1 := common.BytesToHash(common.FromHex("a0b37b8bfae8e71330bd8e278e4a45ca916d00475dd8b85e9352533454c9fec8"))
	tD2 := common.BytesToHash(common.FromHex("9f2898da52dedaca29f05bcac0c8e43e4b9f7cb5707c14cc3f35a567232cec7c"))
	tD3 := common.BytesToHash(common.FromHex("5a082c81a7e4d5833ee20bd67d2f4d736f679da33e4bebd3838217cb27bec1d3"))

	// Transcode claims
	tClaim0 := &ethTypes.TranscodeClaim{
		streamID,
		big.NewInt(0),
		d0,
		tD0,
		bSig0,
	}
	tClaim1 := &ethTypes.TranscodeClaim{
		streamID,
		big.NewInt(1),
		d1,
		tD1,
		bSig1,
	}
	tClaim2 := &ethTypes.TranscodeClaim{
		streamID,
		big.NewInt(2),
		d2,
		tD2,
		bSig2,
	}
	tClaim3 := &ethTypes.TranscodeClaim{
		streamID,
		big.NewInt(3),
		d3,
		tD3,
		bSig3,
	}

	tcHashes := []common.Hash{tClaim0.Hash(), tClaim1.Hash(), tClaim2.Hash(), tClaim3.Hash()}
	tcRoot, proofs, err := ethTypes.NewMerkleTree(tcHashes)

	tx, err = client0.ClaimWork(big.NewInt(0), big.NewInt(0), big.NewInt(3), [32]byte(tcRoot.Hash))

	if err != nil {
		t.Fatalf("Client 0 failed to claim work: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	// VERIFY

	tx, err = client0.Verify(big.NewInt(0), big.NewInt(0), [32]byte(d0), [32]byte(tD0), bSig0, proofs[0].Bytes())

	if err != nil {
		t.Fatalf("Client 0 failed to invoke verify: %v", err)
	}

	receipt, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}

	if tx.Gas().Cmp(receipt.GasUsed) == 0 {
		t.Fatalf("Client 0 failed verification")
	}

	logsSub.Unsubscribe()
	close(logsCh)
}

func TestDeployContract(t *testing.T) {
	backend, err := ethclient.Dial(filepath.Join(datadir, "geth.ipc"))

	if err != nil {
		t.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	accounts := keyStore.Accounts()

	transactOpts, err := NewTransactOptsForAccount(accounts[0], defaultPassword, keyStore)

	if err != nil {
		t.Fatalf("Failed to create transact opts: %v", err)
	}

	// DEPLOY

	protocolAddr := deployContracts(t, transactOpts, backend)
	fmt.Printf("Contract addr: %x\n", protocolAddr.String())

}

func TestTranscoderLoop(t *testing.T) {
	backend, err := ethclient.Dial(filepath.Join(datadir, "geth.ipc"))

	if err != nil {
		t.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	accounts := keyStore.Accounts()

	transactOpts, err := NewTransactOptsForAccount(accounts[0], defaultPassword, keyStore)

	if err != nil {
		t.Fatalf("Failed to create transact opts: %v", err)
	}

	// DEPLOY

	protocolAddr := deployContracts(t, transactOpts, backend)
	fmt.Printf("Contract addr: %x\n", protocolAddr.String())

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client 0: %v", err)
	}

	// TRANSCODER REGISTRATION & BONDING

	// Start at the beginning of a round to avoid timing edge cases in tests
	err = WaitUntilNextRound(backend, rpcTimeout, big.NewInt(40))

	if err != nil {
		t.Fatalf("Failed to wait until next round: %v", err)
	}
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err := client0.Transcoder(10, 5, big.NewInt(100))

	if err != nil {
		t.Fatalf("Client 0 failed to call transcoder: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())

	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}
	tx, err = client0.Bond(big.NewInt(100), accounts[0].Address)

	if err != nil {
		t.Fatalf("Client 0 failed to bond: %v", err)
	}

	receipt, err := WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())
	if err != nil {
		t.Fatalf("%v", err)
	}
	if tx.Gas().Cmp(receipt.GasUsed) == 0 {
		glog.Errorf("Client 0 failed bonding")
	}

	active, err := client0.IsActiveTranscoder()
	if err != nil {
		glog.Errorf("Error getting transcoder state: %v", err)
	}
	// glog.Infof("Transcoder stake: %v", s)
	if !active {
		glog.Infof("Transcoder %v is inactive", accounts[0].Address)
	} else {
		s, err := client0.TranscoderStake()
		if err != nil {
			glog.Errorf("Error getting transcoder stake: %v", err)
		}
		glog.Infof("Transcoder stake: %v", s)
	}

	// SUBSCRIBE TO JOB EVENT

	logsSub, logsCh, err := client0.SubscribeToJobEvent(func(l types.Log) error {
		glog.Infof("Got log: %v", l)
		return nil
	})
	defer logsSub.Unsubscribe()
	defer close(logsCh)

	if err != nil {
		t.Fatalf("Client 0 failed to subscribe to job event: %v", err)
	}

	// CREATE JOB

	// Start at the beginning of a round
	err = WaitUntilNextRound(backend, rpcTimeout, big.NewInt(40))
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}

	// Stream ID
	streamID := "1"

	dummyTranscodingOptions := common.BytesToHash([]byte{5})
	tx, err = client0.Job(streamID, dummyTranscodingOptions, big.NewInt(150))
	if err != nil {
		t.Fatalf("Client 1 failed to create a job: %v", err)
	}
	receipt, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())
	if err != nil {
		glog.Errorf("%v", err)
		return
	}
	if tx.Gas().Cmp(receipt.GasUsed) == 0 {
		glog.Errorf("Job Creation Failed")
		return
	}

	time.Sleep(10 * time.Second)
}
