package common

import (
	"database/sql"
	"fmt"
	"math/big"
	"testing"
)

func dbPath(t *testing.T) string {
	return fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=1", t.Name())
}

func TempDB(t *testing.T) (*DB, *sql.DB, error) {
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

func MaxUint256OrFatal(t *testing.T) *big.Int {
	n, ok := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", 16)
	if !ok {
		t.Fatalf("unexpected error creating max value of uint256")
	}
	return n
}
