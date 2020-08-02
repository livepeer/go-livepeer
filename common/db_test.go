package common

import (
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateKVStore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	expectedChainID := "1337"

	dbh, dbraw, err := TempDB(t)
	require.Nil(err)

	defer dbh.Close()
	defer dbraw.Close()

	var chainID string
	row := dbraw.QueryRow("SELECT value FROM kv WHERE key = 'chainID'")
	err = row.Scan(&chainID)
	assert.EqualError(err, "sql: no rows in result set")
	assert.Equal("", chainID)

	dbh.updateKVStore("chainID", expectedChainID)
	row = dbraw.QueryRow("SELECT value FROM kv WHERE key = 'chainID'")
	err = row.Scan(&chainID)
	assert.Nil(err)
	assert.Equal(expectedChainID, chainID)
}

func TestSelectKVStore(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	key := "foo"
	value := "bar"

	dbh, dbraw, err := TempDB(t)
	require.Nil(err)
	defer dbh.Close()
	defer dbraw.Close()

	err = dbh.updateKVStore(key, value)
	require.Nil(err)

	val, err := dbh.selectKVStore(key)
	assert.Nil(err)
	assert.Equal(val, value)
}

func TestChainID(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	expectedChainID := "1337"

	dbh, dbraw, err := TempDB(t)
	require.Nil(err)

	defer dbh.Close()
	defer dbraw.Close()

	expectedChainIDInt, ok := new(big.Int).SetString(expectedChainID, 10)
	require.True(ok)
	dbh.SetChainID(expectedChainIDInt)

	chainID, err := dbh.ChainID()
	assert.Nil(err)
	assert.Equal(chainID.String(), expectedChainID)
}

func TestSetChainID(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	expectedChainID := "1337"

	dbh, dbraw, err := TempDB(t)
	require.Nil(err)

	defer dbh.Close()
	defer dbraw.Close()

	expectedChainIDInt, ok := new(big.Int).SetString(expectedChainID, 10)
	require.True(ok)
	dbh.SetChainID(expectedChainIDInt)

	chainID, err := dbh.ChainID()
	assert.Nil(err)
	assert.Equal(chainID, expectedChainIDInt)
}

func TestDBLastSeenBlock(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	if err != nil {
		return
	}
	defer dbh.Close()
	defer dbraw.Close()

	assert := assert.New(t)
	require := require.New(t)

	// When there are no headers, return nil
	blk, err := dbh.LastSeenBlock()
	assert.Nil(err)
	assert.Nil(blk)

	// When there is a single header, return its number
	h0 := defaultMiniHeader()
	h0.Number = big.NewInt(100)
	err = dbh.InsertMiniHeader(h0)
	require.Nil(err)

	blk, err = dbh.LastSeenBlock()
	assert.Nil(err)
	assert.Equal(h0.Number, blk)

	// When there are multiple headers, return the latest header number
	h1 := defaultMiniHeader()
	h1.Number = big.NewInt(101)
	err = dbh.InsertMiniHeader(h1)
	require.Nil(err)

	blk, err = dbh.LastSeenBlock()
	assert.Nil(err)
	assert.Equal(h1.Number, blk)

	h2 := defaultMiniHeader()
	h2.Number = big.NewInt(99)
	err = dbh.InsertMiniHeader(h2)
	require.Nil(err)

	blk, err = dbh.LastSeenBlock()
	assert.Nil(err)
	assert.Equal(h1.Number, blk)
}

func TestDBVersion(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	if err != nil {
		return
	}
	defer dbh.Close()
	defer dbraw.Close()

	// sanity check db version matches
	var dbVersion int
	row := dbraw.QueryRow("SELECT value FROM kv WHERE key = 'dbVersion'")
	err = row.Scan(&dbVersion)
	if err != nil || dbVersion != LivepeerDBVersion {
		t.Errorf("Unexpected result from sanity check; got %v - %v", err, dbVersion)
		return
	}

	// ensure error when db version > current node version
	stmt := fmt.Sprintf("UPDATE kv SET value='%v' WHERE key='dbVersion'", LivepeerDBVersion+1)
	_, err = dbraw.Exec(stmt)
	if err != nil {
		t.Error("Could not update dbversion", err)
		return
	}
	dbh2, err := InitDB(dbPath(t))
	if err == nil || err != ErrDBTooNew {
		t.Error("Did not get expected error DBTooNew; got ", err)
		return
	}
	if dbh2 != nil {
		dbh.Close()
	}
}

func profilesMatch(j1 []ffmpeg.VideoProfile, j2 []ffmpeg.VideoProfile) bool {
	if len(j1) != len(j2) {
		return false
	}
	for i, v := range j1 {
		if j2[i] != v {
			return false
		}
	}
	return true
}

func TestSelectUpdateOrchs_EmptyOrNilInputs_NoError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	// selecting empty set of orchs
	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Empty(orchs)

	// updating a nil value
	err = dbh.UpdateOrch(nil)
	require.Nil(err)
}

func TestSelectUpdateOrchs_AddingUpdatingRow_NoError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	// adding row
	orchAddress := pm.RandAddress().String()
	orch := &DBOrch{
		EthereumAddr:      orchAddress,
		ServiceURI:        "127.0.0.1:8936",
		PricePerPixel:     1,
		ActivationRound:   0,
		DeactivationRound: 0,
	}

	err = dbh.UpdateOrch(orch)
	require.Nil(err)

	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 1)
	assert.Equal(orchs[0].ServiceURI, orch.ServiceURI)
	// Default value for stake should be 0
	assert.Equal(orchs[0].Stake, int64(0))

	// updating row with same orchAddress
	orchUpdate := NewDBOrch(orchAddress, "127.0.0.1:8937", 1000, 5, 10, 50)
	err = dbh.UpdateOrch(orchUpdate)
	require.Nil(err)

	updatedOrch, err := dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.Equal(updatedOrch[0].ServiceURI, orchUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, orchUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, orchUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, orchUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, orchUpdate.Stake)

	// updating only serviceURI
	serviceURIUpdate := &DBOrch{
		EthereumAddr: orchAddress,
		ServiceURI:   "127.0.0.1:8938",
	}
	err = dbh.UpdateOrch(serviceURIUpdate)
	require.Nil(err)

	updatedOrch, err = dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.Equal(updatedOrch[0].ServiceURI, serviceURIUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, orchUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, orchUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, orchUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, orchUpdate.Stake)

	// udpating only pricePerPixel
	priceUpdate := &DBOrch{
		EthereumAddr:  orchAddress,
		PricePerPixel: 99,
	}
	err = dbh.UpdateOrch(priceUpdate)
	require.Nil(err)

	updatedOrch, err = dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.Equal(updatedOrch[0].ServiceURI, serviceURIUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, orchUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, orchUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, priceUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, orchUpdate.Stake)

	// updating only activationRound
	activationRoundUpdate := &DBOrch{
		EthereumAddr:    orchAddress,
		ActivationRound: 304,
	}
	err = dbh.UpdateOrch(activationRoundUpdate)
	require.Nil(err)

	updatedOrch, err = dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.Equal(updatedOrch[0].ServiceURI, serviceURIUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, activationRoundUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, orchUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, priceUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, orchUpdate.Stake)

	// updating only deactivationRound
	deactivationRoundUpdate := &DBOrch{
		EthereumAddr:      orchAddress,
		DeactivationRound: 597,
	}
	err = dbh.UpdateOrch(deactivationRoundUpdate)
	require.Nil(err)

	updatedOrch, err = dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.Equal(updatedOrch[0].ServiceURI, serviceURIUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, activationRoundUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, deactivationRoundUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, priceUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, orchUpdate.Stake)

	// Updating only stake
	stakeUpdate := &DBOrch{
		EthereumAddr: orchAddress,
		Stake:        1000,
	}
	err = dbh.UpdateOrch(stakeUpdate)
	require.Nil(err)

	updatedOrch, err = dbh.SelectOrchs(nil)
	assert.Len(updatedOrch, 1)
	assert.NoError(err)
	assert.Equal(updatedOrch[0].ServiceURI, serviceURIUpdate.ServiceURI)
	assert.Equal(updatedOrch[0].ActivationRound, activationRoundUpdate.ActivationRound)
	assert.Equal(updatedOrch[0].DeactivationRound, deactivationRoundUpdate.DeactivationRound)
	assert.Equal(updatedOrch[0].PricePerPixel, priceUpdate.PricePerPixel)
	assert.Equal(updatedOrch[0].Stake, stakeUpdate.Stake)
}

func TestSelectUpdateOrchs_AddingMultipleRows_NoError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	assert := assert.New(t)
	require.Nil(err)

	// adding one row
	orchAddress := pm.RandAddress().String()

	orch := NewDBOrch(orchAddress, "127.0.0.1:8936", 1, 0, 0, 0)
	err = dbh.UpdateOrch(orch)
	require.Nil(err)

	orchs, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchs, 1)
	assert.Equal(orchs[0].ServiceURI, orch.ServiceURI)

	// adding second row
	orchAddress = pm.RandAddress().String()

	orchAdd := NewDBOrch(orchAddress, "127.0.0.1:8938", 1, 0, 0, 0)
	err = dbh.UpdateOrch(orchAdd)
	require.Nil(err)

	orchsUpdated, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchsUpdated, 2)
	assert.Equal(orchsUpdated[1].ServiceURI, orchAdd.ServiceURI)
}

func TestOrchCount(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	var orchList []string
	var nilDB *DB
	zeroOrchs, nilErr := nilDB.OrchCount(&DBOrchFilter{})
	assert.Zero(zeroOrchs)
	assert.Nil(nilErr)

	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)

	for i := 0; i < 10; i++ {
		orch := NewDBOrch("https://127.0.0.1:"+strconv.Itoa(8936+i), pm.RandAddress().String(), 1, int64(i), int64(5+i), 0)
		orch.PricePerPixel, err = PriceToFixed(big.NewRat(1, int64(5+i)))
		require.Nil(err)
		err = dbh.UpdateOrch(orch)
		require.Nil(err)
		orchList = append(orchList, orch.ServiceURI)
	}

	//URI - MaxPrice - ActivationRound - DeactivationRound
	//127.0.0.1:8936 1/5 0 5
	//127.0.0.1:8937 1/6 1 6
	//127.0.0.1:8938 1/7 2 7
	//127.0.0.1:8939 1/8 3 8
	//127.0.0.1:8940 1/9 4 9
	//127.0.0.1:8941 1/10 5 10
	//127.0.0.1:8942 1/11 6 11
	//127.0.0.1:8943 1/12 7 12
	//127.0.0.1:8944 1/13 8 13
	//127.0.0.1:8945 1/14 9 14

	orchCount, err := dbh.OrchCount(&DBOrchFilter{})
	assert.Nil(err)
	assert.Equal(orchCount, len(orchList))

	// use active filter, should return 5 results
	orchCount, err = dbh.OrchCount(&DBOrchFilter{CurrentRound: big.NewInt(5)})
	assert.Nil(err)
	assert.Equal(5, orchCount)

	// use maxPrice filter, should return 5 results
	orchCount, err = dbh.OrchCount(&DBOrchFilter{MaxPrice: big.NewRat(1, 10)})
	assert.Nil(err)
	assert.Equal(5, orchCount)

	// use maxPrice and active O filter, should return 3 results
	orchCount, err = dbh.OrchCount(&DBOrchFilter{MaxPrice: big.NewRat(1, 8), CurrentRound: big.NewInt(5)})
	assert.Nil(err)
	assert.Equal(3, orchCount)
}

func TestDBFilterOrchs(t *testing.T) {
	assert := assert.New(t)
	var orchList []string
	var orchAddrList []string
	var nilDb *DB
	nilOrchs, nilErr := nilDb.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(1, 1)})
	assert.Nil(nilOrchs)
	assert.Nil(nilErr)

	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	for i := 0; i < 10; i++ {
		orch := NewDBOrch(pm.RandAddress().String(), "https://127.0.0.1:"+strconv.Itoa(8936+i), 1, int64(i), int64(5+i), 0)
		orch.PricePerPixel, err = PriceToFixed(big.NewRat(1, int64(5+i)))
		require.Nil(err)
		err = dbh.UpdateOrch(orch)
		require.Nil(err)
		orchList = append(orchList, orch.ServiceURI)
		orchAddrList = append(orchAddrList, orch.EthereumAddr)
	}

	//URI - MaxPrice - ActivationRound - DeactivationRound
	//127.0.0.1:8936 1/5 0 5
	//127.0.0.1:8937 1/6 1 6
	//127.0.0.1:8938 1/7 2 7
	//127.0.0.1:8939 1/8 3 8
	//127.0.0.1:8940 1/9 4 9
	//127.0.0.1:8941 1/10 5 10
	//127.0.0.1:8942 1/11 6 11
	//127.0.0.1:8943 1/12 7 12
	//127.0.0.1:8944 1/13 8 13
	//127.0.0.1:8945 1/14 9 14

	orchsUpdated, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchsUpdated, 10)

	// Passing in nil max price to filterOrchs returns a query for selectOrchs
	orchsFiltered, err := dbh.SelectOrchs(nil)
	require.Nil(err)
	assert.Len(orchsFiltered, 10)

	// Passing in a higher maxPrice than all orchs to filterOrchs returns all orchs
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(10, 1)})
	require.Nil(err)
	assert.Len(orchsFiltered, 10)

	// Passing in a lower price than all orchs returns no orchs
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(1, 15)})
	require.Nil(err)
	assert.Len(orchsFiltered, 0)

	// Passing in 1/10 returns 5 orchs
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(1, 10)})
	require.Nil(err)
	assert.Len(orchsFiltered, 5)

	// Select only active orchs: activationRound <= currentRound && currentRound < deactivationRound
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{CurrentRound: big.NewInt(5)})
	assert.Nil(err)
	// Should return 5 results, index 1 to 5
	assert.Len(orchsFiltered, 5)
	for _, o := range orchsFiltered {
		assert.Contains(orchList[1:6], o.ServiceURI)
	}

	// Select only active orchs and orchs that pass price filter
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(1, 8), CurrentRound: big.NewInt(5)})
	assert.Nil(err)
	// Should return 3 results, index 3 to 5
	assert.Len(orchsFiltered, 3)
	for _, o := range orchsFiltered {
		assert.Contains(orchList[3:6], o.ServiceURI)
	}

	// Select only active orchs that pass price filter and that are included in Addresses list
	filterAddrs := []ethcommon.Address{ethcommon.HexToAddress(orchAddrList[3]), ethcommon.HexToAddress(orchAddrList[4])}
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{MaxPrice: big.NewRat(1, 8), CurrentRound: big.NewInt(5), Addresses: filterAddrs})
	assert.Nil(err)
	assert.Len(orchsFiltered, 2)
	for _, o := range orchsFiltered {
		assert.Contains(orchList[3:5], o.ServiceURI)
		assert.Contains(orchAddrList[3:5], o.EthereumAddr)
	}

	// Select orchs that are included in Addresses list when list length > 1
	filterAddrs = []ethcommon.Address{ethcommon.HexToAddress(orchAddrList[0]), ethcommon.HexToAddress(orchAddrList[1])}
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{Addresses: filterAddrs})
	assert.Nil(err)
	assert.Len(orchsFiltered, 2)
	for _, o := range orchsFiltered {
		assert.Contains(orchList[0:2], o.ServiceURI)
		assert.Contains(orchAddrList[0:2], o.EthereumAddr)
	}

	// Select orchs that are included in Addresses list when list length = 0
	filterAddrs = []ethcommon.Address{ethcommon.HexToAddress(orchAddrList[1])}
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{Addresses: filterAddrs})
	assert.Nil(err)
	assert.Len(orchsFiltered, 1)
	assert.Equal(orchList[1], orchsFiltered[0].ServiceURI)
	assert.Equal(orchAddrList[1], orchsFiltered[0].EthereumAddr)

	// Empty result when no orchs match Addresses list
	filterAddrs = []ethcommon.Address{ethcommon.BytesToAddress([]byte("foobarbaz"))}
	orchsFiltered, err = dbh.SelectOrchs(&DBOrchFilter{Addresses: filterAddrs})
	assert.Nil(err)
	assert.Len(orchsFiltered, 0)
}

func TestDBUnbondingLocks(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	if err != nil {
		t.Error(err)
		return
	}

	delegator := ethcommon.Address{}

	// Check insertion
	err = dbh.InsertUnbondingLock(big.NewInt(0), delegator, big.NewInt(10), big.NewInt(100))
	if err != nil {
		t.Error(err)
		return
	}
	err = dbh.InsertUnbondingLock(big.NewInt(1), delegator, big.NewInt(10), big.NewInt(100))
	if err != nil {
		t.Error(err)
		return
	}
	err = dbh.InsertUnbondingLock(big.NewInt(2), delegator, big.NewInt(10), big.NewInt(100))
	if err != nil {
		t.Error(err)
		return
	}

	// Check # of unbonding locks
	var numUnbondingLocks int
	row := dbraw.QueryRow("SELECT count(*) FROM unbondingLocks")
	err = row.Scan(&numUnbondingLocks)
	if err != nil {
		t.Error(err)
		return
	}
	if numUnbondingLocks != 3 {
		t.Error("Unexpected number of unbonding locks; expected 3 total, got ", numUnbondingLocks)
		return
	}

	// Check unbonding lock IDs
	unbondingLockIDs, err := dbh.UnbondingLockIDs()
	if err != nil {
		t.Error("Error retrieving unbonding lock IDs ", err)
		return
	}
	if len(unbondingLockIDs) != 3 {
		t.Error("Unexpected number of unbonding lock IDs; expected 3, got ", len(unbondingLockIDs))
		return
	}
	if unbondingLockIDs[0].Cmp(big.NewInt(0)) != 0 {
		t.Error("Unexpected unbonding lock ID; expected 0, got ", unbondingLockIDs[0])
		return
	}

	// Check for failure with duplicate ID
	err = dbh.InsertUnbondingLock(big.NewInt(0), delegator, big.NewInt(10), big.NewInt(100))
	if err == nil {
		t.Error("Expected constraint to fail; duplicate unbonding lock ID and delegator")
		return
	}

	// Check retrieving when all unbonding locks are unused
	unbondingLocks, err := dbh.UnbondingLocks(nil)
	if err != nil {
		t.Error("Error retrieving unbonding locks ", err)
		return
	}
	if len(unbondingLocks) != 3 {
		t.Error("Unexpected number of unbonding locks; expected 3 total, got ", len(unbondingLocks))
		return
	}
	if unbondingLocks[0].ID != 0 {
		t.Error("Unexpected unbonding lock ID; expected 0, got ", unbondingLocks[0].ID)
		return
	}
	if unbondingLocks[0].Delegator != delegator {
		t.Errorf("Unexpected unbonding lock delegator; expected %v, got %v", delegator, unbondingLocks[0].Delegator)
		return
	}
	if unbondingLocks[0].Amount.Cmp(big.NewInt(10)) != 0 {
		t.Errorf("Unexpected unbonding lock amount; expected 10, got %v", unbondingLocks[0].Amount)
		return
	}
	if unbondingLocks[0].WithdrawRound != 100 {
		t.Errorf("Unexpected unbonding lock withdraw round; expected 100, got %v", unbondingLocks[0].WithdrawRound)
		return
	}

	// Check update
	err = dbh.UseUnbondingLock(big.NewInt(0), delegator, big.NewInt(15))
	if err != nil {
		t.Error(err)
		return
	}
	var usedBlock int64
	row = dbraw.QueryRow("SELECT usedBlock FROM unbondingLocks WHERE id = 0 AND delegator = ?", delegator.Hex())
	err = row.Scan(&usedBlock)
	if err != nil {
		t.Error(err)
		return
	}
	if usedBlock != 15 {
		t.Errorf("Unexpected used block; expected 15, got %v", usedBlock)
		return
	}
	err = dbh.UseUnbondingLock(big.NewInt(1), delegator, big.NewInt(16))
	if err != nil {
		t.Error(err)
		return
	}
	row = dbraw.QueryRow("SELECT usedBlock FROM unbondingLocks WHERE id = 1 AND delegator = ?", delegator.Hex())
	err = row.Scan(&usedBlock)
	if err != nil {
		t.Error(err)
		return
	}
	if usedBlock != 16 {
		t.Errorf("Unexpected used block; expected 16; got %v", usedBlock)
		return
	}

	// Check retrieving when some unbonding locks are used
	unbondingLocks, err = dbh.UnbondingLocks(nil)
	if err != nil {
		t.Error("Error retrieving unbonding locks ", err)
		return
	}
	if len(unbondingLocks) != 1 {
		t.Error("Unexpected number of unbonding locks; expected 1 total, got ", len(unbondingLocks))
		return
	}

	err = dbh.InsertUnbondingLock(big.NewInt(3), delegator, big.NewInt(10), big.NewInt(150))
	if err != nil {
		t.Error(err)
		return
	}
	err = dbh.InsertUnbondingLock(big.NewInt(4), delegator, big.NewInt(10), big.NewInt(200))
	if err != nil {
		t.Error(err)
		return
	}

	// Check retrieving withdrawable unbonding locks
	unbondingLocks, err = dbh.UnbondingLocks(big.NewInt(99))
	if err != nil {
		t.Error("Error retrieving unbonding locks ", err)
		return
	}
	if len(unbondingLocks) != 0 {
		t.Error("Unexpected number of withdrawable unbonding locks; expected 0, got ", len(unbondingLocks))
		return
	}
	unbondingLocks, err = dbh.UnbondingLocks(big.NewInt(150))
	if err != nil {
		t.Error("Error retrieving unbonding locks ", err)
		return
	}
	if len(unbondingLocks) != 2 {
		t.Error("Unexpected number of unbonding locks; expected 2, got ", len(unbondingLocks))
		return
	}

	// Check deleting existing lock
	err = dbh.DeleteUnbondingLock(big.NewInt(3), delegator)
	if err != nil {
		t.Error(err)
		return
	}

	unbondingLocks, err = dbh.UnbondingLocks(nil)
	if err != nil {
		t.Error(err)
		return
	}
	if len(unbondingLocks) != 2 {
		t.Error("Unxpected number of unbonding locks after deletion; expected 2, got", len(unbondingLocks))
		return
	}

	// Check setting usedBlock to NULL for existing lock
	err = dbh.UseUnbondingLock(big.NewInt(1), delegator, nil)
	if err != nil {
		t.Error(err)
		return
	}

	unbondingLocks, err = dbh.UnbondingLocks(nil)
	if err != nil {
		t.Error(err)
		return
	}
	if len(unbondingLocks) != 3 {
		t.Error("Unexpected number of unbonding locks after reverting used lock; expected 3, got", len(unbondingLocks))
		return
	}
}

func TestWinningTicketCount(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sender := pm.RandAddress()

	_, ticket, sig, recipientRand := defaultWinningTicket(t)
	ticket.Sender = sender
	count, err := dbh.WinningTicketCount(sender, ticket.CreationRound)
	assert.Nil(err)
	assert.Equal(count, 0)

	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	})
	require.Nil(err)

	_, ticket, sig, recipientRand = defaultWinningTicket(t)
	ticket.Sender = pm.RandAddress()
	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	})
	require.Nil(err)

	count, err = dbh.WinningTicketCount(sender, ticket.CreationRound)
	assert.Nil(err)
	assert.Equal(count, 1)

	// add a submitted ticket , should not change count
	_, ticket, sig, recipientRand = defaultWinningTicket(t)
	ticket.Sender = sender
	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	})
	dbh.MarkWinningTicketRedeemed(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}, pm.RandHash())
	require.Nil(err)

	count, err = dbh.WinningTicketCount(sender, ticket.CreationRound)
	assert.Nil(err)
	assert.Equal(count, 1)

	// all tickets are expirted, should return 0
	count, err = dbh.WinningTicketCount(sender, ticket.CreationRound+100)
	assert.Nil(err)
	assert.Equal(count, 0)
}

func TestInsertWinningTicket_GivenValidInputs_InsertsOneRowCorrectly(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, recipientRand := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	})
	require.Nil(err)

	row := dbraw.QueryRow("SELECT sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, creationRound, creationRoundBlockHash, paramsExpirationBlock FROM ticketQueue")
	var actualSender, actualRecipient, actualRecipientRandHash, actualCreationRoundBlockHash string
	var actualFaceValueBytes, actualWinProbBytes, actualRecipientRandBytes, actualSig []byte
	var actualSenderNonce uint32
	var actualCreationRound int64
	var paramsExpirationBlock int64
	err = row.Scan(&actualSender, &actualRecipient, &actualFaceValueBytes, &actualWinProbBytes, &actualSenderNonce, &actualRecipientRandBytes, &actualRecipientRandHash, &actualSig, &actualCreationRound, &actualCreationRoundBlockHash, &paramsExpirationBlock)

	assert := assert.New(t)
	assert.Equal(ticket.Sender.Hex(), actualSender)
	assert.Equal(ticket.Recipient.Hex(), actualRecipient)
	assert.Equal(ticket.FaceValue, new(big.Int).SetBytes(actualFaceValueBytes))
	assert.Equal(ticket.WinProb, new(big.Int).SetBytes(actualWinProbBytes))
	assert.Equal(ticket.SenderNonce, actualSenderNonce)
	assert.Equal(recipientRand, new(big.Int).SetBytes(actualRecipientRandBytes))
	assert.Equal(ticket.RecipientRandHash, ethcommon.HexToHash(actualRecipientRandHash))
	assert.Equal(ticket.CreationRound, actualCreationRound)
	assert.Equal(ticket.CreationRoundBlockHash, ethcommon.HexToHash(actualCreationRoundBlockHash))
	assert.Equal(ticket.ParamsExpirationBlock.Int64(), paramsExpirationBlock)
	assert.Equal(sig, actualSig)

	ticketsCount := getRowCountOrFatal("SELECT count(*) FROM ticketQueue", dbraw, t)
	assert.Equal(1, ticketsCount)
}

func TestInsertWinningTicket_GivenMaxValueInputs_InsertsOneRowCorrectly(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, recipientRand := defaultWinningTicket(t)
	ticket.FaceValue = MaxUint256OrFatal(t)
	ticket.WinProb = MaxUint256OrFatal(t)
	ticket.SenderNonce = math.MaxUint32

	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	})
	require.Nil(err)

	row := dbraw.QueryRow("SELECT sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, creationRound, creationRoundBlockHash, paramsExpirationBlock FROM ticketQueue")
	var actualSender, actualRecipient, actualRecipientRandHash, actualCreationRoundBlockHash string
	var actualFaceValueBytes, actualWinProbBytes, actualRecipientRandBytes, actualSig []byte
	var actualSenderNonce uint32
	var actualCreationRound int64
	var paramsExpirationBlock int64
	err = row.Scan(&actualSender, &actualRecipient, &actualFaceValueBytes, &actualWinProbBytes, &actualSenderNonce, &actualRecipientRandBytes, &actualRecipientRandHash, &actualSig, &actualCreationRound, &actualCreationRoundBlockHash, &paramsExpirationBlock)

	assert := assert.New(t)
	assert.Equal(ticket.Sender.Hex(), actualSender)
	assert.Equal(ticket.Recipient.Hex(), actualRecipient)
	assert.Equal(ticket.FaceValue, new(big.Int).SetBytes(actualFaceValueBytes))
	assert.Equal(ticket.WinProb, new(big.Int).SetBytes(actualWinProbBytes))
	assert.Equal(ticket.SenderNonce, actualSenderNonce)
	assert.Equal(recipientRand, new(big.Int).SetBytes(actualRecipientRandBytes))
	assert.Equal(ticket.RecipientRandHash, ethcommon.HexToHash(actualRecipientRandHash))
	assert.Equal(ticket.CreationRound, actualCreationRound)
	assert.Equal(ticket.CreationRoundBlockHash, ethcommon.HexToHash(actualCreationRoundBlockHash))
	assert.Equal(ticket.ParamsExpirationBlock.Int64(), paramsExpirationBlock)
	assert.Equal(sig, actualSig)

	ticketsCount := getRowCountOrFatal("SELECT count(*) FROM ticketQueue", dbraw, t)
	assert.Equal(1, ticketsCount)
}

func TestStoreWinningTicket_GivenNilTicket_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	err = dbh.StoreWinningTicket(nil)

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil ticket")

	err = dbh.StoreWinningTicket(&pm.SignedTicket{})
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil ticket")
}

func TestStoreWinningTicket_GivenNilSig_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, _, recipientRand := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		RecipientRand: recipientRand,
	})

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil sig")
}

func TestStoreWinningTicket_GivenNilRecipientRand_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, _ := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(&pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: nil,
	})

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil recipientRand")
}

func TestSelectEarliestWinningTicket(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, recipientRand := defaultWinningTicket(t)
	ticket.Sender = ethcommon.HexToAddress("charizard")

	defaultCreationRound := ticket.CreationRound

	signedTicket0 := &pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}

	_, ticket, sig, recipientRand = defaultWinningTicket(t)
	ticket.Sender = ethcommon.HexToAddress("pikachu")
	signedTicket1 := &pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}
	err = dbh.StoreWinningTicket(signedTicket1)
	require.Nil(err)

	// no tickets found
	earliest, err := dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound)
	assert.Nil(err)
	assert.Nil(earliest)

	err = dbh.StoreWinningTicket(signedTicket0)
	require.Nil(err)
	earliest, err = dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound)
	assert.Nil(err)
	assert.Equal(signedTicket0, earliest)

	// test ticket expired
	earliest, err = dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound+100)
	assert.Nil(err)
	assert.Nil(earliest)

	_, ticket, sig, recipientRand = defaultWinningTicket(t)
	ticket.Sender = ethcommon.HexToAddress("charizard")
	signedTicket2 := &pm.SignedTicket{
		Ticket:        ticket,
		Sig:           pm.RandBytes(32),
		RecipientRand: new(big.Int).SetBytes(pm.RandBytes(32)),
	}
	signedTicket2.CreationRound = ticket.CreationRound + 100

	err = dbh.StoreWinningTicket(signedTicket2)
	require.Nil(err)

	earliest, err = dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound)
	assert.Nil(err)
	assert.Equal(earliest, signedTicket0)

	// Test excluding expired tickets
	earliest, err = dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound+100)
	assert.Nil(err)
	assert.Equal(earliest, signedTicket2)

	// Test excluding submitted tickets
	err = dbh.MarkWinningTicketRedeemed(signedTicket0, pm.RandHash())
	require.Nil(err)
	earliest, err = dbh.SelectEarliestWinningTicket(ethcommon.HexToAddress("charizard"), defaultCreationRound)
	assert.Equal(earliest, signedTicket2)

}

func TestMarkWinningTicketRedeemed_GivenNilTicket_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	err = dbh.MarkWinningTicketRedeemed(nil, ethcommon.Hash{})

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil ticket")

	err = dbh.MarkWinningTicketRedeemed(&pm.SignedTicket{}, ethcommon.Hash{})
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil ticket")
}

func TestMarkWinningTicketRedeemed_GivenNilSig_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, _, recipientRand := defaultWinningTicket(t)

	err = dbh.MarkWinningTicketRedeemed(&pm.SignedTicket{
		Ticket:        ticket,
		RecipientRand: recipientRand,
	}, ethcommon.Hash{})

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil sig")
}

func TestMarkWinningTicketRedeemed_Update_TxHash_And_SubmittedAt(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, recipientRand := defaultWinningTicket(t)

	txHash := pm.RandHash()
	signedT := &pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}
	// test no record found
	err = dbh.MarkWinningTicketRedeemed(signedT, txHash)
	assert.EqualError(err, fmt.Sprintf("no record found for sig=0x%x", sig))

	// store a record
	err = dbh.StoreWinningTicket(signedT)
	require.Nil(err)

	// success
	err = dbh.MarkWinningTicketRedeemed(signedT, txHash)
	assert.Nil(err)
	row := dbraw.QueryRow("SELECT txHash, redeemedAt FROM ticketQueue WHERE sig=?", sig)
	var txHashActual string
	var redeemedAt time.Time
	err = row.Scan(&txHashActual, &redeemedAt)
	require.Nil(err)
	assert.Equal(txHash.Hex(), txHashActual)
	assert.InDelta(redeemedAt.Day(), time.Now().Day(), 1)
}

func TestRemoveWinningTicket(t *testing.T) {
	assert := assert.New(t)
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	_, ticket, sig, recipientRand := defaultWinningTicket(t)

	signedTicket := &pm.SignedTicket{
		Ticket:        ticket,
		Sig:           sig,
		RecipientRand: recipientRand,
	}

	err = dbh.StoreWinningTicket(signedTicket)
	require.Nil(err)

	// confirm ticket is added correctly
	count, err := dbh.WinningTicketCount(ticket.Sender, ticket.CreationRound)
	require.Nil(err)
	require.Equal(count, 1)

	// removing the wrong ticket should return nil
	signedTicketDup := *signedTicket
	signedTicketDup.Sig = pm.RandBytes(32)
	err = dbh.RemoveWinningTicket(&signedTicketDup)
	assert.NoError(err)
	// confirm ticket is not removed
	count, err = dbh.WinningTicketCount(signedTicket.Sender, ticket.CreationRound)
	require.Nil(err)
	assert.Equal(count, 1)

	err = dbh.RemoveWinningTicket(signedTicket)
	assert.Nil(err)
	// confirm ticket is removed correctly
	count, _ = dbh.WinningTicketCount(ticket.Sender, ticket.CreationRound)
	require.Equal(count, 0)
}

func TestInsertMiniHeader_ReturnsFindLatestMiniHeader(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	assert := assert.New(t)
	require := require.New(t)
	require.Nil(err)

	h0 := defaultMiniHeader()
	err = dbh.InsertMiniHeader(h0)
	require.Nil(err)

	h0db, err := dbh.FindLatestMiniHeader()
	require.Nil(err)
	assert.Equal(h0, h0db)

	// Test FindLatestMiniHeader with 2 blocks
	h1 := defaultMiniHeader()
	h1.Number = big.NewInt(451)
	h1.Logs = append(h1.Logs, types.Log{
		Topics:    []common.Hash{pm.RandHash(), pm.RandHash()},
		Data:      pm.RandBytes(32),
		BlockHash: h1.Hash,
	})
	err = dbh.InsertMiniHeader(h1)
	require.Nil(err)
	earliest, err := dbh.FindLatestMiniHeader()
	require.Nil(err)
	assert.Equal(h1, earliest)
	assert.Equal(len(h1.Logs), 2)

	// test MiniHeader = nil error
	err = dbh.InsertMiniHeader(nil)
	assert.EqualError(err, "must provide a MiniHeader")

	// test blocknumber = nil error
	h1.Number = nil
	err = dbh.InsertMiniHeader(h1)
	assert.EqualError(err, "no block number found")
}

func TestFindAllMiniHeadersSortedByNumber(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	assert := assert.New(t)
	require := require.New(t)
	require.Nil(err)

	added := make([]*blockwatch.MiniHeader, 10)
	for i := 0; i < 10; i++ {
		h := defaultMiniHeader()
		h.Number = big.NewInt(int64(i))
		if i%2 == 0 {
			h.Logs = append(h.Logs, types.Log{
				Topics:    []common.Hash{pm.RandHash(), pm.RandHash()},
				Data:      pm.RandBytes(32),
				BlockHash: h.Hash,
			})
		}
		err = dbh.InsertMiniHeader(h)
		require.Nil(err)
		added[9-i] = h
	}

	headers, err := dbh.FindAllMiniHeadersSortedByNumber()
	for i, h := range headers {
		assert.Equal(h.Number.Int64(), int64(len(headers)-1-i))
		assert.Equal(h, added[i])
		// even = 1 log , uneven = 2 logs
		if i%2 == 0 {
			assert.Len(h.Logs, 1)
		} else {
			assert.Len(h.Logs, 2)
		}
	}
}

func TestDeleteMiniHeader(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	assert := assert.New(t)
	require := require.New(t)
	require.Nil(err)

	h0 := defaultMiniHeader()
	err = dbh.InsertMiniHeader(h0)
	require.Nil(err)

	h0db, err := dbh.FindLatestMiniHeader()
	require.Nil(err)
	assert.Equal(h0, h0db)

	err = dbh.DeleteMiniHeader(h0.Hash)
	require.Nil(err)
	headers, err := dbh.FindAllMiniHeadersSortedByNumber()
	require.Nil(err)
	assert.Equal(len(headers), 0)

	// test FindLatestMiniHeader error path
	h0db, err = dbh.FindLatestMiniHeader()
	assert.Nil(err)

	// Test header to be deleted doesn't exist
	err = dbh.DeleteMiniHeader(h0.Hash)
	assert.Nil(err)
	headers, _ = dbh.FindAllMiniHeadersSortedByNumber()
	assert.Equal(len(headers), 0)

	// test correct amount of remaining headers when more than 1
	err = dbh.InsertMiniHeader(h0)
	require.Nil(err)
	h1 := defaultMiniHeader()
	err = dbh.InsertMiniHeader(h1)
	require.Nil(err)
	err = dbh.DeleteMiniHeader(h0.Hash)
	require.Nil(err)
	headers, err = dbh.FindAllMiniHeadersSortedByNumber()
	assert.Equal(len(headers), 1)
	assert.Nil(err)
	assert.Equal(headers[0].Hash, h1.Hash)
}

func defaultWinningTicket(t *testing.T) (sessionID string, ticket *pm.Ticket, sig []byte, recipientRand *big.Int) {
	sessionID = "foo bar"
	ticket = &pm.Ticket{
		Sender:                pm.RandAddress(),
		Recipient:             pm.RandAddress(),
		FaceValue:             big.NewInt(1234),
		WinProb:               big.NewInt(2345),
		SenderNonce:           uint32(123),
		RecipientRandHash:     pm.RandHash(),
		ParamsExpirationBlock: big.NewInt(1337),
	}
	sig = pm.RandBytes(42)
	recipientRand = big.NewInt(4567)
	return
}

func getRowCountOrFatal(query string, dbraw *sql.DB, t *testing.T) int {
	var count int
	row := dbraw.QueryRow(query)
	err := row.Scan(&count)
	require.Nil(t, err)

	return count
}

func defaultMiniHeader() *blockwatch.MiniHeader {
	block := &blockwatch.MiniHeader{
		Number: big.NewInt(450),
		Parent: pm.RandHash(),
		Hash:   pm.RandHash(),
	}
	log := types.Log{
		Topics:    []common.Hash{pm.RandHash(), pm.RandHash()},
		Data:      pm.RandBytes(32),
		BlockHash: block.Hash,
	}
	block.Logs = []types.Log{log}
	return block
}
