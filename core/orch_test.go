package core

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/livepeer/go-livepeer/pm"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/lpms/ffmpeg"

	"github.com/livepeer/go-livepeer/net"

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

func TestServeTranscoder(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	strm := &StubTranscoderServer{}

	// test that a transcoder was created
	go n.serveTranscoder(strm, 5)
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
		cap := 5
		tc := NewRemoteTranscoder(n, strm, cap)
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

func TestProcessPayment_GivenRecipientError_ReturnsError(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("", false, errors.New("mock error"))

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert.Contains(t, err.Error(), "mock error")
}

func TestProcessPayment_GivenWinningTicketAndRecipientError_DoesNotCacheSessionID(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("some sessionID", true, errors.New("mock error"))

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert := assert.New(t)
	assert.NotNil(err)
	assert.Empty(n.pmSessions)
}

func TestProcessPayment_GivenLosingTicket_DoesNotCacheSessionID(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return("some sessionID", false, nil)

	err := orch.ProcessPayment(defaultPayment(t), ManifestID("some manifest"))

	assert := assert.New(t)
	assert.Nil(err)
	assert.Empty(n.pmSessions)
}

func TestProcessPayment_GivenWinningTicket_CachesSessionID(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	sessionID := "some sessionID"
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID, true, nil)

	err := orch.ProcessPayment(defaultPayment(t), manifestID)

	assert := assert.New(t)
	assert.Nil(err)
	assert.Contains(n.pmSessions, manifestID)
	assert.Contains(n.pmSessions[manifestID], sessionID)
}

func TestProcessPayment_GivenWinningTicketsInMultipleSessions_CachesAllSessionIDs(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestID := ManifestID("some manifest")
	sessionID0 := "first sessionID"
	sessionID1 := "second sessionID"
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID0, true, nil).Once()
	recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionID1, true, nil).Once()
	assert := assert.New(t)

	err := orch.ProcessPayment(defaultPayment(t), manifestID)
	assert.Nil(err)
	err = orch.ProcessPayment(defaultPayment(t), manifestID)
	assert.Nil(err)

	assert.Contains(n.pmSessions, manifestID)
	assert.Contains(n.pmSessions[manifestID], sessionID0)
	assert.Contains(n.pmSessions[manifestID], sessionID1)
}

func TestProcessPayment_GivenConcurrentWinningTickets_CachesAllSessionIDs(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	recipient := new(pm.MockRecipient)
	n.Recipient = recipient
	orch := NewOrchestrator(n)
	manifestIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		manifestIDs = append(manifestIDs, randString())
	}
	sessionIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		sessionIDs[i] = randString()
	}
	for i := 0; i < len(sessionIDs); i++ {
		recipient.On("ReceiveTicket", mock.Anything, mock.Anything, mock.Anything).Return(sessionIDs[i], true, nil).Once()
	}

	var wg sync.WaitGroup
	wg.Add(len(sessionIDs))
	for i := 0; i < len(sessionIDs); i++ {
		manifestID := manifestIDs[rand.Intn(len(manifestIDs))]
		go func() {
			orch.ProcessPayment(defaultPayment(t), ManifestID(manifestID))
			wg.Done()
		}()
	}
	wg.Wait()

	actualSessionIDs := make([]string, len(sessionIDs))
	i := 0
	for _, sessionsMap := range n.pmSessions {
		for sessionID := range sessionsMap {
			actualSessionIDs[i] = sessionID
			i++
		}
	}
	assert := assert.New(t)
	assert.ElementsMatch(sessionIDs, actualSessionIDs)
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
	recipient.On("TicketParams", mock.Anything).Return(expectedParams)
	orch := NewOrchestrator(n)

	actualParams := orch.TicketParams(pm.RandAddress())

	assert := assert.New(t)
	assert.Equal(expectedParams.Recipient.Bytes(), actualParams.Recipient)
	assert.Equal(expectedParams.FaceValue.Bytes(), actualParams.FaceValue)
	assert.Equal(expectedParams.WinProb.Bytes(), actualParams.WinProb)
	assert.Equal(expectedParams.RecipientRandHash.Bytes(), actualParams.RecipientRandHash)
	assert.Equal(expectedParams.Seed.Bytes(), actualParams.Seed)
}

func TestTicketParams_GivenNilNode_ReturnsNil(t *testing.T) {
	orch := &orchestrator{}

	params := orch.TicketParams(ethcommon.Address{})

	assert.Nil(t, params)
}

func TestTicketParams_GivenNilRecipient_ReturnsNil(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	orch := NewOrchestrator(n)
	n.Recipient = nil

	params := orch.TicketParams(ethcommon.Address{})

	assert.Nil(t, params)
}

func defaultPayment(t *testing.T) net.Payment {
	ticket := &net.Ticket{
		Recipient:         pm.RandBytes(123),
		Sender:            pm.RandBytes(123),
		FaceValue:         pm.RandBytes(123),
		WinProb:           pm.RandBytes(123),
		SenderNonce:       456,
		RecipientRandHash: pm.RandBytes(123),
	}
	return net.Payment{
		Ticket: ticket,
		Sig:    pm.RandBytes(123),
		Seed:   pm.RandBytes(123),
	}
}

func randString() string {
	x := make([]byte, 42)
	for i := 0; i < len(x); i++ {
		x[i] = byte(rand.Uint32())
	}
	return fmt.Sprintf("%x", x)
}
