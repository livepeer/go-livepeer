package common

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBLastSeenBlock(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	if err != nil {
		return
	}
	defer dbh.Close()
	defer dbraw.Close()

	// sanity check default value
	var val int64
	var created_at string
	var updated_at string
	stmt := "SELECT value, updatedAt FROM kv WHERE key = 'lastBlock'"
	row := dbraw.QueryRow(stmt)
	err = row.Scan(&val, &created_at)
	if err != nil || val != int64(0) {
		t.Errorf("Unexpected result from sanity check; got %v - %v", err, val)
		return
	}
	// set last updated at timestamp to sometime in the past
	update_stmt := "UPDATE kv SET updatedAt = datetime('now', '-2 months') WHERE key = 'lastBlock'"
	_, err = dbraw.Exec(update_stmt) // really should sanity check this result
	if err != nil {
		t.Error("Could not update db ", err)
	}

	// now test set
	blkval := int64(54321)
	err = dbh.SetLastSeenBlock(big.NewInt(blkval))
	if err != nil {
		t.Error("Unable to set last seen block ", err)
		return
	}
	row = dbraw.QueryRow(stmt)
	err = row.Scan(&val, &updated_at)
	if err != nil || val != blkval {
		t.Errorf("Unexpected result from value check; got %v - %v", err, val)
		return
	}
	// small possibility of a test failure if we executed over a 1s boundary
	if updated_at != created_at {
		t.Errorf("Unexpected result from update check; got %v:%v", updated_at, created_at)
		return
	}

	// test getter function
	blk, err := dbh.LastSeenBlock()
	if err != nil {
		t.Error(err)
		return
	}
	if blk.Int64() != blkval {
		t.Errorf("Unexpected result from getter; expected %v, got %v", blkval, blk.Int64())
		return
	}
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
}

func TestInsertWinningTicket_GivenValidInputs_InsertsOneRowCorrectly(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sessionID, ticket, sig, recipientRand := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(sessionID, ticket, sig, recipientRand)
	require.Nil(err)

	row := dbraw.QueryRow("SELECT sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, sessionID FROM winningTickets")
	var actualSender, actualRecipient, actualRecipientRandHash, actualSessionID string
	var actualFaceValueBytes, actualWinProbBytes, actualRecipientRandBytes, actualSig []byte
	var actualSenderNonce uint32
	err = row.Scan(&actualSender, &actualRecipient, &actualFaceValueBytes, &actualWinProbBytes, &actualSenderNonce, &actualRecipientRandBytes, &actualRecipientRandHash, &actualSig, &actualSessionID)

	assert := assert.New(t)
	assert.Equal(ticket.Sender.Hex(), actualSender)
	assert.Equal(ticket.Recipient.Hex(), actualRecipient)
	assert.Equal(ticket.FaceValue, new(big.Int).SetBytes(actualFaceValueBytes))
	assert.Equal(ticket.WinProb, new(big.Int).SetBytes(actualWinProbBytes))
	assert.Equal(ticket.SenderNonce, actualSenderNonce)
	assert.Equal(recipientRand, new(big.Int).SetBytes(actualRecipientRandBytes))
	assert.Equal(ticket.RecipientRandHash, ethcommon.HexToHash(actualRecipientRandHash))
	assert.Equal(sig, actualSig)
	assert.Equal(sessionID, actualSessionID)

	ticketsCount := getRowCountOrFatal("SELECT count(*) FROM winningTickets", dbraw, t)
	assert.Equal(1, ticketsCount)
}

func TestInsertWinningTicket_GivenMaxValueInputs_InsertsOneRowCorrectly(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sessionID, ticket, sig, recipientRand := defaultWinningTicket(t)
	ticket.FaceValue = maxUint256OrFatal(t)
	ticket.WinProb = maxUint256OrFatal(t)
	ticket.SenderNonce = math.MaxUint32

	err = dbh.StoreWinningTicket(sessionID, ticket, sig, recipientRand)
	require.Nil(err)

	row := dbraw.QueryRow("SELECT sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, sessionID FROM winningTickets")
	var actualSender, actualRecipient, actualRecipientRandHash, actualSessionID string
	var actualFaceValueBytes, actualWinProbBytes, actualRecipientRandBytes, actualSig []byte
	var actualSenderNonce uint32
	err = row.Scan(&actualSender, &actualRecipient, &actualFaceValueBytes, &actualWinProbBytes, &actualSenderNonce, &actualRecipientRandBytes, &actualRecipientRandHash, &actualSig, &actualSessionID)

	assert := assert.New(t)
	assert.Equal(ticket.Sender.Hex(), actualSender)
	assert.Equal(ticket.Recipient.Hex(), actualRecipient)
	assert.Equal(ticket.FaceValue, new(big.Int).SetBytes(actualFaceValueBytes))
	assert.Equal(ticket.WinProb, new(big.Int).SetBytes(actualWinProbBytes))
	assert.Equal(ticket.SenderNonce, actualSenderNonce)
	assert.Equal(recipientRand, new(big.Int).SetBytes(actualRecipientRandBytes))
	assert.Equal(ticket.RecipientRandHash, ethcommon.HexToHash(actualRecipientRandHash))
	assert.Equal(sig, actualSig)
	assert.Equal(sessionID, actualSessionID)

	ticketsCount := getRowCountOrFatal("SELECT count(*) FROM winningTickets", dbraw, t)
	assert.Equal(1, ticketsCount)
}

func TestStoreWinningTicket_GivenNilTicket_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sig := pm.RandBytesOrFatal(42, t)
	recipientRand := new(big.Int).SetInt64(1234)

	err = dbh.StoreWinningTicket("sessionID", nil, sig, recipientRand)

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil ticket")
}

func TestStoreWinningTicket_GivenNilSig_ReturnsError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sessionID, ticket, _, recipientRand := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(sessionID, ticket, nil, recipientRand)

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

	sessionID, ticket, sig, _ := defaultWinningTicket(t)

	err = dbh.StoreWinningTicket(sessionID, ticket, sig, nil)

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Contains(err.Error(), "nil recipientRand")
}

func TestLoadWinningTicket_GivenStoredTicket_LoadsItCorrectly(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	sessionID, ticket, sig, recipientRand := defaultWinningTicket(t)
	err = dbh.StoreWinningTicket(sessionID, ticket, sig, recipientRand)
	require.Nil(err)

	tickets, sigs, recipientRands, err := dbh.LoadWinningTickets(sessionID)
	require.Nil(err)

	assert := assert.New(t)
	assert.Len(tickets, 1)
	assert.Len(sigs, 1)
	assert.Len(recipientRands, 1)
	actualTicket := tickets[0]
	actualSig := sigs[0]
	actualRecipientRand := recipientRands[0]
	assert.Equal(ticket, actualTicket)
	assert.Equal(sig, actualSig)
	assert.Equal(recipientRand, actualRecipientRand)
}

func TestLoadWinningTicket_GivenStoredTicketsFromDifferentSessions_OnlyLoadsFromSpecificSessionID(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	// Two tickets in the first session
	firstSessionID := "first session"
	_, ticket0, sig0, recipientRand0 := defaultWinningTicket(t)
	err = dbh.StoreWinningTicket(firstSessionID, ticket0, sig0, recipientRand0)
	require.Nil(err)

	_, ticket1, sig1, recipientRand1 := defaultWinningTicket(t)
	err = dbh.StoreWinningTicket(firstSessionID, ticket1, sig1, recipientRand1)
	require.Nil(err)

	// One ticket in the second session
	secondSessionID := "second session"
	_, ticket2, sig2, recipientRand2 := defaultWinningTicket(t)
	err = dbh.StoreWinningTicket(secondSessionID, ticket2, sig2, recipientRand2)
	require.Nil(err)

	tickets, sigs, recipientRands, err := dbh.LoadWinningTickets(firstSessionID)

	assert := assert.New(t)
	assert.Nil(err)
	assert.Len(tickets, 2)
	assert.Len(sigs, 2)
	assert.Len(recipientRands, 2)
	assert.Equal(ticket0, tickets[0])
	assert.Equal(sig0, sigs[0])
	assert.Equal(recipientRand0, recipientRands[0])
	assert.Equal(ticket1, tickets[1])
	assert.Equal(sig1, sigs[1])
	assert.Equal(recipientRand1, recipientRands[1])
}

func TestLoadWinningTicket_GivenNonexistentSessionID_ReturnsEmptySlicesNoError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	tickets, sigs, recipientRands, err := dbh.LoadWinningTickets("some sessionID")

	assert := assert.New(t)
	assert.Nil(err)
	assert.Len(tickets, 0)
	assert.Len(sigs, 0)
	assert.Len(recipientRands, 0)
}

func TestLoadWinningTicket_GivenEmptySessionID_ReturnsEmptySlicesNoError(t *testing.T) {
	dbh, dbraw, err := TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require := require.New(t)
	require.Nil(err)

	tickets, sigs, recipientRands, err := dbh.LoadWinningTickets("")

	assert := assert.New(t)
	assert.Nil(err)
	assert.Len(tickets, 0)
	assert.Len(sigs, 0)
	assert.Len(recipientRands, 0)
}

func defaultWinningTicket(t *testing.T) (sessionID string, ticket *pm.Ticket, sig []byte, recipientRand *big.Int) {
	sessionID = "foo bar"
	ticket = &pm.Ticket{
		Sender:            pm.RandAddressOrFatal(t),
		Recipient:         pm.RandAddressOrFatal(t),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(2345),
		SenderNonce:       uint32(123),
		RecipientRandHash: pm.RandHashOrFatal(t),
	}
	sig = pm.RandBytesOrFatal(42, t)
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

func maxUint256OrFatal(t *testing.T) *big.Int {
	n, ok := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", 16)
	if !ok {
		t.Fatalf("unexpected error creating max value of uint256")
	}
	return n
}
