package eth

import (
	"context"
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
	"github.com/livepeer/golp/net"
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
	err = client0.WaitUntilNextRound(big.NewInt(40))

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
	err = client0.WaitUntilNextRound(big.NewInt(40))

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

	logsCh := make(chan types.Log)
	logsSub, err := client0.SubscribeToJobEvent(context.Background(), logsCh)
	defer close(logsCh)
	defer logsSub.Unsubscribe()

	if err != nil {
		t.Fatalf("Client 0 failed to subscribe to job event: %v", err)
	}

	// CREATE JOB

	// Start at the beginning of a round
	err = client1.WaitUntilNextRound(big.NewInt(40))
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

	time.Sleep(3 * time.Second)

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

	bSig0, err := SignSegmentHash(client1, defaultPassword, s0.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig1, err := SignSegmentHash(client1, defaultPassword, s1.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig2, err := SignSegmentHash(client1, defaultPassword, s2.Hash().Bytes())

	if err != nil {
		t.Fatalf("Client 1 failed to sign segment hash: %v", err)
	}

	bSig3, err := SignSegmentHash(client1, defaultPassword, s3.Hash().Bytes())

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

	// logsSub.Unsubscribe()
	// close(logsCh)
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

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)
	b0, _ := client0.TokenBalance()
	glog.Infof("Token balance for %v: %v", accounts[0].Address.Hex(), b0)

	tx, _ := client0.Transfer(accounts[1].Address, big.NewInt(1000000000000))
	client1, err := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)
	b1, _ := client1.TokenBalance()
	WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash())
	glog.Infof("Token balance for %v: %v", accounts[1].Address.Hex(), b1)
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
	// protocolAddr := common.HexToAddress("0xb3c840ad7a8681b7d7e4a5133fa0d60237496cab")

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client 0: %v", err)
	}

	// TRANSCODER REGISTRATION & BONDING

	// Start at the beginning of a round to avoid timing edge cases in tests
	err = client0.WaitUntilNextRound(big.NewInt(20))

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

	logsCh := make(chan types.Log)
	logsSub, err := client0.SubscribeToJobEvent(context.Background(), logsCh)
	defer logsSub.Unsubscribe()
	defer close(logsCh)
	go func() {
		for {
			select {
			case l := <-logsCh:
				glog.Infof("Got log: %v - addr: %v - ", l.Data, l.Address.Hex())
				// glog.Infof("l: %v", l)
				tx, _, err := client0.backend.TransactionByHash(context.Background(), l.TxHash)
				if err != nil {
					glog.Errorf("Error getting transaction: %v", err)
				}
				glog.Infof("transaction data: %v", tx.Data())

				strmId, tData, err := ParseJobTxData(tx.Data())
				if err != nil {
					glog.Errorf("Error parsing job tx data: %v", err)
				}
				glog.Infof("strmId: %v, tData: %v", strmId, tData)
			}
		}
	}()

	if err != nil {
		t.Fatalf("Client 0 failed to subscribe to job event: %v", err)
	}

	// CREATE JOB

	// Start at the beginning of a round
	err = client0.WaitUntilNextRound(big.NewInt(20))
	if err := CheckRoundAndInit(client0, rpcTimeout, minedTxTimeout); err != nil {
		t.Fatalf("%v", err)
	}

	// Stream ID
	streamID := "122098300ddff9dbca808ade6dfcd9ac904bf5eed0bab78ad93f6d366b5a5a5905452ec56b878673a51f52ca0393f0217ae11e79587527cfbcd052a1de1e5782228f"

	var d [32]byte
	dbs := []byte(net.P_720P_60FPS_16_9.Name)
	copy(d[:], dbs[:31])
	tx, err = client0.Job(streamID, d, big.NewInt(150))
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
	glog.Infof("Job Transaction Data: %v", tx.Data())

	for i := 0; i < 1000; i++ {
		jid, jdata, price, baddr, taddr, endblock, err := client0.JobDetails(big.NewInt(int64(i)))

		if err != nil {
			glog.Errorf("Error getting job detail: %v", err)
			return
		}
		glog.Infof("jid: %v, jdata: %v, price: %v, baddr: %v, taddr: %v, endblk: %v", jid, jdata, price, baddr, taddr, endblock)
	}
	time.Sleep(10 * time.Second)
}

func TestParse(t *testing.T) {
	d := []byte{142, 203, 126, 151, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 96, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 80, 50, 52, 48, 80, 51, 48, 70, 80, 83, 52, 51, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 150, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 132, 49, 50, 50, 48, 50, 54, 54, 49, 54, 51, 99, 55, 99, 101, 52, 98, 97, 54, 50, 54, 53, 50, 55, 49, 100, 54, 56, 48, 55, 51, 101, 49, 102, 57, 53, 50, 98, 101, 54, 51, 98, 49, 49, 97, 51, 99, 99, 49, 101, 48, 53, 97, 50, 51, 97, 97, 56, 55, 54, 53, 57, 99, 100, 102, 97, 99, 56, 101, 56, 52, 53, 56, 97, 101, 54, 48, 57, 56, 97, 50, 54, 54, 52, 53, 97, 99, 50, 100, 49, 97, 99, 102, 53, 102, 51, 52, 102, 55, 97, 48, 54, 56, 52, 56, 99, 97, 55, 50, 57, 55, 97, 98, 56, 48, 56, 98, 51, 57, 101, 49, 57, 54, 98, 48, 55, 55, 49, 50, 51, 100, 98, 55, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	sid, v, err := ParseJobTxData(d)
	if err != nil {
		t.Errorf("Error parsing job data: %v", err)
	}
	if v != "P240P30FPS43" {
		t.Errorf("Expecting P240P30FPS43, got %v", v)
	}
	if sid != "1220266163c7ce4ba6265271d68073e1f952be63b11a3cc1e05a23aa87659cdfac8e8458ae6098a26645ac2d1acf5f34f7a06848ca7297ab808b39e196b077123db7" {
		t.Errorf("Expecting 1220266163c7ce4ba6265271d68073e1f952be63b11a3cc1e05a23aa87659cdfac8e8458ae6098a26645ac2d1acf5f34f7a06848ca7297ab808b39e196b077123db7, got %v", sid)
	}
}
