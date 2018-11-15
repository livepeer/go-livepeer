package core

import (
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/livepeer/go-livepeer/common"
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
