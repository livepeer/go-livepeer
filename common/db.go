package common

import (
	"bytes"
	"database/sql"
	"errors"
	"math/big"
	"text/template"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/lpms/ffmpeg"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	dbh *sql.DB

	// prepared statements
	updateKV          *sql.Stmt
	insertJob         *sql.Stmt
	selectJobs        *sql.Stmt
	stopReason        *sql.Stmt
	insertRec         *sql.Stmt
	checkRec          *sql.Stmt
	insertClaim       *sql.Stmt
	countClaims       *sql.Stmt
	setReceiptClaim   *sql.Stmt
	setClaimStatus    *sql.Stmt
	unclaimedReceipts *sql.Stmt
	receiptsByClaim   *sql.Stmt
	insertBcast       *sql.Stmt
}

type DBJob struct {
	ID          int64
	streamID    string
	price       int64
	profiles    []ffmpeg.VideoProfile
	broadcaster ethcommon.Address
	Transcoder  ethcommon.Address
	startBlock  int64
	endBlock    int64
	stopReason  string
}

type DBReceipt struct {
	JobID     int64
	SeqNo     int64
	BcastFile string
	BcastHash []byte
	BcastSig  []byte
	TcodeHash []byte
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
		endBlock INTEGER,
		stopReason STRING DEFAULT NULL,
		stoppedAt STRING DEFAULT NULL
	);
	-- Index to avoid a full jobs table scan during recovery
	CREATE INDEX IF NOT EXISTS idx_jobs_endblock_stopreason ON jobs(endBlock, stopReason);

	CREATE TABLE IF NOT EXISTS claims (
		id INTEGER,
		jobID INTEGER,
		claimRoot STRING,
		claimBlock INTEGER,
		claimedAt STRING DEFAULT CURRENT_TIMESTAMP,
		updatedAt STRING DEFAULT CURRENT_TIMESTAMP,
		status STRING DEFAULT 'Created',
		PRIMARY KEY(id, jobID),
		FOREIGN KEY(jobID) REFERENCES jobs(id)
	);

	CREATE TABLE IF NOT EXISTS receipts (
		jobID INTEGER NOT NULL,
		claimID INTEGER,
		seqNo INTEGER NOT NULL,
		bcastFile STRING,
		bcastHash STRING,
		bcastSig STRING,
		transcodedHash STRING,
		transcodeStartedAt STRING,
		transcodeEndedAt STRING,
		errorMsg STRING DEFAULT NULL,
		PRIMARY KEY(jobID, seqNo),
		FOREIGN KEY(jobID) REFERENCES jobs(id),
		FOREIGN KEY(claimID, jobID) REFERENCES claims(id, jobID)
	);
	CREATE INDEX IF NOT EXISTS idx_receipts_claimid_errormsg ON receipts(claimID, errorMsg);

	CREATE TABLE IF NOT EXISTS broadcasts (
		id INTEGER PRIMARY KEY,
		recordedAt STRING DEFAULT CURRENT_TIMESTAMP,
		streamID STRING,
		segmentPrice INTEGER,
		segmentCount INTEGER DEFAULT 0,
		transcodeOptions STRING,
		broadcaster STRING,
		transcoder STRING,
		startBlock INTEGER,
		endBlock INTEGER,
		stopReason STRING DEFAULT NULL,
		stoppedAt STRING DEFAULT NULL
	);
	-- Index to avoid a full table scan
	CREATE INDEX IF NOT EXISTS idx_broadcasts_endblock_stopreason ON broadcasts(endBlock, stopReason);
`

func NewDBJob(id *big.Int, streamID string,
	segmentPrice *big.Int, profiles []ffmpeg.VideoProfile,
	broadcaster ethcommon.Address, transcoder ethcommon.Address,
	startBlock *big.Int, endBlock *big.Int) *DBJob {
	return &DBJob{
		ID: id.Int64(), streamID: streamID, profiles: profiles,
		price: segmentPrice.Int64(), broadcaster: broadcaster, Transcoder: transcoder,
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

	// select all jobs since
	stmt, err = db.Prepare("SELECT id, streamID, segmentPrice, transcodeOptions, broadcaster, transcoder, startBlock, endBlock FROM jobs WHERE endBlock > ? AND stopReason IS NULL")
	if err != nil {
		glog.Error("Unable to prepare selectjob stmt ", err)
		d.Close()
		return nil, err
	}
	d.selectJobs = stmt

	// set reason for stopping a job
	stmt, err = db.Prepare("UPDATE jobs SET stopReason=?, stoppedAt=datetime() WHERE id=?")
	if err != nil {
		glog.Error("Unable to prepare stop reason statement ", err)
		d.Close()
		return nil, err
	}
	d.stopReason = stmt

	// Insert receipt prepared statement
	stmt, err = db.Prepare("INSERT INTO receipts(jobID, seqNo, bcastFile, bcastHash, bcastSig, transcodedHash, transcodeStartedAt, transcodeEndedAt) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insert segment ", err)
		d.Close()
		return nil, err
	}
	d.insertRec = stmt

	// Check if receipt with the given seqno exists
	stmt, err = db.Prepare("SELECT 1 FROM receipts WHERE jobID=? AND seqNo=?")
	if err != nil {
		glog.Error("Unable to prepare segment exists statement")
		d.Close()
		return nil, err
	}
	d.checkRec = stmt

	// Recover receipt prepared statement
	stmt, err = db.Prepare("SELECT jobID, seqNo, bcastFile, bcastHash, bcastSig, transcodedHash FROM receipts WHERE claimID IS NULL and errorMsg IS NULL")
	if err != nil {
		glog.Error("Unable to prepare unclaimed receipts", err)
		d.Close()
		return nil, err
	}
	d.unclaimedReceipts = stmt

	// Receipts by claim for removing old segments
	stmt, err = db.Prepare("SELECT bcastFile FROM receipts WHERE claimID = ? AND jobID = ?")
	if err != nil {
		glog.Error("Unable to prepare receipts by claim ", err)
		d.Close()
		return nil, err
	}
	d.receiptsByClaim = stmt

	// Claim related prepared statements
	stmt, err = db.Prepare("INSERT INTO claims(id, jobID, claimRoot) VALUES(?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare insert claims ", err)
		d.Close()
		return nil, err
	}
	d.insertClaim = stmt
	stmt, err = db.Prepare("SELECT count(*) FROM claims WHERE jobID=?")
	if err != nil {
		glog.Error("Unable to prepare claim count ", err)
		d.Close()
		return nil, err
	}
	d.countClaims = stmt
	stmt, err = db.Prepare("UPDATE receipts SET claimID = ? WHERE jobID = ? AND seqNo BETWEEN ? AND ?")
	if err != nil {
		glog.Error("Unable to prepare setclaimid ", err)
		d.Close()
		return nil, err
	}
	d.setReceiptClaim = stmt

	stmt, err = db.Prepare("UPDATE claims SET status=?, updatedAt=datetime() WHERE jobID=? AND id=?")
	if err != nil {
		glog.Error("Unable to prepare  setclaimstatus ", err)
		d.Close()
		return nil, err
	}
	d.setClaimStatus = stmt

	// Broadcast related
	stmt, err = db.Prepare("INSERT INTO broadcasts(id, streamID, segmentPrice, transcodeOptions, broadcaster, transcoder, startBlock, endBlock) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		glog.Error("Unable to prepare broadcast insertion ", err)
		d.Close()
		return nil, err
	}
	d.insertBcast = stmt

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
	if db.selectJobs != nil {
		db.selectJobs.Close()
	}
	if db.stopReason != nil {
		db.stopReason.Close()
	}
	if db.insertRec != nil {
		db.insertRec.Close()
	}
	if db.checkRec != nil {
		db.checkRec.Close()
	}
	if db.unclaimedReceipts != nil {
		db.unclaimedReceipts.Close()
	}
	if db.receiptsByClaim != nil {
		db.receiptsByClaim.Close()
	}
	if db.insertClaim != nil {
		db.insertClaim.Close()
	}
	if db.setReceiptClaim != nil {
		db.setReceiptClaim.Close()
	}
	if db.setClaimStatus != nil {
		db.setClaimStatus.Close()
	}
	if db.insertBcast != nil {
		db.insertBcast.Close()
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
	options := ethcommon.ToHex(ProfilesToTranscodeOpts(job.profiles))[2:]
	glog.V(DEBUG).Info("db: Inserting job ", job.ID)
	_, err := db.insertJob.Exec(job.ID, job.streamID, job.price, options,
		job.broadcaster.String(), job.Transcoder.String(),
		job.startBlock, job.endBlock)
	if err != nil {
		glog.Error("db: Unable to insert job ", err)
	}
	return err
}

func (db *DB) ActiveJobs(since *big.Int) ([]*DBJob, error) {
	if db == nil {
		return []*DBJob{}, nil
	}
	glog.V(DEBUG).Info("db: Querying active jobs since ", since)
	rows, err := db.selectJobs.Query(since.Int64())
	if err != nil {
		glog.Error("db: Unable to select jobs ", err)
		return nil, err
	}
	defer rows.Close()
	jobs := []*DBJob{}
	for rows.Next() {
		var job DBJob
		var transcoder string
		var broadcaster string
		var options string
		if err := rows.Scan(&job.ID, &job.streamID, &job.price, &options, &broadcaster, &transcoder, &job.startBlock, &job.endBlock); err != nil {
			glog.Error("db: Unable to fetch job ", err)
			continue
		}
		profiles, err := TxDataToVideoProfile(options)
		if err != nil {
			glog.Error("Unable to convert transcode options into ffmpeg profile ", err)
		}
		job.Transcoder = ethcommon.HexToAddress(transcoder)
		job.broadcaster = ethcommon.HexToAddress(broadcaster)
		job.profiles = profiles
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (db *DB) SetStopReason(id *big.Int, reason string) error {
	if db == nil {
		return nil
	}
	glog.V(DEBUG).Infof("db: Setting StopReason for job %v to %v", id, reason)
	_, err := db.stopReason.Exec(reason, id.Int64())
	if err != nil {
		glog.Error("db: Error setting stop reason ", id, err)
		return err
	}
	return nil
}

func (db *DB) ReceiptExists(jobID *big.Int, seqNo uint64) (bool, error) {
	if db == nil {
		return false, nil
	}
	glog.V(DEBUG).Infof("db: Checking receipt %v for job %v", seqNo, jobID)
	var res int
	err := db.checkRec.QueryRow(jobID.Int64(), seqNo).Scan(&res)
	if err != nil && err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		glog.Error("Could not check receipt existence ", err)
		return false, err
	}
	return true, nil
}

func (db *DB) InsertReceipt(jobID *big.Int, seqNo int64,
	bcastFile string, bcastHash []byte, bcastSig []byte, tcodeHash []byte,
	tcodeStartedAt time.Time, tcodeEndedAt time.Time) error {
	if db == nil {
		return nil
	}
	glog.V(DEBUG).Infof("db: Inserting receipt for job %v - %v", jobID.String(), seqNo)
	time2str := func(t time.Time) string {
		return t.UTC().Format("2006-01-02 15:04:05.000")
	}
	_, err := db.insertRec.Exec(jobID.Int64(), seqNo, bcastFile,
		ethcommon.ToHex(bcastHash), ethcommon.ToHex(bcastSig),
		ethcommon.ToHex(tcodeHash),
		time2str(tcodeStartedAt), time2str(tcodeEndedAt))
	if err != nil {
		glog.Error("db: Error inserting segment ", jobID, err)
		return err
	}
	return nil
}
func (db *DB) UnclaimedReceipts() (map[int64][]*DBReceipt, error) {
	receipts := make(map[int64][]*DBReceipt)
	glog.V(DEBUG).Info("db: Querying unclaimed receipts")
	rows, err := db.unclaimedReceipts.Query()
	defer rows.Close()
	if err != nil {
		glog.Error("db: Unable to select receipts ", err)
		return receipts, err
	}
	for rows.Next() {
		var r DBReceipt
		var bh string
		var bs string
		var th string
		if err := rows.Scan(&r.JobID, &r.SeqNo, &r.BcastFile, &bh, &bs, &th); err != nil {
			glog.Error("db: Unable to fetch receipt ", err)
			continue
		}
		r.BcastHash = ethcommon.FromHex(bh)
		r.BcastSig = ethcommon.FromHex(bs)
		r.TcodeHash = ethcommon.FromHex(th)
		receipts[r.JobID] = append(receipts[r.JobID], &r)
	}
	return receipts, nil
}

func (db *DB) ReceiptBCastFilesByClaim(claimID int64, jobID *big.Int) ([]string, error) {
	glog.V(DEBUG).Info("db: Querying receipts by claim")
	receipts := []string{}
	rows, err := db.receiptsByClaim.Query(claimID, jobID.Int64())
	defer rows.Close()
	if err != nil {
		glog.Error("db: Unable to select receipts ", err)
		return receipts, err
	}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			glog.Error("db: Unable to fetch receipt ", err)
			continue
		}
		receipts = append(receipts, f)
	}
	return receipts, nil
}

func (db *DB) InsertClaim(jobID *big.Int, segRange [2]int64,
	root [32]byte) (*int64, error) {
	glog.V(DEBUG).Infof("Inserting claim for job %v", jobID)
	tx, err := db.dbh.Begin()
	if err != nil {
		glog.Error("Unable to begin tx ", err)
		return nil, err
	}
	var claimID int64
	insert := tx.Stmt(db.insertClaim)
	count := tx.Stmt(db.countClaims)
	update := tx.Stmt(db.setReceiptClaim)
	row := count.QueryRow(jobID.Int64())
	err = row.Scan(&claimID)
	if err != nil {
		glog.Error("Unable to count claims ", err)
		tx.Rollback()
		return nil, err
	}
	glog.V(DEBUG).Infof("Guessed claim ID to be %v for job %v", claimID, jobID)
	_, err = insert.Exec(claimID, jobID.Int64(), ethcommon.ToHex(root[:]))
	if err != nil {
		glog.Error("Unable to insert claim ", err)
		tx.Rollback()
		return nil, err
	}
	_, err = update.Exec(claimID, jobID.Int64(), segRange[0], segRange[1])
	if err != nil {
		glog.Error("Unable to update segments with claims ", err)
		tx.Rollback()
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		glog.Error("Unable to commit tx ", err)
		tx.Rollback()
		return nil, err
	}
	return &claimID, nil
}

func (db *DB) SetClaimStatus(jobID *big.Int, id int64, status string) error {
	glog.V(DEBUG).Infof("db: Setting ClaimStatus for job %v claim %v to %v", jobID, id, status)
	_, err := db.setClaimStatus.Exec(status, jobID.Int64(), id)
	if err != nil {
		glog.Error("db: Error setting claim status ", id, err)
		return err
	}
	return nil
}

func (db *DB) InsertBroadcast(job *lpTypes.Job) error {
	options := ethcommon.ToHex(ProfilesToTranscodeOpts(job.Profiles))[2:]
	glog.V(DEBUG).Info("db: Inserting broadcast ", job.JobId)
	_, err := db.insertBcast.Exec(job.JobId.Int64(), job.StreamId,
		job.MaxPricePerSegment.Int64(), options,
		job.BroadcasterAddress.String(), job.TranscoderAddress.String(),
		job.CreationBlock.Int64(), job.EndBlock.Int64())
	if err != nil {
		glog.Error("db: Unable to insert job ", err)
	}
	return err
}
