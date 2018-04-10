package common

import (
	"bytes"
	"database/sql"
	"errors"
	"math/big"
	"text/template"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/lpms/ffmpeg"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	dbh *sql.DB

	// prepared statements
	updateKV  *sql.Stmt
	insertJob *sql.Stmt
}

type DBJob struct {
	ID          int64
	streamID    string
	price       int64
	profiles    []ffmpeg.VideoProfile
	broadcaster ethcommon.Address
	transcoder  ethcommon.Address
	startBlock  int64
	endBlock    int64
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

	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY,
		recordedAt STRING DEFAULT CURRENT_TIMESTAMP,
		streamID STRING,
		segmentPrice INTEGER,
		transcodeOptions STRING,
		broadcaster STRING,
		transcoder STRING,
		startBlock INTEGER,
		endBlock INTEGER
	);
`

func NewDBJob(id *big.Int, streamID string,
	segmentPrice *big.Int, profiles []ffmpeg.VideoProfile,
	broadcaster ethcommon.Address, transcoder ethcommon.Address,
	startBlock *big.Int, endBlock *big.Int) *DBJob {
	return &DBJob{
		ID: id.Int64(), streamID: streamID, profiles: profiles,
		price: segmentPrice.Int64(), broadcaster: broadcaster, transcoder: transcoder,
		startBlock: startBlock.Int64(), endBlock: endBlock.Int64(),
	}
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

	// insertJob prepared statement
	stmt, err = db.Prepare("INSERT INTO jobs(id, streamID, segmentPrice, transcodeOptions, broadcaster, transcoder, startBlock, endBlock) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insertjob stmt ", err)
		d.Close()
		return nil, err
	}
	d.insertJob = stmt

	glog.V(DEBUG).Info("Initialized DB node")
	return &d, nil
}

func (db *DB) Close() {
	glog.V(DEBUG).Info("Closing DB")
	if db.updateKV != nil {
		db.updateKV.Close()
	}
	if db.insertJob != nil {
		db.insertJob.Close()
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

func (db *DB) InsertJob(job *DBJob) error {
	if db == nil {
		return nil
	}
	options := ethcommon.ToHex(ProfilesToTranscodeOpts(job.profiles))
	glog.V(DEBUG).Info("db: Inserting job ", job.ID)
	_, err := db.insertJob.Exec(job.ID, job.streamID, job.price, options,
		job.broadcaster.String(), job.transcoder.String(),
		job.startBlock, job.endBlock)
	if err != nil {
		glog.Error("db: Unable to insert job ", err)
	}
	return err
}
