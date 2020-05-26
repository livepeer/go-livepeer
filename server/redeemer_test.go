package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/peer"
)

var baseRPCErr = "rpc error: code = Internal desc = "

// Test server
func TestRedeemerServer_NewRedeemer(t *testing.T) {
	assert := assert.New(t)
	// test no recipient
	r, err := NewRedeemer(ethcommon.Address{}, nil, nil)
	assert.Nil(r)
	assert.EqualError(err, "must provide a recipient")

	// test no LivepeerEthClient
	r, err = NewRedeemer(pm.RandAddress(), nil, nil)
	assert.Nil(r)
	assert.EqualError(err, "must provide a LivepeerEthClient")

	// test no SenderMonitor
	r, err = NewRedeemer(pm.RandAddress(), &eth.StubClient{}, nil)
	assert.Nil(r)
	assert.EqualError(err, "must provide a SenderMonitor")

	recipient := pm.RandAddress()
	r, err = NewRedeemer(recipient, &eth.StubClient{}, &stubSenderMonitor{})
	assert.Nil(err)
	assert.Equal(r.(*redeemer).recipient, recipient)
}

func TestRedeemerServer_QueueTicket(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	sm.maxFloat = big.NewInt(100)
	r, err := NewRedeemer(recipient, eth, sm)
	require.Nil(err)

	ticket := &net.Ticket{
		Recipient:              recipient.Bytes(),
		Sender:                 pm.RandAddress().Bytes(),
		FaceValue:              big.NewInt(100).Bytes(),
		WinProb:                big.NewInt(100).Bytes(),
		SenderNonce:            1,
		CreationRound:          100,
		CreationRoundBlockHash: pm.RandHash().Bytes(),
		Sig:                    pm.RandBytes(32),
		RecipientRand:          big.NewInt(1337).Bytes(),
		ParamsExpirationBlock:  100,
	}

	sm.shouldFail = errors.New("QueueTicket error")
	res, err := r.QueueTicket(context.Background(), ticket)
	assert.Nil(res)
	assert.EqualError(err, baseRPCErr+sm.shouldFail.Error())
	sm.shouldFail = nil

	res, err = r.QueueTicket(context.Background(), ticket)
	assert.Nil(err)
	assert.Equal(res, &empty.Empty{})
	assert.Equal(sm.queued[0], pmTicket(ticket))

	// check that monitorMaxFloat is called
	time.Sleep(time.Millisecond)
	_, ok := r.(*redeemer).liveSenders.Load(ethcommon.BytesToAddress(ticket.Sender))
	assert.True(ok)
}

func TestRedeemerServer_MonitorMaxFloat(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Set up a mock context to be returned from the stream server
	// Client subcriptions are stored based on their peer address
	stubPeer := &peer.Peer{
		Addr: &stubAddr{
			addr: "foo",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = peer.NewContext(ctx, stubPeer)
	ctrl, ctx := gomock.WithContext(ctx, t)
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx)
	p, ok := peer.FromContext(stream.Context())
	require.True(ok)
	require.Equal(p.Addr.String(), stubPeer.Addr.String())

	// Set up empty request
	req := &empty.Empty{}

	// Set up redeemer server
	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	r, err := NewRedeemer(recipient, eth, sm)
	require.Nil(err)

	// Test no context on call (sanity check)
	stream.EXPECT().Context().Return(context.TODO())
	err = r.MonitorMaxFloat(req, stream)
	assert.EqualError(err, baseRPCErr+"context is nil")

	// Test wrong channel type (sanity check)
	r.(*redeemer).subs.Store(stubPeer.Addr.String(), struct{}{})
	stream.EXPECT().Context().Return(ctx).AnyTimes()
	err = r.MonitorMaxFloat(req, stream)
	assert.EqualError(err, baseRPCErr+"maxFloatUpdates is of the wrong type")

	errC := make(chan error)

	// Create a channel for receive updates
	// To be sent into the stream for the subscriber
	maxFloatUpdates := make(chan *net.MaxFloatUpdate)
	r.(*redeemer).subs.Store(stubPeer.Addr.String(), maxFloatUpdates)

	// Test send error - io.EOF
	expErr := io.EOF
	stream.EXPECT().Send(gomock.Any()).Return(expErr)

	go func() {
		errC <- r.MonitorMaxFloat(req, stream)
	}()
	time.Sleep(time.Millisecond)

	maxFloatUpdates <- &net.MaxFloatUpdate{}
	err = <-errC
	assert.EqualError(err, baseRPCErr+expErr.Error())
	_, ok = r.(*redeemer).subs.Load(stubPeer.Addr.String())
	assert.False(ok)

	// Test send error - not io.EOF
	// Re-add channel since io.EOF error removed it from the subs list
	r.(*redeemer).subs.Store(stubPeer.Addr.String(), maxFloatUpdates)
	expErr = errors.New("send error")
	stream.EXPECT().Send(gomock.Any()).Return(expErr)
	go func() {
		errC <- r.MonitorMaxFloat(req, stream)
	}()
	time.Sleep(time.Millisecond)

	errLogsBefore := glog.Stats.Error.Lines()
	maxFloatUpdates <- &net.MaxFloatUpdate{}
	// clean up the current goroutine and test quit case
	close(r.(*redeemer).quit)
	err = <-errC
	assert.Nil(err)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(errLogsAfter-errLogsBefore, int64(1))
	_, ok = r.(*redeemer).subs.Load(stubPeer.Addr.String())
	assert.True(ok)
	// instantiate a new quit channel since we closed the original one
	r.(*redeemer).quit = make(chan struct{})

	// Test no error
	require.Nil(err)
	expErr = nil
	stream.EXPECT().Send(gomock.Any()).Return(nil)
	go func() {
		errC <- r.MonitorMaxFloat(req, stream)
	}()
	time.Sleep(time.Millisecond)
	errLogsBefore = glog.Stats.Error.Lines()

	maxFloatUpdates <- &net.MaxFloatUpdate{}
	// clean up the current goroutine and test quit case
	close(r.(*redeemer).quit)
	err = <-errC
	assert.Nil(err)
	errLogsAfter = glog.Stats.Error.Lines()
	assert.Equal(errLogsAfter-errLogsBefore, int64(0))

	_, ok = r.(*redeemer).subs.Load(stubPeer.Addr.String())
	assert.True(ok)
	// instantiate a new quit channel since we closed the original one
	r.(*redeemer).quit = make(chan struct{})

	// test context.Done()
	stream.EXPECT().Send(gomock.Any()).Return(nil)
	go func() {
		errC <- r.MonitorMaxFloat(req, stream)
	}()
	time.Sleep(time.Millisecond)
	cancel()
	err = <-errC
	assert.Nil(err)
}

func TestRedeemerServer_CleanupLoop(t *testing.T) {
	assert := assert.New(t)
	r := &redeemer{
		quit: make(chan struct{}),
	}
	oldCleanupTime := cleanupLoopTime
	cleanupLoopTime = 3 * time.Millisecond
	defer func() {
		cleanupLoopTime = oldCleanupTime
	}()
	sender0 := ethcommon.HexToAddress("foo")
	sender1 := ethcommon.HexToAddress("bar")
	sender2 := ethcommon.HexToAddress("caz")
	r.liveSenders.Store(sender0, time.Now())
	r.liveSenders.Store(sender1, time.Now().Add(cleanupLoopTime))
	r.liveSenders.Store(sender2, time.Now().Add(-cleanupLoopTime))
	// cleanupLoop will run after 'cleanupLoopTime' so
	// sender0 and sender1 will not be cleared
	// sender2 will be cleared
	go r.startCleanupLoop()
	time.Sleep(cleanupLoopTime)
	close(r.quit)
	r.quit = make(chan struct{})
	_, ok := r.liveSenders.Load(sender0)
	assert.True(ok)
	_, ok = r.liveSenders.Load(sender1)
	assert.True(ok)
	_, ok = r.liveSenders.Load(sender2)
	assert.False(ok)
	// on the next iteration sender0 will be cleared
	go r.startCleanupLoop()
	time.Sleep(cleanupLoopTime)
	// avoid failing test due to timing
	r.liveSenders.Store(sender1, time.Now())
	close(r.quit)
	_, ok = r.liveSenders.Load(sender1)
	assert.True(ok)
	_, ok = r.liveSenders.Load(sender0)
	assert.False(ok)
}

// Test client
func TestRedeemerClient_MonitorMaxFloat_RequestErr(t *testing.T) {
	assert := assert.New(t)

	sender := ethcommon.HexToAddress("foo")
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}

	ctx := context.TODO()

	// rpc.MonitorMaxFloat error
	rpc.EXPECT().MonitorMaxFloat(ctx, gomock.Any()).Return(nil, errors.New("error")).Times(1)
	errLogsBefore := glog.Stats.Error.Lines()
	rc.monitorMaxFloat(ctx)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(errLogsAfter-errLogsBefore, int64(1))
	// check that maxfloat didn't change
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Nil(mf)
}

func TestRedeemerClient_MonitorMaxFloat_StreamRecvErr(t *testing.T) {
	assert := assert.New(t)

	sender := ethcommon.HexToAddress("foo")
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}
	// clean up go routine
	defer rc.Stop()

	ctx := context.TODO()
	// stream.Recv error
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)
	rpc.EXPECT().MonitorMaxFloat(ctx, gomock.Any()).Return(stream, nil).Times(1)
	stream.EXPECT().Recv().Return(nil, errors.New("error")).AnyTimes()
	errLogsBefore := glog.Stats.Error.Lines()
	go rc.monitorMaxFloat(ctx)
	time.Sleep(5 * time.Millisecond)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Greater(errLogsAfter-errLogsBefore, int64(0))
	// check that maxfloat didn't change
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Nil(mf)
}

func TestRedeemerClient_MonitorMaxFloat_ContextDone(t *testing.T) {
	assert := assert.New(t)

	sender := ethcommon.HexToAddress("foo")
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)
	rpc.EXPECT().MonitorMaxFloat(ctx, gomock.Any()).Return(stream, nil).Times(1)
	stream.EXPECT().Recv().Return(&net.MaxFloatUpdate{Sender: (ethcommon.Address{}).Bytes(), MaxFloat: big.NewInt(10).Bytes()}, nil).AnyTimes()
	errLogsBefore := glog.Stats.Info.Lines()
	go rc.monitorMaxFloat(ctx)
	time.Sleep(time.Millisecond)
	cancel()
	time.Sleep(time.Millisecond)
	errLogsAfter := glog.Stats.Info.Lines()
	assert.Greater(errLogsAfter-errLogsBefore, int64(0))
	// check that maxfloat didn't change
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Nil(mf)
}

func TestRedeemerClient_MonitorMaxFloat_UpdateMaxFloat(t *testing.T) {
	assert := assert.New(t)

	sender := ethcommon.HexToAddress("foo")
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}
	// clean up go routine
	defer rc.Stop()

	ctx := context.TODO()
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)
	rpc.EXPECT().MonitorMaxFloat(ctx, gomock.Any()).Return(stream, nil).Times(1)
	stream.EXPECT().Recv().Return(&net.MaxFloatUpdate{Sender: sender.Bytes(), MaxFloat: big.NewInt(10).Bytes()}, nil).AnyTimes()
	go rc.monitorMaxFloat(ctx)
	time.Sleep(time.Millisecond)
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Equal(mf, big.NewInt(10))
}

func TestRedeemerClient_QueueTicket_RPCErr(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}

	ticket := &net.Ticket{
		Recipient:              pm.RandAddress().Bytes(),
		Sender:                 pm.RandAddress().Bytes(),
		FaceValue:              big.NewInt(100).Bytes(),
		WinProb:                big.NewInt(100).Bytes(),
		SenderNonce:            1,
		CreationRound:          100,
		CreationRoundBlockHash: pm.RandHash().Bytes(),
		Sig:                    pm.RandBytes(32),
		RecipientRand:          big.NewInt(1337).Bytes(),
		ParamsExpirationBlock:  100,
	}

	rpc.EXPECT().QueueTicket(gomock.Any(), gomock.Any()).Return(nil, errors.New("QueueTicket error"))
	assert.EqualError(rc.QueueTicket(pmTicket(ticket)), "QueueTicket error")
}

func TestRedeemerClient_QueueTicket_Success(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &redeemerClient{
		rpc:      rpc,
		maxFloat: make(map[ethcommon.Address]*big.Int),
		quit:     make(chan struct{}),
	}

	ticket := &net.Ticket{
		Recipient:              pm.RandAddress().Bytes(),
		Sender:                 pm.RandAddress().Bytes(),
		FaceValue:              big.NewInt(100).Bytes(),
		WinProb:                big.NewInt(100).Bytes(),
		SenderNonce:            1,
		CreationRound:          100,
		CreationRoundBlockHash: pm.RandHash().Bytes(),
		Sig:                    pm.RandBytes(32),
		RecipientRand:          big.NewInt(1337).Bytes(),
		ParamsExpirationBlock:  100,
	}

	rpc.EXPECT().QueueTicket(gomock.Any(), gomock.Any()).Return(nil, nil)
	err := rc.QueueTicket(pmTicket(ticket))
	assert.Nil(err)
}

func TestRedeemerClient_MaxFloat(t *testing.T) {
	assert := assert.New(t)
	rc := &redeemerClient{
		maxFloat: make(map[ethcommon.Address]*big.Int),
	}
	sender := pm.RandAddress()
	rc.maxFloat[sender] = big.NewInt(100)
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Equal(mf, big.NewInt(100))
}

func TestRedeemerClient_ValidateSender_SenderInfoErr(t *testing.T) {
	assert := assert.New(t)
	rc := &redeemerClient{
		sm: newStubSenderManager(),
	}
	rc.sm.(*stubSenderManager).err = errors.New("GetSenderInfo error")
	assert.True(strings.Contains(rc.ValidateSender(pm.RandAddress()).Error(), "GetSenderInfo error"))
}

func TestRedeemerClient_ValidateSender_MaxWithDrawRoundErr(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	rc := &redeemerClient{
		sm: newStubSenderManager(),
		tm: &stubTimeManager{
			round: big.NewInt(10),
		},
	}
	sender := pm.RandAddress()
	rc.sm.(*stubSenderManager).info[sender] = &pm.SenderInfo{
		WithdrawRound: big.NewInt(10),
	}
	info, err := rc.sm.GetSenderInfo(sender)
	require.Nil(err)
	require.Equal(info.WithdrawRound, big.NewInt(10))

	err = rc.ValidateSender(sender)
	assert.EqualError(err, fmt.Sprintf("deposit and reserve for sender %v is set to unlock soon", sender.Hex()))
}

func TestRedeemerClient_ValidateSender_Success(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	rc := &redeemerClient{
		sm: newStubSenderManager(),
		tm: &stubTimeManager{
			round: big.NewInt(0),
		},
	}
	sender := pm.RandAddress()
	rc.sm.(*stubSenderManager).info[sender] = &pm.SenderInfo{
		WithdrawRound: big.NewInt(10),
	}
	info, err := rc.sm.GetSenderInfo(sender)
	require.Nil(err)
	require.Equal(info.WithdrawRound, big.NewInt(10))

	assert.Nil(rc.ValidateSender(sender))
}

// Stubs

type stubSenderMonitor struct {
	maxFloat   *big.Int
	queued     []*pm.SignedTicket
	shouldFail error
	sink       chan<- *big.Int
	sub        *stubSubscription
}

func newStubSenderMonitor() *stubSenderMonitor {
	return &stubSenderMonitor{
		maxFloat: big.NewInt(0),
	}
}

func (s *stubSenderMonitor) Start() {}

func (s *stubSenderMonitor) Stop() {}

func (s *stubSenderMonitor) QueueTicket(ticket *pm.SignedTicket) error {
	if s.shouldFail != nil {
		return s.shouldFail
	}
	s.queued = append(s.queued, ticket)
	return nil
}

func (s *stubSenderMonitor) MaxFloat(addr ethcommon.Address) (*big.Int, error) {
	if s.shouldFail != nil {
		return nil, s.shouldFail
	}

	return s.maxFloat, nil
}

func (s *stubSenderMonitor) ValidateSender(addr ethcommon.Address) error { return s.shouldFail }

func (s *stubSenderMonitor) MonitorMaxFloat(sender ethcommon.Address, sink chan<- *big.Int) event.Subscription {
	s.sink = sink
	s.sub = &stubSubscription{errCh: make(<-chan error)}
	return s.sub
}

type stubSubscription struct {
	errCh        <-chan error
	unsubscribed bool
}

func (s *stubSubscription) Unsubscribe() {
	s.unsubscribed = true
}

func (s *stubSubscription) Err() <-chan error {
	return s.errCh
}

// stubAddr stubs the peer.Addr interface
type stubAddr struct {
	network string
	addr    string
}

func (a *stubAddr) Network() string {
	return a.network
}

func (a *stubAddr) String() string {
	return a.addr
}

type stubSenderManager struct {
	info           map[ethcommon.Address]*pm.SenderInfo
	claimedReserve map[ethcommon.Address]*big.Int
	err            error
}

func newStubSenderManager() *stubSenderManager {
	return &stubSenderManager{
		info:           make(map[ethcommon.Address]*pm.SenderInfo),
		claimedReserve: make(map[ethcommon.Address]*big.Int),
	}
}

func (s *stubSenderManager) GetSenderInfo(addr ethcommon.Address) (*pm.SenderInfo, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.info[addr], nil
}

func (s *stubSenderManager) ClaimedReserve(reserveHolder ethcommon.Address, claimant ethcommon.Address) (*big.Int, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.claimedReserve[reserveHolder], nil
}

func (s *stubSenderManager) Clear(addr ethcommon.Address) {
	delete(s.info, addr)
	delete(s.claimedReserve, addr)
}

type stubTimeManager struct {
	round              *big.Int
	blkHash            [32]byte
	transcoderPoolSize *big.Int
	lastSeenBlock      *big.Int

	blockNumSink chan<- *big.Int
	blockNumSub  event.Subscription
}

func (m *stubTimeManager) LastInitializedRound() *big.Int {
	return m.round
}

func (m *stubTimeManager) LastInitializedBlockHash() [32]byte {
	return m.blkHash
}

func (m *stubTimeManager) GetTranscoderPoolSize() *big.Int {
	return m.transcoderPoolSize
}

func (m *stubTimeManager) LastSeenBlock() *big.Int {
	return m.lastSeenBlock
}

func (m *stubTimeManager) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	return &stubSubscription{}
}

func (m *stubTimeManager) SubscribeBlocks(sink chan<- *big.Int) event.Subscription {
	m.blockNumSink = sink
	m.blockNumSub = &stubSubscription{errCh: make(<-chan error)}
	return m.blockNumSub
}
