package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"

	ethcommon "github.com/ethereum/go-ethereum/common"

	"google.golang.org/grpc/metadata"
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

func TestServeTranscoder(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	strm := &StubTranscoderServer{}

	// test that a transcoder was created
	go n.serveTranscoder(strm)
	time.Sleep(1 * time.Second)
	if n.Transcoder == nil {
		t.Error("Transcoder nil")
	}
	tc, ok := n.Transcoder.(*RemoteTranscoder)
	if !ok {
		t.Error("Unexpected transcoder type")
	}

	// test shutdown
	tc.eof <- struct{}{}
	time.Sleep(1 * time.Second)
	if n.Transcoder != nil {
		t.Error("Transcoder not nil")
	}
}

func TestRemoteTranscoder(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	initTranscoder := func() (*RemoteTranscoder, *StubTranscoderServer) {
		strm := &StubTranscoderServer{node: n}
		tc := NewRemoteTranscoder(n, strm)
		return tc, strm
	}

	// happy path
	tc, strm := initTranscoder()
	res, err := tc.Transcode("", nil)
	if err != nil || string(res[0]) != "asdf" {
		t.Error("Error transcoding ", err)
	}

	// error on remote while transcoding
	tc, strm = initTranscoder()
	strm.TranscodeError = fmt.Errorf("TranscodeError")
	res, err = tc.Transcode("", nil)
	if err != strm.TranscodeError {
		t.Error("Unexpected error ", err, res)
	}

	// simulate error with sending
	tc, strm = initTranscoder()
	strm.SendError = fmt.Errorf("SendError")
	_, err = tc.Transcode("", nil)
	if err != strm.SendError {
		t.Error("Unexpected error ", err)
	}

	// simulate timeout
	tc, strm = initTranscoder()
	strm.WithholdResults = true
	RemoteTranscoderTimeout = 1 * time.Millisecond
	_, err = tc.Transcode("", nil)
	if err.Error() != "Remote transcoder took too long" {
		t.Error("Unexpected error ", err)
	}
	RemoteTranscoderTimeout = 8 * time.Second
}

func TestTaskChan(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	// Sanity check task ID
	if n.taskCount != 0 {
		t.Error("Unexpected taskid")
	}
	if len(n.taskChans) != int(n.taskCount) {
		t.Error("Unexpected task chan length")
	}

	// Adding task chans
	const MaxTasks = 1000
	for i := 0; i < MaxTasks; i++ {
		go n.addTaskChan() // hopefully concurrently...
	}
	for j := 0; j < 10; j++ {
		n.taskMutex.RLock()
		tid := n.taskCount
		n.taskMutex.RUnlock()
		if tid >= MaxTasks {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n.taskCount != MaxTasks {
		t.Error("Time elapsed")
	}
	if len(n.taskChans) != int(n.taskCount) {
		t.Error("Unexpected task chan length")
	}

	// Accessing task chans
	existingIds := []int64{0, 1, MaxTasks / 2, MaxTasks - 2, MaxTasks - 1}
	for _, id := range existingIds {
		_, err := n.getTaskChan(int64(id))
		if err != nil {
			t.Error("Unexpected error getting task chan for ", id, err)
		}
	}
	missingIds := []int64{-1, MaxTasks}
	testNonexistentChans := func(ids []int64) {
		for _, id := range ids {
			_, err := n.getTaskChan(int64(id))
			if err == nil || err.Error() != "No transcoder channel" {
				t.Error("Did not get expected error for ", id, err)
			}
		}
	}
	testNonexistentChans(missingIds)

	// Removing task chans
	for i := 0; i < MaxTasks; i++ {
		go n.removeTaskChan(int64(i)) // hopefully concurrently...
	}
	for j := 0; j < 10; j++ {
		n.taskMutex.RLock()
		tlen := len(n.taskChans)
		n.taskMutex.RUnlock()
		if tlen <= 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(n.taskChans) != 0 {
		t.Error("Time elapsed")
	}
	testNonexistentChans(existingIds) // sanity check for removal
}

type StubTranscoderServer struct {
	node            *LivepeerNode
	SendError       error
	TranscodeError  error
	WithholdResults bool

	StubServerStream
}

func (s *StubTranscoderServer) Send(n *net.NotifySegment) error {
	res := RemoteTranscoderResult{Segments: [][]byte{[]byte("asdf")}, Err: s.TranscodeError}
	if !s.WithholdResults {
		s.node.transcoderResults(n.TaskId, &res)
	}
	return s.SendError
}

type StubServerStream struct {
}

func (s *StubServerStream) Context() context.Context {
	return context.Background()
}
func (s *StubServerStream) SetHeader(md metadata.MD) error {
	return nil
}
func (s *StubServerStream) SendHeader(md metadata.MD) error {
	return nil
}
func (s *StubServerStream) SetTrailer(md metadata.MD) {
}
func (s *StubServerStream) SendMsg(m interface{}) error {
	return nil
}
func (s *StubServerStream) RecvMsg(m interface{}) error {
	return nil
}
