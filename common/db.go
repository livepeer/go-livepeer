package common

import (
	"bytes"
	"database/sql"
	"errors"
	"math/big"
	"text/template"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	dbh *sql.DB

	// prepared statements
	updateKV                   *sql.Stmt
	insertUnbondingLock        *sql.Stmt
	useUnbondingLock           *sql.Stmt
	unbondingLocks             *sql.Stmt
	withdrawableUnbondingLocks *sql.Stmt
}

type DBUnbondingLock struct {
	ID            int64
	Delegator     ethcommon.Address
	Amount        *big.Int
	WithdrawRound int64
}

var LivepeerDBVersion = 1

var ErrDBTooNew = errors.New("DB Too New")

var schema = `
	CREATE TABLE IF NOT EXISTS kv (
		key STRING PRIMARY KEY,
		value STRING,
		updatedAt STRING DEFAULT CURRENT_TIMESTAMP
	);
	INSERT OR IGNORE INTO kv(key, value) VALUES('dbVersion', '{{ . }}');
	INSERT OR IGNORE INTO kv(key, value) VALUES('lastBlock', '0');

	CREATE TABLE IF NOT EXISTS unbondingLocks (
		id INTEGER NOT NULL,
		delegator STRING,
		amount TEXT,
		withdrawRound int64,
		usedBlock int64,
		PRIMARY KEY(id, delegator)
	);
	-- Index to only retrieve unbonding locks that have not been used
	CREATE INDEX IF NOT EXISTS idx_unbondinglocks_usedblock ON unbondingLocks(usedBlock);
`

func InitDB(dbPath string) (*DB, error) {
	// XXX need a way to ensure (via unit tests?) that all DB{} fields are
	// properly closed / cleaned up in the case of an error
	d := DB{}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		glog.Error("Unable to open DB ", dbPath, err)
		return nil, err
	}
	// The DB connection might be used in multiple goroutines (i.e. when recovering claims during node restart)
	// resulting in concurrent access. SQLite can only handle one writer at a time, so if concurrent writes occur
	// we can encounter a `database is locked` error. To avoid concurrent writes, we limit SQLite to a single connection
	db.SetMaxOpenConns(1)
	d.dbh = db
	schemaBuf := new(bytes.Buffer)
	tmpl := template.Must(template.New("schema").Parse(schema))
	tmpl.Execute(schemaBuf, LivepeerDBVersion)
	_, err = db.Exec(schemaBuf.String())
	if err != nil {
		glog.Error("Error initializing schema ", err)
		d.Close()
		return nil, err
	}

	// Check for correct DB version and upgrade if needed
	var dbVersion int
	row := db.QueryRow("SELECT value FROM kv WHERE key = 'dbVersion'")
	err = row.Scan(&dbVersion)
	if err != nil {
		glog.Error("Unable to fetch DB version ", err)
		d.Close()
		return nil, err
	}
	if dbVersion > LivepeerDBVersion {
		glog.Error("Database too new")
		d.Close()
		return nil, ErrDBTooNew
	} else if dbVersion < LivepeerDBVersion {
		// Upgrade stepwise up to the correct version using the migration
		// procedure for each version
	} else if dbVersion == LivepeerDBVersion {
		// all good; nothing to do
	}

	// updateKV prepared statement
	stmt, err := db.Prepare("UPDATE kv SET value=?, updatedAt = datetime() WHERE key=?")
	if err != nil {
		glog.Error("Unable to prepare updatekv stmt ", err)
		d.Close()
		return nil, err
	}
	d.updateKV = stmt

	// Unbonding locks prepared statements
	stmt, err = db.Prepare("INSERT INTO unbondingLocks(id, delegator, amount, withdrawRound) VALUES(?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insertUnbondingLock ", err)
		d.Close()
		return nil, err
	}
	d.insertUnbondingLock = stmt
	stmt, err = db.Prepare("UPDATE unbondingLocks SET usedBlock=? WHERE id=? AND delegator=?")
	if err != nil {
		glog.Error("Unable to prepare useUnbondingLock ", err)
		d.Close()
		return nil, err
	}
	d.useUnbondingLock = stmt
	stmt, err = db.Prepare("SELECT id, delegator, amount, withdrawRound FROM unbondingLocks WHERE usedBlock IS NULL")
	if err != nil {
		glog.Error("Unable to prepare unbondingLocks ", err)
		d.Close()
		return nil, err
	}
	d.unbondingLocks = stmt
	stmt, err = db.Prepare("SELECT id, delegator, amount, withdrawRound FROM unbondingLocks WHERE usedBlock IS NULL AND withdrawRound <= ?")
	if err != nil {
		glog.Error("Unable to prepare withdrawableUnbondingLocks ", err)
		d.Close()
		return nil, err
	}
	d.withdrawableUnbondingLocks = stmt

	glog.V(DEBUG).Info("Initialized DB node")
	return &d, nil
}

func (db *DB) Close() {
	glog.V(DEBUG).Info("Closing DB")
	if db.updateKV != nil {
		db.updateKV.Close()
	}
	if db.insertUnbondingLock != nil {
		db.insertUnbondingLock.Close()
	}
	if db.useUnbondingLock != nil {
		db.useUnbondingLock.Close()
	}
	if db.unbondingLocks != nil {
		db.unbondingLocks.Close()
	}
	if db.withdrawableUnbondingLocks != nil {
		db.withdrawableUnbondingLocks.Close()
	}
	if db.dbh != nil {
		db.dbh.Close()
	}
}

func (db *DB) SetLastSeenBlock(block *big.Int) error {
	if db == nil {
		return nil
	}
	glog.V(DEBUG).Info("db: Setting LastSeenBlock to ", block)
	_, err := db.updateKV.Exec(block.String(), "lastBlock")
	if err != nil {
		glog.Error("db: Got err in updating block ", err)
		return err
	}
	return err
}

func (db *DB) LastSeenBlock() (*big.Int, error) {
	if db == nil {
		return nil, nil
	}

	var lastSeenBlock int64
	row := db.dbh.QueryRow("SELECT value FROM kv WHERE key = 'lastBlock'")
	err := row.Scan(&lastSeenBlock)
	if err != nil {
		glog.Error("db: Got err in retrieving block ", err)
		return nil, err
	}

	return big.NewInt(lastSeenBlock), nil
}

func (db *DB) InsertUnbondingLock(id *big.Int, delegator ethcommon.Address, amount, withdrawRound *big.Int) error {
	glog.V(DEBUG).Infof("db: Inserting unbonding lock %v for delegator %v", id, delegator.Hex())
	_, err := db.insertUnbondingLock.Exec(id.Int64(), delegator.Hex(), amount.String(), withdrawRound.Int64())
	if err != nil {
		glog.Errorf("db: Error inserting unbonding lock %v for delegator %v: %v", id, delegator.Hex(), err)
		return err
	}
	return nil
}

func (db *DB) UseUnbondingLock(id *big.Int, delegator ethcommon.Address, usedBlock *big.Int) error {
	glog.V(DEBUG).Infof("db: Using unbonding lock %v for delegator %v", id, delegator.Hex())
	_, err := db.useUnbondingLock.Exec(usedBlock.Int64(), id.Int64(), delegator.Hex())
	if err != nil {
		glog.Errorf("db: Error using unbonding lock %v for delegator %v: %v", id, delegator.Hex(), err)
		return err
	}
	return nil
}

func (db *DB) UnbondingLockIDs() ([]*big.Int, error) {
	glog.V(DEBUG).Infof("db: Querying unbonding lock IDs")

	rows, err := db.dbh.Query("SELECT id FROM unbondingLocks")
	if err != nil {
		glog.Error("db: Unable to select unbonding lock IDs ", err)
		return nil, err
	}
	defer rows.Close()
	unbondingLockIDs := []*big.Int{}
	for rows.Next() {
		var unbondingLockID int64
		if err := rows.Scan(&unbondingLockID); err != nil {
			glog.Error("db: Unable to fetch unbonding lock ID ", err)
			continue
		}
		unbondingLockIDs = append(unbondingLockIDs, big.NewInt(unbondingLockID))
	}
	return unbondingLockIDs, nil
}

func (db *DB) UnbondingLocks(currentRound *big.Int) ([]*DBUnbondingLock, error) {
	if db == nil {
		return []*DBUnbondingLock{}, nil
	}
	glog.V(DEBUG).Infof("db: Querying unbonding locks")

	var (
		rows *sql.Rows
		err  error
	)

	if currentRound == nil {
		rows, err = db.unbondingLocks.Query()
	} else {
		rows, err = db.withdrawableUnbondingLocks.Query(currentRound.Int64())
	}
	if err != nil {
		glog.Error("db: Unable to select unbonding locks ", err)
		return nil, err
	}
	defer rows.Close()
	unbondingLocks := []*DBUnbondingLock{}
	for rows.Next() {
		var unbondingLock DBUnbondingLock
		var delegator string
		var amount string
		if err := rows.Scan(&unbondingLock.ID, &delegator, &amount, &unbondingLock.WithdrawRound); err != nil {
			glog.Error("db: Unable to fetch unbonding lock ", err)
			continue
		}
		unbondingLock.Delegator = ethcommon.HexToAddress(delegator)

		bigAmount, ok := new(big.Int).SetString(amount, 10)
		if !ok {
			glog.Errorf("db: Unable to convert amount string %v to big int", amount)
			continue
		}

		unbondingLock.Amount = bigAmount

		unbondingLocks = append(unbondingLocks, &unbondingLock)
	}
	return unbondingLocks, nil
}
