package common

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"text/template"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/eth/blockwatch"
	"github.com/livepeer/go-livepeer/pm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type DB struct {
	dbh *sql.DB

	// prepared statements
	selectOrchs                      *sql.Stmt
	updateOrch                       *sql.Stmt
	selectKV                         *sql.Stmt
	updateKV                         *sql.Stmt
	insertUnbondingLock              *sql.Stmt
	deleteUnbondingLock              *sql.Stmt
	useUnbondingLock                 *sql.Stmt
	unbondingLocks                   *sql.Stmt
	withdrawableUnbondingLocks       *sql.Stmt
	insertWinningTicket              *sql.Stmt
	insertMiniHeader                 *sql.Stmt
	findLatestMiniHeader             *sql.Stmt
	findAllMiniHeadersSortedByNumber *sql.Stmt
	deleteMiniHeader                 *sql.Stmt
}

type DBOrch struct {
	ServiceURI    string
	EthereumAddr  string
	PricePerPixel int64
}

type DBUnbondingLock struct {
	ID            int64
	Delegator     ethcommon.Address
	Amount        *big.Int
	WithdrawRound int64
}

type DBOrchFilter struct {
	MaxPrice *big.Rat
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

	CREATE TABLE IF NOT EXISTS orchestrators (
		ethereumAddr STRING PRIMARY KEY,
		createdAt STRING DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updatedAt STRING DEFAULT CURRENT_TIMESTAMP NOT NULL,
		serviceURI STRING,
		pricePerPixel int64
	);

	CREATE TABLE IF NOT EXISTS unbondingLocks (
		createdAt STRING DEFAULT CURRENT_TIMESTAMP,
		id INTEGER NOT NULL,
		delegator STRING,
		amount TEXT,
		withdrawRound int64,
		usedBlock int64,
		PRIMARY KEY(id, delegator)
	);
	-- Index to only retrieve unbonding locks that have not been used
	CREATE INDEX IF NOT EXISTS idx_unbondinglocks_usedblock ON unbondingLocks(usedBlock);

	CREATE TABLE IF NOT EXISTS winningTickets (
		createdAt STRING DEFAULT CURRENT_TIMESTAMP,
		sender STRING,
		recipient STRING,
		faceValue BLOB,
		winProb BLOB,
		senderNonce INTEGER,
		recipientRand BLOB,
		recipientRandHash STRING,
		sig BLOB,
		sessionID STRING
	);

	CREATE INDEX IF NOT EXISTS idx_winningtickets_sessionid ON winningTickets(sessionID);

	CREATE TABLE IF NOT EXISTS blockheaders (
		number int64,
		parent STRING,
		hash STRING PRIMARY KEY,
		logs BLOB
	);

	CREATE INDEX IF NOT EXISTS idx_blockheaders_number ON blockheaders(number);
`

func NewDBOrch(serviceURI string, orchAddr string) *DBOrch {
	return &DBOrch{ServiceURI: serviceURI, EthereumAddr: orchAddr}
}

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

	// updateOrchestrators statement
	stmt, err := db.Prepare("INSERT OR REPLACE INTO orchestrators(updatedAt, serviceURI, ethereumAddr, pricePerPixel, createdAt) VALUES(datetime(), ?1, ?2, ?3, (SELECT createdAt FROM orchestrators WHERE ethereumAddr = ?2))")
	if err != nil {
		glog.Error("Unable to prepare updateOrchestrators stmt ", err)
		d.Close()
		return nil, err
	}
	d.updateOrch = stmt

	// selectKV prepared statement
	stmt, err = db.Prepare("SELECT value FROM kv WHERE key=?")
	if err != nil {
		glog.Error("Unable to prepare selectKV stmt", err)
		d.Close()
		return nil, err
	}
	d.selectKV = stmt

	// updateKV prepared statement
	stmt, err = db.Prepare("INSERT OR REPLACE INTO kv(key, value, updatedAt) VALUES(?1, ?2, datetime())")
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
	stmt, err = db.Prepare("DELETE FROM unbondingLocks WHERE id=? AND delegator=?")
	if err != nil {
		glog.Error("Unable to prepare deleteUnbondingLock ", err)
		d.Close()
		return nil, err
	}
	d.deleteUnbondingLock = stmt
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

	// Winning tickets prepared statements
	stmt, err = db.Prepare("INSERT INTO winningTickets(sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, sessionID) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insertWinningTicket ", err)
		d.Close()
		return nil, err
	}
	d.insertWinningTicket = stmt

	// Insert block header
	stmt, err = db.Prepare("INSERT INTO blockheaders(number, parent, hash, logs) VALUES(?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insertMiniHeader", err)
		d.Close()
		return nil, err
	}
	d.insertMiniHeader = stmt

	// Find the latest block header
	stmt, err = db.Prepare("SELECT * FROM blockheaders ORDER BY number DESC LIMIT 1")
	if err != nil {
		glog.Error("Unable to prepare findLatestMiniHeader", err)
		d.Close()
		return nil, err
	}
	d.findLatestMiniHeader = stmt

	// Find all block headers sorted by number
	stmt, err = db.Prepare("SELECT * FROM blockheaders ORDER BY number DESC")
	if err != nil {
		glog.Error("Unable to prepare findAllMiniHeadersSortedByNumber", err)
		d.Close()
		return nil, err
	}
	d.findAllMiniHeadersSortedByNumber = stmt

	// Delete block header
	stmt, err = db.Prepare("DELETE FROM blockheaders WHERE hash=?")
	if err != nil {
		glog.Error("Unable to prepare deleteMiniHeader", err)
		d.Close()
		return nil, err
	}
	d.deleteMiniHeader = stmt

	glog.V(DEBUG).Info("Initialized DB node")
	return &d, nil
}

func (db *DB) Close() {
	glog.V(DEBUG).Info("Closing DB")
	if db.updateOrch != nil {
		db.updateOrch.Close()
	}
	if db.selectOrchs != nil {
		db.selectOrchs.Close()
	}
	if db.selectKV != nil {
		db.selectKV.Close()
	}
	if db.updateKV != nil {
		db.updateKV.Close()
	}
	if db.insertUnbondingLock != nil {
		db.insertUnbondingLock.Close()
	}
	if db.deleteUnbondingLock != nil {
		db.deleteMiniHeader.Close()
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
	if db.insertWinningTicket != nil {
		db.insertWinningTicket.Close()
	}
	if db.insertMiniHeader != nil {
		db.insertMiniHeader.Close()
	}
	if db.findLatestMiniHeader != nil {
		db.findLatestMiniHeader.Close()
	}
	if db.findAllMiniHeadersSortedByNumber != nil {
		db.findAllMiniHeadersSortedByNumber.Close()
	}
	if db.deleteMiniHeader != nil {
		db.deleteMiniHeader.Close()
	}
	if db.dbh != nil {
		db.dbh.Close()
	}
}

// LastSeenBlock returns the last block number stored by the DB
func (db *DB) LastSeenBlock() (*big.Int, error) {
	header, err := db.FindLatestMiniHeader()
	if err != nil {
		return nil, err
	}
	if header == nil {
		return nil, nil
	}

	return header.Number, nil
}

func (db *DB) ChainID() (*big.Int, error) {
	idString, err := db.selectKVStore("chainID")
	if err != nil {
		return nil, err
	}

	if idString == "" {
		return nil, nil
	}

	id, ok := new(big.Int).SetString(idString, 10)
	if !ok {
		return nil, fmt.Errorf("unable to convert chainID string to big.Int")
	}

	return id, nil
}

func (db *DB) SetChainID(id *big.Int) error {
	if err := db.updateKVStore("chainID", id.String()); err != nil {
		return err
	}
	return nil
}

func (db *DB) selectKVStore(key string) (string, error) {
	row := db.selectKV.QueryRow(key)
	var valueString string
	if err := row.Scan(&valueString); err != nil {
		if err.Error() != "sql: no rows in result set" {
			return "", fmt.Errorf("could not retrieve key from database: %v", err)
		}
		// If there is no result return no error, just zero value
		return "", nil
	}
	return valueString, nil
}

func (db *DB) updateKVStore(key, value string) error {
	_, err := db.updateKV.Exec(key, value)
	if err != nil {
		glog.Errorf("db: Unable to update %v in database: %v", key, err)
	}
	return err
}

func (db *DB) UpdateOrch(orch *DBOrch) error {
	if db == nil || orch == nil || orch.ServiceURI == "" || orch.EthereumAddr == "" {
		return nil
	}

	_, err := db.updateOrch.Exec(orch.ServiceURI, orch.EthereumAddr, orch.PricePerPixel)
	if err != nil {
		glog.Error("db: Unable to update orchestrator ", err)
	}

	return err
}

func (db *DB) SelectOrchs(filter *DBOrchFilter) ([]*DBOrch, error) {
	if db == nil {
		return nil, nil
	}

	rows, err := db.dbh.Query(buildSelectOrchsQuery(filter))
	defer rows.Close()
	if err != nil {
		glog.Error("db: Unable to get orchestrators updated in the last 24 hours: ", err)
		return nil, err
	}
	orchs := []*DBOrch{}
	for rows.Next() {
		var orch DBOrch
		var serviceURI string
		var ethereumAddr string
		if err := rows.Scan(&serviceURI, &ethereumAddr); err != nil {
			glog.Error("db: Unable to fetch orchestrator ", err)
			continue
		}
		orch.ServiceURI = serviceURI
		orch.EthereumAddr = ethereumAddr
		orchs = append(orchs, &orch)
	}
	return orchs, nil
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

// DeleteUnbondingLock deletes an unbonding lock from the DB with the given ID and delegator address.
// This method will return nil for non-existent unbonding locks
func (db *DB) DeleteUnbondingLock(id *big.Int, delegator ethcommon.Address) error {
	glog.V(DEBUG).Infof("db: Deleting unbonding lock %v for delegator %v", id, delegator.Hex())
	_, err := db.deleteUnbondingLock.Exec(id.Int64(), delegator.Hex())
	if err != nil {
		glog.Errorf("db: Error deleting unbonding lock %v for delegator %v: %v", id, delegator.Hex(), err)
		return err
	}
	return nil
}

// UseUnbondingLock sets an unbonding lock in the DB as used by setting the lock's used block.
// If usedBlock is nil this method will set the lock's used block to NULL
func (db *DB) UseUnbondingLock(id *big.Int, delegator ethcommon.Address, usedBlock *big.Int) error {
	glog.V(DEBUG).Infof("db: Using unbonding lock %v for delegator %v", id, delegator.Hex())

	var err error
	if usedBlock == nil {
		_, err = db.useUnbondingLock.Exec(nil, id.Int64(), delegator.Hex())
	} else {
		_, err = db.useUnbondingLock.Exec(usedBlock.Int64(), id.Int64(), delegator.Hex())
	}
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

func (db *DB) StoreWinningTicket(sessionID string, ticket *pm.Ticket, sig []byte, recipientRand *big.Int) error {
	if ticket == nil {
		return errors.New("cannot store nil ticket")
	}
	if sig == nil {
		return errors.New("cannot store nil sig")
	}
	if recipientRand == nil {
		return errors.New("cannot store nil recipientRand")
	}
	glog.V(DEBUG).Infof("db: Inserting winning ticket from %v, recipientRand %d, senderNonce %d", ticket.Sender.Hex(), recipientRand, ticket.SenderNonce)

	_, err := db.insertWinningTicket.Exec(ticket.Sender.Hex(), ticket.Recipient.Hex(), ticket.FaceValue.Bytes(), ticket.WinProb.Bytes(), ticket.SenderNonce, recipientRand.Bytes(), ticket.RecipientRandHash.Hex(), sig, sessionID)

	if err != nil {
		return errors.Wrapf(err, "failed inserting winning ticket for sessionID: %v, ticket: %v", sessionID, ticket)
	}
	return nil
}

func (db *DB) LoadWinningTickets(sessionIDs []string) (tickets []*pm.Ticket, sigs [][]byte, recipientRands []*big.Int, err error) {
	rows, err := db.dbh.Query(buildWinningTicketsQuery(sessionIDs))
	defer rows.Close()

	if err != nil {
		err = errors.Wrapf(err, "failed loading winning tickets for sessionIDs %v", sessionIDs)
		return
	}

	for rows.Next() {
		var sender, recipient, recipientRandHash, sessionID string
		var faceValue, winProb, recipientRandBytes, sig []byte
		var senderNonce uint32

		err = rows.Scan(&sender, &recipient, &faceValue, &winProb, &senderNonce, &recipientRandBytes, &recipientRandHash, &sig, &sessionID)
		if err != nil {
			err = errors.Wrapf(err, "failed scanning a winning ticket row for sessionID %v", sessionID)
			return
		}

		ticket := &pm.Ticket{
			Sender:            ethcommon.HexToAddress(sender),
			Recipient:         ethcommon.HexToAddress(recipient),
			FaceValue:         new(big.Int).SetBytes(faceValue),
			WinProb:           new(big.Int).SetBytes(winProb),
			SenderNonce:       senderNonce,
			RecipientRandHash: ethcommon.HexToHash(recipientRandHash),
		}
		recipientRand := new(big.Int).SetBytes(recipientRandBytes)

		tickets = append(tickets, ticket)
		sigs = append(sigs, sig)
		recipientRands = append(recipientRands, recipientRand)
	}

	return
}

// We are building a query string instead of using a prepared statement because prepared statements don't
// support IN queries. We want to use IN for the performance benefit, rather than running len(sessionIDs)
// queries.
// The known risk with string queries is SQL injection, however in this case all sessionID values are generated
// by O, so such injections are not likely.
func buildWinningTicketsQuery(sessionIDs []string) string {
	for i := 0; i < len(sessionIDs); i++ {
		sessionIDs[i] = strconv.Quote(sessionIDs[i])
	}
	return "SELECT sender, recipient, faceValue, winProb, senderNonce, recipientRand, recipientRandHash, sig, sessionID FROM winningTickets WHERE sessionID IN (" + strings.Join(sessionIDs, ", ") + ")"
}

func buildSelectOrchsQuery(filter *DBOrchFilter) (string, error) {
	query := "SELECT serviceURI, ethereumAddr FROM orchestrators WHERE updatedAt >= datetime('now','-1 day')"
	if filter != nil && filter.MaxPrice != nil {
		fixedPrice, err := PriceToFixed(filter.MaxPrice)
		if err != nil {
			return "", err
		}
		query = query + " AND pricePerPixel <= " + strconv.FormatInt(fixedPrice, 10)
	}
	return query, nil
}

// FindLatestMiniHeader returns the MiniHeader with the highest blocknumber in the DB
func (db *DB) FindLatestMiniHeader() (*blockwatch.MiniHeader, error) {
	row := db.findLatestMiniHeader.QueryRow()
	var (
		number  int64
		parent  string
		hash    string
		logsEnc []byte
	)
	if err := row.Scan(&number, &parent, &hash, &logsEnc); err != nil {
		if err.Error() != "sql: no rows in result set" {
			return nil, fmt.Errorf("could not retrieve latest header: %v", err)
		}
		// If there is no result return no error, just nil value
		return nil, nil
	}

	logs, err := decodeLogsJSON(logsEnc)
	if err != nil {
		return nil, err
	}
	return &blockwatch.MiniHeader{
		Number: big.NewInt(number),
		Parent: ethcommon.HexToHash(parent),
		Hash:   ethcommon.HexToHash(hash),
		Logs:   logs,
	}, nil
}

// FindAllMiniHeadersSortedByNumber returns all MiniHeaders in the DB sorting in descending order by block number
func (db *DB) FindAllMiniHeadersSortedByNumber() ([]*blockwatch.MiniHeader, error) {
	var headers []*blockwatch.MiniHeader
	rows, err := db.findAllMiniHeadersSortedByNumber.Query()
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			number  int64
			parent  string
			hash    string
			logsEnc []byte
		)
		if err := rows.Scan(&number, &parent, &hash, &logsEnc); err != nil {
			return nil, err
		}
		logs, err := decodeLogsJSON(logsEnc)
		if err != nil {
			return nil, err
		}
		headers = append(headers, &blockwatch.MiniHeader{
			Number: big.NewInt(number),
			Parent: ethcommon.HexToHash(parent),
			Hash:   ethcommon.HexToHash(hash),
			Logs:   logs,
		})
	}
	return headers, nil
}

// InsertMiniHeader inserts a MiniHeader into the database
func (db *DB) InsertMiniHeader(header *blockwatch.MiniHeader) error {
	if header == nil {
		return errors.New("must provide a MiniHeader")
	}
	if header.Number == nil {
		return errors.New("no block number found")
	}
	logsEnc, err := encodeLogsJSON(header.Logs)
	if err != nil {
		return err
	}
	_, err = db.insertMiniHeader.Exec(header.Number.Int64(), header.Parent.Hex(), header.Hash.Hex(), logsEnc)
	if err != nil {
		return err
	}
	return nil
}

// DeleteMiniHeader deletes a MiniHeader from the DB and takes in the blockhash of the block to be deleted as an argument
func (db *DB) DeleteMiniHeader(hash ethcommon.Hash) error {
	_, err := db.deleteMiniHeader.Exec(hash.Hex())
	if err != nil {
		return err
	}
	return nil
}

func encodeLogsJSON(logs []types.Log) ([]byte, error) {
	logsEnc, err := json.Marshal(logs)
	if err != nil {
		return []byte{}, err
	}
	return logsEnc, nil
}

func decodeLogsJSON(logsEnc []byte) ([]types.Log, error) {
	var logs []types.Log
	err := json.Unmarshal(logsEnc, &logs)
	if err != nil {
		return []types.Log{}, err
	}
	return logs, nil
}
