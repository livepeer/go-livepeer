package common

import (
	"database/sql"
	"fmt"
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/lpms/ffmpeg"
	_ "github.com/mattn/go-sqlite3"
)

func dbPath(t *testing.T) string {
	return fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=1", t.Name())
}

func tempDB(t *testing.T) (*DB, *sql.DB, error) {
	dbpath := dbPath(t)
	dbh, err := InitDB(dbpath)
	if err != nil {
		t.Error("Unable to initialize DB ", err)
		return nil, nil, err
	}
	raw, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		t.Error("Unable to open raw sqlite db ", err)
		return nil, nil, err
	}
	return dbh, raw, nil
}

func TestDBLastSeenBlock(t *testing.T) {
	dbh, dbraw, err := tempDB(t)
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
}

func TestDBVersion(t *testing.T) {
	dbh, dbraw, err := tempDB(t)
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

func NewStubJob() *DBJob {
	return &DBJob{
		ID: 0, streamID: "1", price: 0, profiles: []ffmpeg.VideoProfile{},
		broadcaster: ethcommon.Address{}, Transcoder: ethcommon.Address{},
		startBlock: 1, endBlock: 2,
	}
}

func TestDBJobs(t *testing.T) {
	dbh, dbraw, err := tempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	j := NewStubJob()
	dbh.InsertJob(j)
	j.ID = 1
	dbh.InsertJob(j)
	endBlock := j.endBlock
	j.ID = 2
	j.endBlock += 5
	dbh.InsertJob(j)
	jobs, err := dbh.ActiveJobs(big.NewInt(0))
	if err != nil || len(jobs) != 3 {
		t.Error("Unexpected error in active jobs ", err, len(jobs))
		return
	}
	jobs, err = dbh.ActiveJobs(big.NewInt(endBlock))
	if err != nil || len(jobs) != 1 || jobs[0].ID != 2 {
		t.Error("Unexpected error in active jobs ", err, len(jobs))
		return
	}
	// check stop reason filter
	dbh.SetStopReason(big.NewInt(j.ID), "insufficient lolz")
	jobs, err = dbh.ActiveJobs(big.NewInt(0))
	if err != nil || len(jobs) != 2 {
		t.Error("Unexpected error in active jobs ", err, len(jobs))
	}
}

func TestDBReceipts(t *testing.T) {
	dbh, dbraw, err := tempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	jid := big.NewInt(0)
	ir := func(j *big.Int, seq int) error {
		b := []byte("")
		n := time.Now()
		return dbh.InsertReceipt(j, int64(seq), "", b, b, b, n, n)
	}
	err = ir(jid, 1)
	if err == nil {
		t.Error("Expected foreign key constraint to fail; nonexistent job")
		return
	}
	job := NewStubJob()
	dbh.InsertJob(job)
	err = ir(jid, 1)
	if err != nil {
		t.Error(err)
		return
	}
	err = ir(jid, 1)
	if err == nil {
		t.Error("Expected constraint to fail; duplicate seq id")
		return
	}
	err = ir(jid, 2)
	if err != nil {
		t.Error(err)
		return
	}
	// insert receipt for a different job, but same seqid
	job = NewStubJob()
	job.ID = 1
	dbh.InsertJob(job)
	err = ir(big.NewInt(job.ID), 1)
	if err != nil {
		t.Error(err)
		return
	}
	// check unclaimed receipts
	receipts, err := dbh.UnclaimedReceipts()
	if err != nil {
		t.Error(err)
		return
	}
	if len(receipts) != 2 {
		t.Error("Unxpected number of jobs in receipts")
		return
	}
	if len(receipts[0]) != 2 || len(receipts[1]) != 1 {
		t.Error("Unexpected number of receipts for job")
		return
	}

	// Receipt checking functions.

	// Ensure that the receipt we just inserted exists
	exists, err := dbh.ReceiptExists(big.NewInt(job.ID), 1)
	if err != nil {
		t.Error(err)
		return
	}
	if !exists {
		t.Error("Expected segment to exist in DB")
		return
	}
	// Ensure a nonexistent receipt does not exist: valid job, invalid seg
	exists, err = dbh.ReceiptExists(big.NewInt(job.ID), 10)
	if err != nil {
		t.Error(err)
		return
	}
	if exists {
		t.Error("Did not expect a segment to exist")
		return
	}
	// Ensure a nonexistent receipt does not exist: invalid job, seg exists elsewhere
	exists, err = dbh.ReceiptExists(big.NewInt(10), 1)
	if err != nil {
		t.Error(err)
		return
	}
	if exists {
		t.Error("Did not expect a segment to exist")
		return
	}
}

func TestDBClaims(t *testing.T) {
	dbh, dbraw, err := tempDB(t)
	defer dbh.Close()
	defer dbraw.Close()

	ir := func(j int64, seq int) error {
		b := []byte("")
		n := time.Now()
		return dbh.InsertReceipt(big.NewInt(j), int64(seq), "", b, b, b, n, n)
	}

	job := NewStubJob()
	dbh.InsertJob(job)
	ir(job.ID, 0)
	ir(job.ID, 1)
	ir(job.ID, 4)
	ir(job.ID, 5)
	job.ID++
	dbh.InsertJob(job)
	ir(job.ID, 1)
	ir(job.ID, 2)

	cidp, err := dbh.InsertClaim(big.NewInt(0), [2]int64{0, 1}, [32]byte{})
	if err != nil || cidp == nil {
		t.Errorf("Error inserting claim %v %v", err, cidp)
		return
	}
	cidp, err = dbh.InsertClaim(big.NewInt(0), [2]int64{4, 5}, [32]byte{})
	if err != nil || cidp == nil {
		t.Errorf("Error inserting claim %v %v", err, cidp)
		return
	}
	cidp, err = dbh.InsertClaim(big.NewInt(1), [2]int64{1, 2}, [32]byte{})
	if err != nil || cidp == nil {
		t.Errorf("Error inserting claim %v %v", err, cidp)
		return
	}
	// check job 0 claim 0
	var nbreceipts int
	row := dbraw.QueryRow("SELECT count(*) FROM receipts WHERE claimID = 0 AND jobID = 0")
	err = row.Scan(&nbreceipts)
	if err != nil {
		t.Error(err)
		return
	}
	if nbreceipts != 2 {
		t.Error("Mismatched receipts for claim: expected 2 got %d", nbreceipts)
		return
	}
	// check job 0 claim 1
	row = dbraw.QueryRow("SELECT count(*) FROM receipts WHERE claimID = 1 AND jobID = 0")
	err = row.Scan(&nbreceipts)
	if err != nil {
		t.Error(err)
		return
	}
	if nbreceipts != 2 {
		t.Error("Mismatched receipts for claim: expected 2 got %d", nbreceipts)
		return
	}
	// check job 1 claim 0
	row = dbraw.QueryRow("SELECT count(*) FROM receipts WHERE claimID = 0 AND jobID = 1")
	err = row.Scan(&nbreceipts)
	if err != nil {
		t.Error(err)
		return
	}
	if nbreceipts != 2 {
		t.Error("Mismatched receipts for claim: expected 2 got %d", nbreceipts)
		return
	}
	// Sanity check number of claims
	var nbclaims int
	row = dbraw.QueryRow("SELECT count(*) FROM claims")
	err = row.Scan(&nbclaims)
	if err != nil {
		t.Error(err)
		return
	}
	if nbclaims != 3 {
		t.Error("Unexpected number of claims; expected 3 total, got ", nbclaims)
		return
	}
	// check claim status
	var status string
	q := "SELECT status FROM claims WHERE jobID = 0 AND id = 1"
	s := "over the moon"
	row = dbraw.QueryRow(q)
	err = row.Scan(&status)
	if err != nil {
		t.Error(err)
		return
	}
	// sanity check value
	if status == s {
		t.Error("Expected some status value other than ", status)
		return
	}
	err = dbh.SetClaimStatus(big.NewInt(0), 1, s)
	if err != nil {
		t.Error(err)
		return
	}
	row = dbraw.QueryRow(q)
	err = row.Scan(&status)
	if err != nil || status != s {
		t.Errorf("Unexpected: error %v, got %v but wanted %v", err, status, s)
		return
	}
}
