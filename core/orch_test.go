package core

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/livepeer/go-livepeer/pm"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/lpms/ffmpeg"

	"github.com/livepeer/go-livepeer/net"
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

func TestServeTranscoder(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	n.TranscoderManager = NewRemoteTranscoderManager()
	strm := &StubTranscoderServer{}

	// test that a transcoder was created
	go n.serveTranscoder(strm, 5)
	time.Sleep(1 * time.Second)

	tc, ok := n.TranscoderManager.liveTranscoders[strm]
	if !ok {
		t.Error("Unexpected transcoder type")
	}

	// test shutdown
	tc.eof <- struct{}{}
	time.Sleep(1 * time.Second)

	// stream should be removed
	_, ok = n.TranscoderManager.liveTranscoders[strm]
	if ok {
		t.Error("Unexpected transcoder presence")
	}
}

func TestRemoteTranscoder(t *testing.T) {
	m := NewRemoteTranscoderManager()
	initTranscoder := func() (*RemoteTranscoder, *StubTranscoderServer) {
		strm := &StubTranscoderServer{manager: m}
		tc := NewRemoteTranscoder(m, strm, 5)
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
	if _, fatal := err.(RemoteTranscoderFatalError); !fatal ||
		err.Error() != strm.SendError.Error() {
		t.Error("Unexpected error ", err, fatal)
	}

	// simulate timeout
	tc, strm = initTranscoder()
	strm.WithholdResults = true
	m.taskCount = 1001
	RemoteTranscoderTimeout = 1 * time.Millisecond
	_, err = tc.Transcode("fileName", nil)
	if err.Error() != "Remote transcoder took too long" {
		t.Error("Unexpected error: ", err)
	}
	RemoteTranscoderTimeout = 8 * time.Second
}

func newWg(delta int) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(delta)
	return &wg
}

func wgWait(wg *sync.WaitGroup) bool {
	c := make(chan struct{})
	go func() { defer close(c); wg.Wait() }()
	select {
	case <-c:
		return true
	case <-time.After(1 * time.Second):
		return false
	}
}

func TestManageTranscoders(t *testing.T) {
	m := NewRemoteTranscoderManager()
	strm := &StubTranscoderServer{}
	strm2 := &StubTranscoderServer{manager: m}

	// sanity check that liveTranscoders and remoteTranscoders is empty
	assert := assert.New(t)
	assert.Nil(m.liveTranscoders[strm])
	assert.Nil(m.liveTranscoders[strm2])
	assert.Empty(m.remoteTranscoders)
	assert.Equal(0, m.RegisteredTranscodersCount())

	// test that transcoder is added to liveTranscoders and remoteTranscoders
	wg1 := newWg(1)
	go func() { m.Manage(strm, 5); wg1.Done() }()
	time.Sleep(1 * time.Millisecond) // allow the manager to activate

	assert.NotNil(m.liveTranscoders[strm])
	assert.Len(m.liveTranscoders, 1)
	assert.Len(m.remoteTranscoders, 5)
	assert.Equal(1, m.RegisteredTranscodersCount())
	ti := m.RegisteredTranscodersInfo()
	assert.Len(ti, 1)
	assert.Equal(5, ti[0].Capacity)
	assert.Equal("TestAddress", ti[0].Address)

	// test that additional transcoder is added to liveTranscoders and remoteTranscoders
	wg2 := newWg(1)
	go func() { m.Manage(strm2, 4); wg2.Done() }()
	time.Sleep(1 * time.Millisecond) // allow the manager to activate

	assert.NotNil(m.liveTranscoders[strm])
	assert.NotNil(m.liveTranscoders[strm2])
	assert.Len(m.liveTranscoders, 2)
	assert.Len(m.remoteTranscoders, 9)
	assert.Equal(2, m.RegisteredTranscodersCount())

	// test that transcoders are removed from liveTranscoders and remoteTranscoders
	m.liveTranscoders[strm].eof <- struct{}{}
	assert.True(wgWait(wg1)) // time limit
	assert.Nil(m.liveTranscoders[strm])
	assert.NotNil(m.liveTranscoders[strm2])
	assert.Len(m.liveTranscoders, 1)
	assert.Len(m.remoteTranscoders, 9)
	assert.Equal(1, m.RegisteredTranscodersCount())

	m.liveTranscoders[strm2].eof <- struct{}{}
	assert.True(wgWait(wg2)) // time limit
	assert.Nil(m.liveTranscoders[strm])
	assert.Nil(m.liveTranscoders[strm2])
	assert.Len(m.liveTranscoders, 0)
	assert.Len(m.remoteTranscoders, 9)
	assert.Equal(0, m.RegisteredTranscodersCount())
}

func TestSelectTranscoder(t *testing.T) {
	m := NewRemoteTranscoderManager()
	strm := &StubTranscoderServer{manager: m, WithholdResults: false}
	strm2 := &StubTranscoderServer{manager: m}

	// sanity check that transcoder is not in liveTranscoders or remoteTranscoders
	assert := assert.New(t)
	assert.Nil(m.liveTranscoders[strm])
	assert.Empty(m.remoteTranscoders)

	// register transcoders, which adds transcoder to liveTranscoders and remoteTranscoders
	wg := newWg(1)
	go func() { m.Manage(strm, 5) }()
	time.Sleep(1 * time.Millisecond) // allow time for first stream to register
	go func() { m.Manage(strm2, 4); wg.Done() }()
	time.Sleep(1 * time.Millisecond) // allow time for second stream to register

	assert.NotNil(m.liveTranscoders[strm])
	assert.NotNil(m.liveTranscoders[strm2])
	assert.Len(m.remoteTranscoders, 9)

	// assert transcoder is returned from selectTranscoder and removed from list
	t1 := m.liveTranscoders[strm]
	t2 := m.liveTranscoders[strm2]
	currentTranscoder := m.selectTranscoder()
	assert.Equal(currentTranscoder, t2)
	assert.NotNil(m.liveTranscoders[strm])
	assert.Len(m.remoteTranscoders, 8)

	// unregister transcoder
	t2.eof <- struct{}{}
	assert.True(wgWait(wg), "Wait timed out for transcoder to terminate")
	assert.Nil(m.liveTranscoders[strm2])
	assert.NotNil(m.liveTranscoders[strm])

	// assert t1 is selected and t2 drained
	currentTranscoder = m.selectTranscoder()
	assert.Equal(currentTranscoder, t1)
	assert.NotNil(m.liveTranscoders[strm])
	assert.Len(m.remoteTranscoders, 4)

	// assert transcoder gets added back to remoteTranscoders if no transcoding error
	_, err := m.Transcode("", nil)
	assert.Nil(err)
	assert.Len(m.remoteTranscoders, 4)
}

func TestTranscoderManagerTranscoding(t *testing.T) {
	m := NewRemoteTranscoderManager()
	s := &StubTranscoderServer{manager: m}

	// sanity checks
	assert := assert.New(t)
	assert.Empty(m.liveTranscoders)
	assert.Empty(m.remoteTranscoders)

	wg := newWg(1)
	go func() { m.Manage(s, 5); wg.Done() }()
	time.Sleep(1 * time.Millisecond)

	assert.Len(m.remoteTranscoders, 5) // sanity
	assert.Len(m.liveTranscoders, 1)
	assert.NotNil(m.liveTranscoders[s])

	// happy path
	res, err := m.Transcode("", nil)
	assert.Nil(err)
	assert.Len(res, 1)
	assert.Equal(string(res[0]), "asdf")

	// non-fatal error should not remove from list
	s.TranscodeError = fmt.Errorf("TranscodeError")
	_, err = m.Transcode("", nil)
	assert.Equal(s.TranscodeError, err)
	assert.Len(m.remoteTranscoders, 5) // sanity
	assert.Len(m.liveTranscoders, 1)
	assert.NotNil(m.liveTranscoders[s])
	s.TranscodeError = nil

	// fatal error should retry and remove from list
	s.SendError = fmt.Errorf("SendError")
	_, err = m.Transcode("", nil)
	assert.True(wgWait(wg)) // should disconnect manager
	assert.NotNil(err)
	assert.Equal(err.Error(), "No transcoders available")
	assert.Len(m.liveTranscoders, 0)
	assert.Len(m.remoteTranscoders, 0) // retries drain the list
	s.SendError = nil

	// fatal error should not retry
	wg.Add(1)
	go func() { m.Manage(s, 5); wg.Done() }()
	time.Sleep(1 * time.Millisecond)

	assert.Len(m.remoteTranscoders, 5) // sanity check
	assert.Len(m.liveTranscoders, 1)
	s.WithholdResults = true
	RemoteTranscoderTimeout = 1 * time.Millisecond
	_, err = m.Transcode("", nil)
	_, fatal := err.(RemoteTranscoderFatalError)
	wg.Wait()
	assert.True(fatal)
	assert.Len(m.liveTranscoders, 0)
	assert.Len(m.remoteTranscoders, 4) // no retries, so don't drain
	s.WithholdResults = false
	RemoteTranscoderTimeout = 8 * time.Second
}

func TestTaskChan(t *testing.T) {
	n := NewRemoteTranscoderManager()
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
	manager         *RemoteTranscoderManager
	SendError       error
	TranscodeError  error
	WithholdResults bool

	common.StubServerStream
}

func (s *StubTranscoderServer) Send(n *net.NotifySegment) error {
	res := RemoteTranscoderResult{Segments: [][]byte{[]byte("asdf")}, Err: s.TranscodeError}
	if !s.WithholdResults {
		s.manager.transcoderResults(n.TaskId, &res)
	}
	return s.SendError
}

func StubSegTranscodingMetadata() *SegTranscodingMetadata {
	return &SegTranscodingMetadata{
		ManifestID: ManifestID("abcdef"),
		Seq:        1234,
		Hash:       ethcommon.BytesToHash(ethcommon.RightPadBytes([]byte("browns"), 32)),
		Profiles:   []ffmpeg.VideoProfile{ffmpeg.P144p30fps16x9, ffmpeg.P240p30fps16x9},
		OS:         &net.OSInfo{StorageType: net.OSInfo_IPFS},
	}
}

func TestGetSegmentChan(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	segData := StubSegTranscodingMetadata()

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	sc, err := n.getSegmentChan(segData)
	if err != nil {
		t.Error("error with getSegmentChan", err)
	}

	if sc != n.SegmentChans[segData.ManifestID] {
		t.Error("SegmentChans mapping did not include channel")
	}

	// Test max sessions
	oldTranscodeSessions := MaxSessions
	MaxSessions = 0
	if _, err := n.getSegmentChan(segData); err != nil {
		t.Error("Existing mid should continue processing even when O is at capacity: ", err)
	}
	segData.ManifestID = ManifestID(t.Name())
	if _, err := n.getSegmentChan(segData); err != ErrOrchCap {
		t.Error("Didn't fail when orch cap hit: ", err)
	}
	MaxSessions = oldTranscodeSessions

	// Test what happens when invoking the transcode loop fails
	drivers.NodeStorage = nil // will make the transcode loop fail
	node, _ := NewLivepeerNode(nil, "", nil)

	sc, storageError := node.getSegmentChan(segData)
	if storageError.Error() != "Missing local storage" {
		t.Error("transcodingLoop did not fail when expected to", storageError)
	}

	if _, ok := node.SegmentChans[segData.ManifestID]; ok {
		t.Error("SegmentChans mapping included new channel when expected to return an err/nil")
	}

	// The following tests may seem identical to the two cases above
	// however, calling `getSegmentChan` used to hang on the invocation after an
	// error. Reproducing the scenario here but should not hang.
	sc, storageErr := node.getSegmentChan(segData)
	if storageErr.Error() != "Missing local storage" {
		t.Error("transcodingLoop did not fail when expected to", storageErr)
	}

	if _, ok := node.SegmentChans[segData.ManifestID]; ok {
		t.Error("SegmentChans mapping included new channel when expected to return an err/nil")
	}

}

func TestOrchCheckCapacity(t *testing.T) {

	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	n, _ := NewLivepeerNode(nil, "", nil)
	o := NewOrchestrator(n)
	md := StubSegTranscodingMetadata()
	cap := MaxSessions
	assert := assert.New(t)

	// happy case
	assert.Nil(o.CheckCapacity(md.ManifestID))

	// capped case
	MaxSessions = 0
	assert.Equal(ErrOrchCap, o.CheckCapacity(md.ManifestID))

	// ensure existing segment chans pass while cap is active
	MaxSessions = cap
	_, err := n.getSegmentChan(md) // store md into segment chans
	assert.Nil(err)
	MaxSessions = 0
	assert.Nil(o.CheckCapacity(md.ManifestID))
}

func TestProcessPayment_GivenRecipientError_ReturnsNil(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)

	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", false, nil)

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert := assert.New(t)
	assert.Nil(err)
}

func TestProcessPayment_GivenNoSender_ReturnsError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("some sessionID", false, nil)

	protoPayment := defaultPayment(t)

	protoPayment.Sender = nil

	err := orch.ProcessPayment(protoPayment, ManifestID("some manifest"))

	assert := assert.New(t)
	assert.Error(err)
}

func TestProcessPayment_GivenNilNode_ReturnsNilError(t *testing.T) {
	orch := &orchestrator{}

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert.Nil(t, err)
}

func TestProcessPayment_GivenNilRecipient_ReturnsNilError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	orch := NewOrchestrator(n)
	n.Recipient = nil

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert.Nil(t, err)
}

func TestProcessPayment_GivenLosingTicket_DoesNotRedeem(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("some sessionID", false, nil)

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	time.Sleep(time.Millisecond * 20)
	assert := assert.New(t)
	assert.Nil(err)
	recipient.AssertNotCalled(t, "RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessPayment_GivenWinningTicket_RedeemError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	sessionID := "some sessionID"
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID, true, nil)
	recipient.On("RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("RedeemWinningTicket error"))

	errorLogsBefore := glog.Stats.Error.Lines()

	err := orch.ProcessPayment(defaultPayment(t), manifestID)

	time.Sleep(time.Millisecond * 20)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert := assert.New(t)
	assert.Nil(err)
	assert.Equal(int64(1), errorLogsAfter-errorLogsBefore)
	recipient.AssertCalled(t, "RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessPayment_GivenWinningTicket_Redeems(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	sessionID := "some sessionID"
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID, true, nil)
	recipient.On("RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	errorLogsBefore := glog.Stats.Error.Lines()

	err := orch.ProcessPayment(defaultPayment(t), manifestID)

	time.Sleep(time.Millisecond * 20)
	errorLogsAfter := glog.Stats.Error.Lines()
	assert := assert.New(t)
	assert.Zero(errorLogsAfter - errorLogsBefore)
	assert.Nil(err)
	recipient.AssertCalled(t, "RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessPayment_GivenMultipleWinningTickets_RedeemsAll(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	sessionID := "some sessionID"
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID, true, nil)

	numTickets := 5
	recipient.On("RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(numTickets)

	var senderParams []*net.TicketSenderParams
	for i := 0; i < numTickets; i++ {
		senderParams = append(
			senderParams,
			&net.TicketSenderParams{SenderNonce: 456, Sig: pm.RandBytes(123)},
		)
	}

	err := orch.ProcessPayment(*defaultPaymentWithTickets(t, senderParams), manifestID)

	time.Sleep(time.Millisecond * 20)
	assert := assert.New(t)
	assert.Nil(err)
	recipient.AssertNumberOfCalls(t, "RedeemWinningTicket", numTickets)
}

func TestProcessPayment_GivenConcurrentWinningTickets_RedeemsAll(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		manifestIDs = append(manifestIDs, randString())
	}
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", true, nil)

	numTickets := 100
	recipient.On("RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(numTickets)

	assert := assert.New(t)

	var wg sync.WaitGroup
	wg.Add(len(manifestIDs))
	for i := 0; i < len(manifestIDs); i++ {
		go func(manifestID string) {
			var senderParams []*net.TicketSenderParams
			for i := 0; i < numTickets/len(manifestIDs); i++ {
				senderParams = append(
					senderParams,
					&net.TicketSenderParams{SenderNonce: 456, Sig: pm.RandBytes(123)},
				)
			}

			err := orch.ProcessPayment(*defaultPaymentWithTickets(t, senderParams), ManifestID(manifestID))
			assert.Nil(err)

			wg.Done()
		}(manifestIDs[i])
	}
	wg.Wait()

	time.Sleep(time.Millisecond * 20)
	recipient.AssertNumberOfCalls(t, "RedeemWinningTicket", numTickets)
}

func TestProcessPayment_GivenReceiveTicketError_ReturnsError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", false, errors.New("ReceiveTicket error")).Once()
	// This should trigger a redemption even though it returns an error because it still returns won = true
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", true, errors.New("not first error")).Once()
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", true, nil).Once()

	numTickets := 3
	recipient.On("RedeemWinningTicket", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(numTickets)

	var senderParams []*net.TicketSenderParams
	for i := 0; i < numTickets; i++ {
		senderParams = append(
			senderParams,
			&net.TicketSenderParams{SenderNonce: 456, Sig: pm.RandBytes(123)},
		)
	}

	err := orch.ProcessPayment(*defaultPaymentWithTickets(t, senderParams), manifestID)

	time.Sleep(time.Millisecond * 20)
	assert := assert.New(t)
	assert.EqualError(err, "error receiving tickets with payment")
	recipient.AssertNumberOfCalls(t, "RedeemWinningTicket", 2)
}

func TestTicketParams(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	expectedParams := &pm.TicketParams{
		Recipient:         pm.RandAddress(),
		FaceValue:         big.NewInt(1234),
		WinProb:           big.NewInt(2345),
		Seed:              big.NewInt(3456),
		RecipientRandHash: pm.RandHash(),
	}
	recipient.On("TicketParams", mock.Anything).Return(expectedParams, nil)
	orch := NewOrchestrator(n)

	assert := assert.New(t)

	actualParams, err := orch.TicketParams(pm.RandAddress())
	assert.Nil(err)

	assert.Equal(expectedParams.Recipient.Bytes(), actualParams.Recipient)
	assert.Equal(expectedParams.FaceValue.Bytes(), actualParams.FaceValue)
	assert.Equal(expectedParams.WinProb.Bytes(), actualParams.WinProb)
	assert.Equal(expectedParams.RecipientRandHash.Bytes(), actualParams.RecipientRandHash)
	assert.Equal(expectedParams.Seed.Bytes(), actualParams.Seed)
}

func TestTicketParams_GivenNilNode_ReturnsNil(t *testing.T) {
	orch := &orchestrator{}

	params, err := orch.TicketParams(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Nil(t, params)
}

func TestTicketParams_GivenNilRecipient_ReturnsNil(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	orch := NewOrchestrator(n)
	n.Recipient = nil

	params, err := orch.TicketParams(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Nil(t, params)
}

func TestTicketParams_Error(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	expErr := errors.New("TicketParams error")
	recipient.On("TicketParams", mock.Anything).Return(nil, expErr)
	orch := NewOrchestrator(n)

	_, err := orch.TicketParams(ethcommon.Address{})
	assert.EqualError(t, err, expErr.Error())
}

func TestPriceInfo_ReturnsBigRat(t *testing.T) {
	// basePrice = 1/1, txMultiplier = 100/1 => expPricePerPixel = 101/100
	basePrice := big.NewRat(1, 1)
	txMultiplier := big.NewRat(100, 1)
	expPricePerPixel := big.NewRat(101, 100)

	n, _ := NewLivepeerNode(nil, "", nil)
	n.PriceInfo = basePrice

	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	recipient.On("TxCostMultiplier", mock.Anything).Return(txMultiplier, nil)
	orch := NewOrchestrator(n)

	priceInfo, err := orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 10/1, txMultiplier = 100/1 => expPricePerPixel = 1010/100
	basePrice = big.NewRat(10, 1)
	n.PriceInfo = basePrice
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(1010, 100)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 1/10, txMultiplier = 100 => expPricePerPixel = 101/1000
	basePrice = big.NewRat(1, 10)
	n.PriceInfo = basePrice
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(101, 1000)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 25/10 , txMultiplier = 100 => expPricePerPixel = 2525/1000
	basePrice = big.NewRat(25, 10)
	n.PriceInfo = basePrice
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(2525, 1000)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 10/1 , txMultiplier = 100/10 => expPricePerPixel = 11
	basePrice = big.NewRat(10, 1)
	txMultiplier = big.NewRat(100, 10)
	n.PriceInfo = basePrice
	recipient = new(pm.MockRecipient)
	n.Recipient = recipient
	recipient.On("TxCostMultiplier", mock.Anything).Return(txMultiplier, nil)
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(11, 1)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 10/1 , txMultiplier = 1/10 => expPricePerPixel = 110
	basePrice = big.NewRat(10, 1)
	txMultiplier = big.NewRat(1, 10)
	n.PriceInfo = basePrice
	recipient = new(pm.MockRecipient)
	n.Recipient = recipient
	recipient.On("TxCostMultiplier", mock.Anything).Return(txMultiplier, nil)
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(1100, 10)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))

	// basePrice = 10, txMultiplier = 1 => expPricePerPixel = 20
	basePrice = big.NewRat(10, 1)
	txMultiplier = big.NewRat(1, 1)
	n.PriceInfo = basePrice
	recipient = new(pm.MockRecipient)
	n.Recipient = recipient
	recipient.On("TxCostMultiplier", mock.Anything).Return(txMultiplier, nil)
	orch = NewOrchestrator(n)
	expPricePerPixel = big.NewRat(20, 1)

	priceInfo, err = orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Zero(t, expPricePerPixel.Cmp(priceInfo))
}

func TestPriceInfo_GivenNilNode_ReturnsNilError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	orch := NewOrchestrator(n)
	orch.node = nil

	priceInfo, err := orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Nil(t, priceInfo)
}

func TestPriceInfo_GivenNilRecipient_ReturnsNilError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	orch := NewOrchestrator(n)
	n.Recipient = nil

	priceInfo, err := orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, err)
	assert.Nil(t, priceInfo)
}

func TestPriceInfo_TxMultiplierError_ReturnsError(t *testing.T) {
	expError := errors.New("TxMultiplier Error")

	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	recipient.On("TxCostMultiplier", mock.Anything).Return(nil, expError)
	orch := NewOrchestrator(n)

	priceInfo, err := orch.PriceInfo(ethcommon.Address{})
	assert.Nil(t, priceInfo)
	assert.EqualError(t, err, expError.Error())
}

func defaultPayment(t *testing.T) net.Payment {
	ticketSenderParams := &net.TicketSenderParams{
		SenderNonce: 456,
		Sig:         pm.RandBytes(123),
	}

	return *defaultPaymentWithTickets(t, []*net.TicketSenderParams{ticketSenderParams})
}

func defaultPaymentWithTickets(t *testing.T, senderParams []*net.TicketSenderParams) *net.Payment {
	ticketParams := &net.TicketParams{
		Recipient:         pm.RandBytes(123),
		FaceValue:         pm.RandBytes(123),
		WinProb:           pm.RandBytes(123),
		RecipientRandHash: pm.RandBytes(123),
		Seed:              pm.RandBytes(123),
	}

	sender := pm.RandBytes(123)
	expirationParams := &net.TicketExpirationParams{
		CreationRound:          5,
		CreationRoundBlockHash: []byte{5},
	}

	payment := &net.Payment{
		TicketParams:       ticketParams,
		Sender:             sender,
		ExpirationParams:   expirationParams,
		TicketSenderParams: senderParams,
	}
	return payment
}

func defaultTicket(t *testing.T) *net.Ticket {
	return &net.Ticket{
		Recipient:         pm.RandBytes(123),
		Sender:            pm.RandBytes(123),
		FaceValue:         pm.RandBytes(123),
		WinProb:           pm.RandBytes(123),
		SenderNonce:       456,
		RecipientRandHash: pm.RandBytes(123),
	}
}

func randString() string {
	x := make([]byte, 42)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x", x)
}
