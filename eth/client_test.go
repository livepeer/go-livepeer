package eth

import (
	// "fmt"
	"context"
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
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	lpTypes "github.com/livepeer/go-livepeer/types"
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

func deployContracts(t *testing.T, transactOpts *bind.TransactOpts, backend *ethclient.Client) (common.Address, common.Address, common.Address, common.Address, common.Address) {
	var (
		tx  *types.Transaction
		err error
	)

	// DEPLOY NODE

	nodeAddr, tx, err := deployLibrary(transactOpts, backend, Node, nil)
	if err != nil {
		t.Fatalf("Failed to deploy Node: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined Node tx: %v", err)
	}

	// DEPLOY MAXHEAP

	maxHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	maxHeapAddr, tx, err := deployLibrary(transactOpts, backend, MaxHeap, maxHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MaxHeap: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined MaxHeap tx: %v", err)
	}

	// DEPLOY MINHEAP

	minHeapLibraries := map[string]common.Address{"Node": nodeAddr}
	minHeapAddr, tx, err := deployLibrary(transactOpts, backend, MinHeap, minHeapLibraries)

	if err != nil {
		t.Fatalf("Failed to deploy MinHeap: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

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

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined TranscoderPools tx: %v", err)
	}

	// DEPLOY MERKLEPROOF

	merkleProofAddr, tx, err := deployLibrary(transactOpts, backend, MerkleProof, nil)

	if err != nil {
		t.Fatalf("Failed to deploy MerkleProof: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined MerkleProof tx: %v", err)
	}

	// DEPLOY ECRECOVERY

	ecRecoveryAddr, tx, err := deployLibrary(transactOpts, backend, ECRecovery, nil)

	if err != nil {
		t.Fatalf("Failed to deploy ECRecovery: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined ECRecovery tx: %v", err)
	}

	// DEPLOY JOBLIB

	jobLibAddr, tx, err := deployLibrary(transactOpts, backend, JobLib, nil)

	if err != nil {
		t.Fatalf("Failed to deploy JobLib: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined JobLib tx: %v", err)
	}

	// DEPLOY SAFEMATH

	safeMathAddr, tx, err := deployLibrary(transactOpts, backend, SafeMath, nil)

	if err != nil {
		t.Fatalf("Failed to deploy SafeMath: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined SafeMath tx: %v", err)
	}

	// DEPLOY IDENTITYVERIFIER

	identityVerifierAddr, tx, err := deployIdentityVerifier(transactOpts, backend)

	if err != nil {
		t.Fatalf("Failed to deploy IdentityVerifier: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined IdentityVerifier tx: %v", err)
	}

	// DEPLOY LIVEPEERTOKEN

	tokenAddr, tx, err := deployLivepeerToken(transactOpts, backend)

	if err != nil {
		t.Fatalf("Failed to deploy LivepeerToken: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined LivepeerToken tx: %v", err)
	}

	// DEPLOY LIVEPEERPROTOCOL

	protocolAddr, tx, err := deployLivepeerProtocol(transactOpts, backend)

	if err != nil {
		t.Fatalf("Failed to deploy LivepeerProtocol: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined LivepeerProtocol tx: %v", err)
	}

	// DEPLOY BONDINGMANAGER

	bondingManagerLibraries := map[string]common.Address{
		"TranscoderPools": transcoderPoolsAddr,
		"SafeMath":        safeMathAddr,
	}
	bondingManagerAddr, tx, err := deployBondingManager(transactOpts, backend, bondingManagerLibraries, protocolAddr, tokenAddr, big.NewInt(1))

	if err != nil {
		t.Fatalf("Failed to deploy BondingManager: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined BondingManager tx: %v", err)
	}

	// DEPLOY JOBSMANAGER

	jobsManagerLibraries := map[string]common.Address{
		"MerkleProof": merkleProofAddr,
		"ECRecovery":  ecRecoveryAddr,
		"JobLib":      jobLibAddr,
		"SafeMath":    safeMathAddr,
	}
	jobsManagerAddr, tx, err := deployJobsManager(transactOpts, backend, jobsManagerLibraries, protocolAddr, tokenAddr, identityVerifierAddr)

	if err != nil {
		t.Fatalf("Failed to deploy JobsManager: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined JobsManager tx: %v", err)
	}

	// DEPLOY ROUNDSMANAGER

	roundsManagerLibraries := map[string]common.Address{
		"SafeMath": safeMathAddr,
	}
	roundsManagerAddr, tx, err := deployRoundsManager(transactOpts, backend, roundsManagerLibraries, protocolAddr)

	if err != nil {
		t.Fatalf("Failed to deploy RoundsManager: %v", err)
	}

	_, err = WaitForMinedTx(backend, rpcTimeout, minedTxTimeout, tx.Hash(), tx.Gas())

	if err != nil {
		t.Fatalf("Failed to wait for mined RoundsManager tx: %v", err)
	}

	return protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr
}

func checkTxReceipt(t *testing.T, receiptCh <-chan types.Receipt, errCh <-chan error) {
	select {
	case <-receiptCh:
		return
	case err := <-errCh:
		t.Fatalf("%v", err)
	}
}

func TestReward(t *testing.T) {
	var (
		err error

		blockRewardCut  uint8 = 10
		feeShare        uint8 = 5
		pricePerSegment       = big.NewInt(10)

		transcoderBond = big.NewInt(100)
		delegatorBond0 = big.NewInt(100)
		delegatorBond1 = big.NewInt(100)
		delegatorBond2 = big.NewInt(100)

		receiptCh <-chan types.Receipt
		errCh     <-chan error
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

	protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr := deployContracts(t, transactOpts, backend)
	distributeTokens(transactOpts, backend, rpcTimeout, minedTxTimeout, tokenAddr, big.NewInt(1000000), accounts)
	initProtocol(transactOpts, backend, rpcTimeout, minedTxTimeout, protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr)

	// SETUP CLIENTS

	transcoderClient, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	delegatorClient0, err := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	delegatorClient1, err := NewClient(accounts[2], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	delegatorClient2, err := NewClient(accounts[3], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failedd to create client: %v", err)
	}

	// TRANSCODER REGISTRATION & BONDING
	timeParams, err := transcoderClient.ProtocolTimeParams()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Start at the beginning of a round to avoid timing edge cases in tests
	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(transcoderClient); err != nil {
		t.Fatalf("%v", err)
	}

	receiptCh, errCh = transcoderClient.Transcoder(blockRewardCut, feeShare, pricePerSegment)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = transcoderClient.Bond(transcoderBond, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = delegatorClient0.Bond(delegatorBond0, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = delegatorClient1.Bond(delegatorBond1, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = delegatorClient2.Bond(delegatorBond2, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	// REWARD

	// Wait until new round
	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("%v", err)
	}

	lastRewardRound, err := transcoderClient.LastRewardRound()
	if err != nil {
		t.Fatalf("Failed to get last reward round: %v", err)
	}

	for i := 0; i < testRewardLength; i++ {
		cr, _, _, err := transcoderClient.RoundInfo()
		if err != nil {
			t.Fatalf("Failed to get round info: %v", err)
		}

		if lastRewardRound.Cmp(cr) == -1 {
			if err := CheckRoundAndInit(transcoderClient); err != nil {
				t.Fatalf("%v", err)
			}

			receiptCh, errCh = transcoderClient.Reward()
			checkTxReceipt(t, receiptCh, errCh)

			lastRewardRound = cr

			glog.Infof("Last reward round updated to: %v", cr)
		}

		time.Sleep(2 * time.Second)
	}
}

func TestJobClaimVerify(t *testing.T) {
	var (
		err error

		receiptCh <-chan types.Receipt
		errCh     <-chan error

		blockRewardCut  uint8 = 10
		feeShare        uint8 = 5
		pricePerSegment       = big.NewInt(1000)

		transcoderBond = big.NewInt(100)
		delegatorBond  = big.NewInt(100)

		streamID           = "122098300ddff9dbca808ade6dfcd9ac904bf5eed0bab78ad93f6d366b5a5a5905452ec56b878673a51f52ca0393f0217ae11e79587527cfbcd052a1de1e5782228f"
		transcodingOptions = lpTypes.P720p60fps16x9.Name
		maxPricePerSegment = big.NewInt(1000)

		jobDeposit = big.NewInt(10000)
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

	protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr := deployContracts(t, transactOpts, backend)
	distributeTokens(transactOpts, backend, rpcTimeout, minedTxTimeout, tokenAddr, big.NewInt(1000000), accounts)
	initProtocol(transactOpts, backend, rpcTimeout, minedTxTimeout, protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr)

	// SETUP CLIENTS

	transcoderClient, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create transcoder client: %v", err)
	}

	delegatorClient, _ := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create delegator client: %v", err)
	}

	broadcasterClient, _ := NewClient(accounts[2], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create broadcaster client: %v", err)
	}

	// TRANSCODER REGISTRATION & BONDING
	timeParams, err := transcoderClient.ProtocolTimeParams()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("Failed to wait until next round: %v", err)
	}

	if err := CheckRoundAndInit(transcoderClient); err != nil {
		t.Fatalf("%v", err)
	}

	receiptCh, errCh = transcoderClient.Transcoder(blockRewardCut, feeShare, pricePerSegment)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = transcoderClient.Bond(transcoderBond, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = delegatorClient.Bond(delegatorBond, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	// SUBSCRIBE TO JOB EVENT

	logsCh := make(chan types.Log)
	logsSub, err := transcoderClient.SubscribeToJobEvent(context.Background(), logsCh)
	if err != nil {
		t.Fatalf("Failed to subscribe to job event: %v", err)
	}

	defer close(logsCh)
	defer logsSub.Unsubscribe()

	// CREATE JOB

	// Start at the beginning of a round
	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(broadcasterClient); err != nil {
		t.Fatalf("%v", err)
	}

	receiptCh, errCh = broadcasterClient.Deposit(jobDeposit)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = broadcasterClient.Job(streamID, transcodingOptions, maxPricePerSegment)
	checkTxReceipt(t, receiptCh, errCh)

	time.Sleep(3 * time.Second)

	// CLAIM WORK

	// Segment data hashes
	d0 := "QmXcDGvp2w9Bu84pQujZQyntyCtymvy4N4UBAEzxS14Bnv"
	d1 := "QmU2ywYUuLd97uCfo1dknZosB6Qem7e32JQuMtBDUkoxu3"
	d2 := "QmU2ywYUuLd97uCfo1dknZosB6Qem7e32JQuMtBDUkoxu3"
	d3 := "QmTeVUJb41cHyLi6AT7t5Hrh8swyR1uRiU6JTLJZUhujuH"

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

	bSig0, err := broadcasterClient.SignSegmentHash(defaultPassword, s0.Hash().Bytes())
	if err != nil {
		t.Fatalf("Failed to sign segment hash: %v", err)
	}

	bSig1, err := broadcasterClient.SignSegmentHash(defaultPassword, s1.Hash().Bytes())
	if err != nil {
		t.Fatalf("Failed to sign segment hash: %v", err)
	}

	bSig2, err := broadcasterClient.SignSegmentHash(defaultPassword, s2.Hash().Bytes())
	if err != nil {
		t.Fatalf("Failed to sign segment hash: %v", err)
	}

	bSig3, err := broadcasterClient.SignSegmentHash(defaultPassword, s3.Hash().Bytes())
	if err != nil {
		t.Fatalf("Failed to sign segment hash: %v", err)
	}

	// Transcoded data hashes
	tD0 := "0x42538602949f370aa331d2c07a1ee7ff26caac9cc676288f94b82eb2188b8465"
	tD1 := "0xa0b37b8bfae8e71330bd8e278e4a45ca916d00475dd8b85e9352533454c9fec8"
	tD2 := "0x9f2898da52dedaca29f05bcac0c8e43e4b9f7cb5707c14cc3f35a567232cec7c"
	tD3 := "0x5a082c81a7e4d5833ee20bd67d2f4d736f679da33e4bebd3838217cb27bec1d3"

	// Transcode receipts
	tReceipt0 := &ethTypes.TranscodeReceipt{
		streamID,
		big.NewInt(0),
		d0,
		tD0,
		bSig0,
	}
	tReceipt1 := &ethTypes.TranscodeReceipt{
		streamID,
		big.NewInt(1),
		d1,
		tD1,
		bSig1,
	}
	tReceipt2 := &ethTypes.TranscodeReceipt{
		streamID,
		big.NewInt(2),
		d2,
		tD2,
		bSig2,
	}
	tReceipt3 := &ethTypes.TranscodeReceipt{
		streamID,
		big.NewInt(3),
		d3,
		tD3,
		bSig3,
	}

	receiptHashes := []common.Hash{tReceipt0.Hash(), tReceipt1.Hash(), tReceipt2.Hash(), tReceipt3.Hash()}
	claimRoot, proofs, err := ethTypes.NewMerkleTree(receiptHashes)

	receiptCh, errCh = transcoderClient.ClaimWork(big.NewInt(0), [2]*big.Int{big.NewInt(0), big.NewInt(3)}, [32]byte(claimRoot.Hash))
	checkTxReceipt(t, receiptCh, errCh)

	// VERIFY

	receiptCh, errCh = transcoderClient.Verify(big.NewInt(0), big.NewInt(0), big.NewInt(0), d0, tD0, bSig0, proofs[0].Bytes())
	checkTxReceipt(t, receiptCh, errCh)

	// DISTRIBUTE FEES

	if err := Wait(backend, rpcTimeout, new(big.Int).Add(timeParams.VerificationPeriod, timeParams.SlashingPeriod)); err != nil {
		t.Fatalf("%v", err)
	}

	receiptCh, errCh = transcoderClient.DistributeFees(big.NewInt(0), big.NewInt(0))
	checkTxReceipt(t, receiptCh, errCh)

	transcoderBalance, _ := transcoderClient.TranscoderBond()
	glog.Infof("Transcoder bond: %v", transcoderBalance)

	delegatorBalance, _ := delegatorClient.DelegatorStake()
	glog.Infof("Delegator stake: %v", delegatorBalance)
}

func TestDeployContract(t *testing.T) {
	var (
		receiptCh <-chan types.Receipt
		errCh     <-chan error
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

	protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr := deployContracts(t, transactOpts, backend)
	distributeTokens(transactOpts, backend, rpcTimeout, minedTxTimeout, tokenAddr, big.NewInt(1000000), accounts)
	initProtocol(transactOpts, backend, rpcTimeout, minedTxTimeout, protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr)

	client0, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)

	b0, _ := client0.TokenBalance()
	glog.Infof("Token balance for %v: %v", accounts[0].Address.Hex(), b0)

	receiptCh, errCh = client0.Transfer(accounts[1].Address, big.NewInt(1000))
	checkTxReceipt(t, receiptCh, errCh)

	client1, err := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	b1, _ := client1.TokenBalance()
	glog.Infof("Token balance for %v: %v", accounts[1].Address.Hex(), b1)
	b0, _ = client0.TokenBalance()
	glog.Infof("Token balance for %v: %v", accounts[0].Address.Hex(), b0)
}

func TestTranscoderLoop(t *testing.T) {
	var (
		err error

		receiptCh <-chan types.Receipt
		errCh     <-chan error

		blockRewardCut  uint8 = 10
		feeShare        uint8 = 5
		pricePerSegment       = big.NewInt(1000)

		transcoderBond = big.NewInt(100)

		streamID           = "122098300ddff9dbca808ade6dfcd9ac904bf5eed0bab78ad93f6d366b5a5a5905452ec56b878673a51f52ca0393f0217ae11e79587527cfbcd052a1de1e5782228f"
		transcodingOptions = lpTypes.P720p60fps16x9.Name
		maxPricePerSegment = big.NewInt(1000)
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

	protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr := deployContracts(t, transactOpts, backend)
	distributeTokens(transactOpts, backend, rpcTimeout, minedTxTimeout, tokenAddr, big.NewInt(1000000), accounts)
	initProtocol(transactOpts, backend, rpcTimeout, minedTxTimeout, protocolAddr, tokenAddr, bondingManagerAddr, jobsManagerAddr, roundsManagerAddr)

	transcoderClient, err := NewClient(accounts[0], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	broadcasterClient, err := NewClient(accounts[1], defaultPassword, datadir, backend, protocolAddr, tokenAddr, rpcTimeout, eventTimeout)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// TRANSCODER REGISTRATION & BONDING
	timeParams, err := transcoderClient.ProtocolTimeParams()
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Start at the beginning of a round to avoid timing edge cases in tests
	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(transcoderClient); err != nil {
		t.Fatalf("%v", err)
	}

	receiptCh, errCh = transcoderClient.Transcoder(blockRewardCut, feeShare, pricePerSegment)
	checkTxReceipt(t, receiptCh, errCh)

	receiptCh, errCh = transcoderClient.Bond(transcoderBond, accounts[0].Address)
	checkTxReceipt(t, receiptCh, errCh)

	// SUBSCRIBE TO JOB EVENT

	logsCh := make(chan types.Log)
	logsSub, err := transcoderClient.SubscribeToJobEvent(context.Background(), logsCh)
	if err != nil {
		t.Fatalf("Failed to subscribe to job event: %v", err)
	}

	defer logsSub.Unsubscribe()
	defer close(logsCh)

	go func() {
		for {
			l, ok := <-logsCh

			if !ok {
				return
			} else {
				glog.Infof("Got log from %v. Topics: %v Data: %v", l.Address.Hex(), l.Topics, l.Data)
				glog.Infof("Transcoder address: %v", common.BytesToAddress(l.Topics[1].Bytes()).Hex())
				glog.Infof("Broadcaster address: %v", common.BytesToAddress(l.Topics[2].Bytes()).Hex())

				newJobID := new(big.Int).SetBytes(l.Data[0:32])
				glog.Infof("Job id: %v", newJobID)

				job, _ := transcoderClient.GetJob(newJobID)
				glog.Infof("Stream id: %v", job.StreamId)
				glog.Infof("Transcoding options: %v", job.TranscodingOptions)
				glog.Infof("Max price per segment: %v", job.MaxPricePerSegment)
				glog.Infof("Escrow: %v", job.Escrow)
			}
		}
	}()

	// CREATE JOB

	// Start at the beginning of a round
	if err := WaitUntilBlockMultiple(backend, rpcTimeout, timeParams.RoundLength); err != nil {
		t.Fatalf("%v", err)
	}

	if err := CheckRoundAndInit(broadcasterClient); err != nil {
		t.Fatalf("%v", err)
	}

	// Stream ID

	receiptCh, errCh = transcoderClient.Job(streamID, transcodingOptions, maxPricePerSegment)
	checkTxReceipt(t, receiptCh, errCh)

	time.Sleep(10 * time.Second)
}
