package core

import (
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestCurrentBlock(t *testing.T) {
	tmpdir, _ := ioutil.TempDir("", "")
	n, err := NewLivepeerNode(nil, tmpdir, nil)
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tmpdir)
	orch := NewOrchestrator(n)

	// test empty db
	if orch.CurrentBlock() != nil {
		t.Error("Expected nil block")
	}

	db, dbraw, err := common.TempDB(t)
	if err != nil {
		t.Error("Error creating db ", err)
	}
	defer db.Close()
	defer dbraw.Close()
	n.Database = db

	db.SetLastSeenBlock(big.NewInt(1234))
	if orch.CurrentBlock().Int64() != big.NewInt(1234).Int64() {
		t.Error("Unexpected block ", orch.CurrentBlock())
	}

	// throw a db error; should still return nil
	if _, err := dbraw.Exec("DELETE FROM kv WHERE key = 'lastBlock'"); err != nil {
		t.Error("Unexpected error deleting lastBlock ", err)
	}

	if orch.CurrentBlock() != nil {
		t.Error("Expected nil getting nonexistent row")
	}

}

func TestGetJob(t *testing.T) {
	tmpdir, _ := ioutil.TempDir("", "")
	n, err := NewLivepeerNode(nil, tmpdir, nil)
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tmpdir)
	orch := NewOrchestrator(n)

	// test without a db
	_, err = orch.GetJob(1)
	if err == nil || err.Error() != "Cannot get job; missing database" {
		t.Error("Unexpected or missing error ", err)
	}
	db, dbraw, err := common.TempDB(t)
	if err != nil {
		t.Error("Error creating db ", err)
	}
	defer db.Close()
	defer dbraw.Close()
	n.Database = db

	sj := StubJob(n)
	dbjob := eth.EthJobToDBJob(sj)
	db.InsertJob(dbjob)
	db.InsertClaim(sj.JobId, [2]int64{1, 10}, [32]byte{})

	job, err := orch.GetJob(dbjob.ID)
	if err != nil || job == nil {
		t.Error("Error getting job ", err, job)
	}
	// sanity check
	if job.JobId.Int64() != sj.JobId.Int64() || job.StreamId != sj.StreamId {
		t.Error("Job fetch sanity check fail")
	}
	if job.TotalClaims.Int64() != 1 || !job.FirstClaimSubmitted {
		t.Error("Job claim counts incorrect")
	}

	db.SetStopReason(sj.JobId, "some reason")
	job, err = orch.GetJob(dbjob.ID)
	if err == nil || err.Error() != "Job stopped" {
		t.Error("Expected to fail on a stopped job ", err)
	}

	// nonexistent job in db; fall back to eth client
	_, err = orch.GetJob(10)
	if err == nil || err.Error() != "Cannot get job; missing Eth client" {
		t.Error("Unexpected or missing error ", err)
	}

	seth := &eth.StubClient{JobsMap: make(map[string]*lpTypes.Job)}
	n.Eth = seth
	sj.JobId = big.NewInt(10)
	seth.JobsMap["10"] = sj
	seth.TranscoderAddress = ethcommon.BytesToAddress([]byte("111 Transcoder Address 1"))

	// sanity check empty transcoder addr in job
	if (sj.TranscoderAddress != ethcommon.Address{}) {
		t.Error("Unexpected transcoder address ", sj.TranscoderAddress)
	}

	// get valid job; check for transcoder assignment?
	j, err := orch.GetJob(10)
	if err != nil || j.JobId.Int64() != sj.JobId.Int64() {
		t.Error("Unexpected error or job id ", err, j)
	}
	// check that the address was assigned appropriately based on seth
	if j.TranscoderAddress != seth.TranscoderAddress {
		t.Error("Unexpected transcoder address ")
	}

	// reset transcoder assignment; check the call preserves it
	sj.TranscoderAddress = ethcommon.BytesToAddress([]byte("222 Transcoder Address 2"))
	j, err = orch.GetJob(10)
	if err != nil || j.TranscoderAddress != sj.TranscoderAddress {
		t.Error("Got error or unexpected transcoder address ", err, j.TranscoderAddress)
	}
}
