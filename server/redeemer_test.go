package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/golang/mock/gomock"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	r, err = NewRedeemer(recipient, &eth.StubClient{}, &pm.LocalSenderMonitor{})
	assert.Nil(err)
	assert.Equal(r.recipient, recipient)
}

func TestRedeemerServer_Start(t *testing.T) {
	require := require.New(t)

	r, err := NewRedeemer(ethcommon.BytesToAddress([]byte("foo")), &eth.StubClient{}, &pm.LocalSenderMonitor{})
	require.Nil(err)

	url, err := url.ParseRequestURI("https://127.0.0.1:8990")
	require.NoError(err)

	tmpdir := t.TempDir()

	errCh := make(chan error)
	go func() {
		errCh <- r.Start(url, tmpdir)
	}()

	time.Sleep(20 * time.Millisecond)

	// Check that client can connect to server
	_, err = NewRedeemerClient(url.Host, newStubSenderManager(), &stubTimeManager{})
	require.NoError(err)

	r.Stop()
	require.NoError(<-errCh)
}

func TestRedeemerServer_QueueTicket(t *testing.T) {
	assert := assert.New(t)

	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	sm.maxFloat = big.NewInt(100)
	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	ticket := &net.Ticket{
		Sender:        pm.RandAddress().Bytes(),
		RecipientRand: big.NewInt(1337).Bytes(),
		TicketParams: &net.TicketParams{
			Recipient:         recipient.Bytes(),
			FaceValue:         big.NewInt(100).Bytes(),
			WinProb:           big.NewInt(100).Bytes(),
			RecipientRandHash: pm.RandBytes(32),
			ExpirationBlock:   big.NewInt(100).Bytes(),
		},
		SenderParams: &net.TicketSenderParams{
			Sig:         pm.RandBytes(32),
			SenderNonce: 1,
		},
		ExpirationParams: &net.TicketExpirationParams{
			CreationRound:          100,
			CreationRoundBlockHash: pm.RandHash().Bytes(),
		},
	}

	sm.shouldFail = errors.New("QueueTicket error")
	res, err := r.QueueTicket(context.Background(), ticket)
	assert.Nil(res)
	assert.EqualError(err, baseRPCErr+sm.shouldFail.Error())
	sm.shouldFail = nil

	res, err = r.QueueTicket(context.Background(), ticket)
	assert.Nil(err)
	assert.Equal(res, &net.QueueTicketRes{})
	assert.Equal(sm.queued[0], pmTicket(ticket))

	// test wrong recipient
	ticket.TicketParams.Recipient = pm.RandAddress().Bytes()
	res, err = r.QueueTicket(context.Background(), ticket)
	assert.Nil(res)
	assert.EqualError(err, fmt.Sprintf("rpc error: code = PermissionDenied desc = invalid ticket recipient 0x%x, expected %v", ticket.TicketParams.Recipient, r.recipient.Hex()))
}

func TestRedeemerServer_MonitorMaxFloat_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), GRPCConnectTimeout)
	defer cancel()
	ctrl, ctx := gomock.WithContext(ctx, t)
	defer ctrl.Finish()

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx).AnyTimes()

	// Set up a request
	sender := pm.RandAddress()
	req := &net.MaxFloatReq{Sender: sender.Bytes()}

	// Set up redeemer server
	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	maxFloat := big.NewInt(100)
	sm.maxFloat = maxFloat

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	errC := make(chan error)
	stream.EXPECT().Send(&net.MaxFloatUpdate{MaxFloat: maxFloat.Bytes()}).Return(nil).Times(1)

	timer := time.NewTimer(1 * time.Second)
	go func() {
		err := r.MonitorMaxFloat(req, stream)
		errC <- err
	}()
	<-sm.subscribed
	sm.sink <- struct{}{}
	select {
	case <-errC:
		t.Fail()
	case <-timer.C:
		return
	}
}

func TestRedeemerServer_MonitorMaxFloat_MaxFloatErr(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), GRPCConnectTimeout)
	defer cancel()
	ctrl, ctx := gomock.WithContext(ctx, t)
	defer ctrl.Finish()

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx).AnyTimes()

	sender := pm.RandAddress()
	req := &net.MaxFloatReq{Sender: sender.Bytes()}

	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	sm.shouldFail = errors.New("MaxFloat error")

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}
	errLogsBefore := glog.Stats.Error.Lines()
	go func() {
		r.MonitorMaxFloat(req, stream)
	}()
	<-sm.subscribed
	sm.sink <- struct{}{}
	time.Sleep(200 * time.Millisecond)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(int64(1), errLogsAfter-errLogsBefore)
}

func TestRedeemerServer_MonitorMaxFloat_SendErr(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), GRPCConnectTimeout)
	defer cancel()
	ctrl, ctx := gomock.WithContext(ctx, t)
	defer ctrl.Finish()

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx).AnyTimes()

	sender := pm.RandAddress()
	req := &net.MaxFloatReq{Sender: sender.Bytes()}

	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	maxFloat := big.NewInt(100)
	sm.maxFloat = maxFloat

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	errC := make(chan error)
	stream.EXPECT().Send(gomock.Any()).Return(errors.New("error")).Times(1)

	go func() {
		err := r.MonitorMaxFloat(req, stream)
		errC <- err
	}()
	<-sm.subscribed
	sm.sink <- struct{}{}
	err := <-errC
	assert.EqualError(err, "rpc error: code = Internal desc = error")
	assert.True(sm.sub.unsubscribed)
}

func TestRedeemerServer_MonitorMaxFloat_SubErr(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), GRPCConnectTimeout)
	defer cancel()
	ctrl, ctx := gomock.WithContext(ctx, t)
	defer ctrl.Finish()

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx).AnyTimes()

	sender := pm.RandAddress()
	req := &net.MaxFloatReq{Sender: sender.Bytes()}

	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	maxFloat := big.NewInt(100)
	sm.maxFloat = maxFloat

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	errC := make(chan error)

	go func() {
		errC <- r.MonitorMaxFloat(req, stream)
	}()
	<-sm.subscribed
	sm.sub.errCh <- nil
	time.Sleep(time.Second)
	err := <-errC
	assert.EqualError(err, "rpc error: code = Canceled desc = subscription closed")
}

func TestRedeemerServer_MonitorMaxFloat_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ctrl, ctx := gomock.WithContext(ctx, t)
	defer ctrl.Finish()

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatServer(ctrl)

	stream.EXPECT().Context().Return(ctx).AnyTimes()

	// Set up a request
	sender := pm.RandAddress()
	req := &net.MaxFloatReq{Sender: sender.Bytes()}

	// Set up redeemer server
	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	maxFloat := big.NewInt(100)
	sm.maxFloat = maxFloat

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	errC := make(chan error)
	stream.EXPECT().Send(&net.MaxFloatUpdate{MaxFloat: maxFloat.Bytes()}).Return(nil).Times(1)

	timer := time.NewTimer(2 * time.Second)
	go func() {
		err := r.MonitorMaxFloat(req, stream)
		errC <- err
	}()
	<-sm.subscribed
	sm.sink <- struct{}{}
	select {
	case err := <-errC:
		assert.Nil(t, err)
	case <-timer.C:
		t.Fail()
	}
}

func TestRedeemerServer_MaxFloat(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sender := pm.RandAddress()
	req := &net.MaxFloatReq{
		Sender: sender.Bytes(),
	}

	// Set up redeemer server
	recipient := pm.RandAddress()
	eth := &eth.StubClient{}
	sm := newStubSenderMonitor()
	maxFloat := big.NewInt(100)
	sm.maxFloat = maxFloat

	r := &Redeemer{
		recipient: recipient,
		eth:       eth,
		sm:        sm,
		quit:      make(chan struct{}),
	}

	mfu, err := r.MaxFloat(ctx, req)
	assert.Nil(err)
	assert.Equal(mfu.MaxFloat, maxFloat.Bytes())

	// Test max float error
	sm.shouldFail = errors.New("maxfloat error")
	mfu, err = r.MaxFloat(ctx, req)
	assert.Nil(mfu)
	assert.EqualError(err, baseRPCErr+sm.shouldFail.Error())
}

// Test client
func TestRedeemerClient_QueueTicket_RPCErr(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &RedeemerClient{
		rpc:     rpc,
		senders: make(map[ethcommon.Address]*remoteSender), quit: make(chan struct{}),
	}

	ticket := &net.Ticket{
		Sender:        pm.RandAddress().Bytes(),
		RecipientRand: big.NewInt(1337).Bytes(),
		TicketParams: &net.TicketParams{
			Recipient:         pm.RandAddress().Bytes(),
			FaceValue:         big.NewInt(100).Bytes(),
			WinProb:           big.NewInt(100).Bytes(),
			RecipientRandHash: pm.RandBytes(32),
			ExpirationBlock:   big.NewInt(100).Bytes(),
		},
		SenderParams: &net.TicketSenderParams{
			Sig:         pm.RandBytes(32),
			SenderNonce: 1,
		},
		ExpirationParams: &net.TicketExpirationParams{
			CreationRound:          100,
			CreationRoundBlockHash: pm.RandHash().Bytes(),
		},
	}

	rpc.EXPECT().QueueTicket(gomock.Any(), gomock.Any()).Return(nil, errors.New("QueueTicket error"))
	assert.EqualError(rc.QueueTicket(pmTicket(ticket)), "QueueTicket error")
}

func TestRedeemerClient_QueueTicket_Success(t *testing.T) {
	assert := assert.New(t)

	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &RedeemerClient{
		rpc:     rpc,
		senders: make(map[ethcommon.Address]*remoteSender), quit: make(chan struct{}),
	}

	ticket := &net.Ticket{
		Sender:        pm.RandAddress().Bytes(),
		RecipientRand: big.NewInt(1337).Bytes(),
		TicketParams: &net.TicketParams{
			Recipient:         pm.RandAddress().Bytes(),
			FaceValue:         big.NewInt(100).Bytes(),
			WinProb:           big.NewInt(100).Bytes(),
			RecipientRandHash: pm.RandBytes(32),
			ExpirationBlock:   big.NewInt(100).Bytes(),
		},
		SenderParams: &net.TicketSenderParams{
			Sig:         pm.RandBytes(32),
			SenderNonce: 1,
		},
		ExpirationParams: &net.TicketExpirationParams{
			CreationRound:          100,
			CreationRoundBlockHash: pm.RandHash().Bytes(),
		},
	}

	rpc.EXPECT().QueueTicket(gomock.Any(), gomock.Any()).Return(nil, nil)
	err := rc.QueueTicket(pmTicket(ticket))
	assert.Nil(err)
}

func TestRedeemerClient_MaxFloat_LocalCacheExists(t *testing.T) {
	assert := assert.New(t)
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender)}
	sender := pm.RandAddress()
	rc.senders[sender] = &remoteSender{maxFloat: big.NewInt(100)}
	mf, err := rc.MaxFloat(sender)
	assert.Nil(err)
	assert.Equal(mf, big.NewInt(100))
}

func TestRedeemerClient_MaxFloat_NoLocalCache_MaxFloatRPCErr(t *testing.T) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		rpc:     rpc,
	}

	sender := ethcommon.HexToAddress("foo")

	// test error
	expErr := errors.New("MaxFloat error")
	rpc.EXPECT().MaxFloat(gomock.Any(), &net.MaxFloatReq{
		Sender: sender.Bytes(),
	}).Return(nil, expErr)

	mf, err := rc.MaxFloat(sender)
	assert.Nil(mf)
	assert.EqualError(err, expErr.Error())
}

func TestRedeemerClient_NoLocalCache_MaxFloatSuccess_MonitorMaxFloatError_ReturnsMaxFloat_NoCacheUpdate(t *testing.T) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		rpc:     rpc,
	}

	sender := ethcommon.HexToAddress("foo")
	expFloat := big.NewInt(100)
	req := &net.MaxFloatReq{
		Sender: sender.Bytes(),
	}
	res := &net.MaxFloatUpdate{MaxFloat: expFloat.Bytes()}
	rpc.EXPECT().MaxFloat(gomock.Any(), req).Return(res, nil)

	expErr := errors.New("MonitorMaxFloat error")
	rpc.EXPECT().MonitorMaxFloat(gomock.Any(), req).Return(nil, expErr)

	errLogsBefore := glog.Stats.Error.Lines()
	mf, err := rc.MaxFloat(sender)
	errLogsAfter := glog.Stats.Error.Lines()
	assert.Equal(mf, expFloat)
	assert.Nil(err)
	assert.Equal(errLogsAfter-errLogsBefore, int64(1))
	// check that local cache hasn't been updated
	_, ok := rc.senders[sender]
	assert.False(ok)
}

func TestRedeemerClient_MaxFloat_NoLocalCache_MaxFloatSuccess_MonitorMaxFloatSuccess_ReturnsMaxFloat_UpdatesCache(t *testing.T) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		rpc:     rpc,
	}

	sender := ethcommon.HexToAddress("foo")
	expFloat := big.NewInt(100)
	req := &net.MaxFloatReq{
		Sender: sender.Bytes(),
	}
	res := &net.MaxFloatUpdate{MaxFloat: expFloat.Bytes()}
	rpc.EXPECT().MaxFloat(gomock.Any(), req).Return(res, nil)

	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)

	rpc.EXPECT().MonitorMaxFloat(gomock.Any(), req).Return(stream, nil)
	stream.EXPECT().Recv().Return(&net.MaxFloatUpdate{MaxFloat: expFloat.Bytes()}, nil).AnyTimes()
	mf, err := rc.MaxFloat(sender)
	time.Sleep(time.Millisecond)
	assert.Equal(mf, expFloat)
	assert.Nil(err)
	// check that local cache is updated
	rc.mu.RLock()
	rs, ok := rc.senders[sender]
	assert.True(ok)
	assert.Equal(rs.stream, stream)
	assert.Equal(rs.maxFloat, expFloat)
	assert.NotZero(rs.lastAccess)
	rc.mu.RUnlock()
}

func TestRedeemerClient_MonitorMaxFloat_NoCache_Returns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
	}
	sender := ethcommon.HexToAddress("foo")
	go rc.monitorMaxFloat(cancel, sender)
	<-ctx.Done()
	assert.Nil(t, rc.senders[sender])
}

func TestRedeemerClient_MonitorMaxFloat_RecvErr(t *testing.T) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	sm := newStubSenderManager()
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		rpc:     rpc,
		sm:      sm,
	}

	sender := ethcommon.HexToAddress("foo")
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)

	rc.senders[sender] = &remoteSender{stream: stream}

	stream.EXPECT().Recv().Return(nil, io.EOF)

	ctx, cancel := context.WithCancel(context.Background())
	go rc.monitorMaxFloat(cancel, sender)
	<-ctx.Done()
	_, ok := rc.senders[sender]
	assert.False(ok)
}

func TestRedeemerClient_MonitorMaxFloat_RecvSuccess_UpdateCache(t *testing.T) {
	assert := assert.New(t)
	ctrl := gomock.NewController(t)
	rpc := net.NewMockTicketRedeemerClient(ctrl)
	defer ctrl.Finish()
	sm := newStubSenderManager()
	rc := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		rpc:     rpc,
		sm:      sm,
	}

	sender := ethcommon.HexToAddress("foo")
	stream := net.NewMockTicketRedeemer_MonitorMaxFloatClient(ctrl)
	lastAccess := time.Now()

	rc.senders[sender] = &remoteSender{
		stream:     stream,
		lastAccess: lastAccess,
		done:       make(chan struct{}),
	}
	expFloat := big.NewInt(100)

	stream.EXPECT().Recv().Return(&net.MaxFloatUpdate{MaxFloat: expFloat.Bytes()}, nil).AnyTimes()
	ctx, cancel := context.WithCancel(context.Background())
	go rc.monitorMaxFloat(cancel, sender)

	time.Sleep(20 * time.Millisecond)
	rc.mu.Lock()
	close(rc.senders[sender].done)
	rc.mu.Unlock()
	<-ctx.Done()
	assert.Equal(rc.senders[sender].maxFloat, expFloat)
	assert.True(rc.senders[sender].lastAccess.After(lastAccess))
}

func TestRedeemerClient_ValidateSender_SenderInfoErr(t *testing.T) {
	assert := assert.New(t)
	rc := &RedeemerClient{
		sm: newStubSenderManager(),
	}
	rc.sm.(*stubSenderManager).err = errors.New("GetSenderInfo error")
	assert.True(strings.Contains(rc.ValidateSender(pm.RandAddress()).Error(), "GetSenderInfo error"))
}

func TestRedeemerClient_ValidateSender_MaxWithDrawRoundErr(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	rc := &RedeemerClient{
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

	rc := &RedeemerClient{
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

func TestRedeemerClient_CleanupLoop(t *testing.T) {
	assert := assert.New(t)
	r := &RedeemerClient{
		senders: make(map[ethcommon.Address]*remoteSender),
		quit:    make(chan struct{}),
		sm:      newStubSenderManager(),
	}

	oldCleanupTime := cleanupLoopTime
	cleanupLoopTime = 20 * time.Millisecond
	defer func() {
		cleanupLoopTime = oldCleanupTime
	}()

	sender0 := ethcommon.HexToAddress("foo")
	sender1 := ethcommon.HexToAddress("bar")

	r.senders[sender0] = &remoteSender{maxFloat: big.NewInt(1), lastAccess: time.Now().Add(2 * cleanupLoopTime), done: make(chan struct{})}

	r.senders[sender1] = &remoteSender{maxFloat: big.NewInt(1), lastAccess: time.Now().Add(-cleanupLoopTime), done: make(chan struct{})}

	// cleanupLoop will run after 'cleanupLoopTime' so
	// sender0 will not be cleared
	// sender1 will be cleared
	go r.startCleanupLoop()
	time.Sleep(cleanupLoopTime)
	time.Sleep(10 * time.Millisecond)
	r.mu.RLock()
	_, ok := r.senders[sender0]
	r.mu.RUnlock()
	assert.True(ok)
	r.mu.RLock()
	_, ok = r.senders[sender1]
	r.mu.RUnlock()
	assert.False(ok)

	close(r.quit)
}

func TestProtoTicket(t *testing.T) {
	ogTicket := &pm.SignedTicket{
		Ticket: &pm.Ticket{
			Recipient:              ethcommon.BytesToAddress([]byte("foo")),
			Sender:                 ethcommon.BytesToAddress([]byte("bar")),
			FaceValue:              big.NewInt(1),
			WinProb:                big.NewInt(2),
			SenderNonce:            3,
			RecipientRandHash:      ethcommon.BytesToHash([]byte("baz")),
			CreationRound:          4,
			CreationRoundBlockHash: ethcommon.BytesToHash([]byte("jar")),
			ParamsExpirationBlock:  big.NewInt(5),
		},
		RecipientRand: big.NewInt(1),
		Sig:           []byte("boo"),
	}
	pTicket := protoTicket(ogTicket)
	assert.Equal(t, ogTicket, pmTicket(pTicket))
}

// Stubs

type stubSenderMonitor struct {
	maxFloat   *big.Int
	queued     []*pm.SignedTicket
	shouldFail error
	sink       chan<- struct{}
	sub        *stubSubscription
	subscribed chan struct{}
}

func newStubSenderMonitor() *stubSenderMonitor {
	return &stubSenderMonitor{
		maxFloat:   big.NewInt(0),
		subscribed: make(chan struct{}, 8),
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

func (s *stubSenderMonitor) SubscribeMaxFloatChange(sender ethcommon.Address, sink chan<- struct{}) event.Subscription {
	s.sink = sink
	s.sub = &stubSubscription{errCh: make(chan error)}
	s.subscribed <- struct{}{}
	return s.sub
}

type stubSubscription struct {
	errCh        chan error
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
	senderInfoSub  event.Subscription
	senderInfoSink chan<- ethcommon.Address
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

func (s *stubSenderManager) SubscribeReserveChange(sink chan<- ethcommon.Address) event.Subscription {
	s.senderInfoSink = sink
	s.senderInfoSub = &stubSubscription{errCh: make(chan error)}
	return s.senderInfoSub
}

func (s *stubSenderManager) Clear(addr ethcommon.Address) {
	delete(s.info, addr)
	delete(s.claimedReserve, addr)
}

type stubTimeManager struct {
	round              *big.Int
	blkHash            [32]byte
	preBlkHash         [32]byte
	transcoderPoolSize *big.Int
	lastSeenBlock      *big.Int

	blockNumSink chan<- *big.Int
	blockNumSub  event.Subscription
}

func (m *stubTimeManager) LastInitializedRound() *big.Int {
	return m.round
}

func (m *stubTimeManager) LastInitializedL1BlockHash() [32]byte {
	return m.blkHash
}

func (m *stubTimeManager) PreLastInitializedL1BlockHash() [32]byte {
	return m.preBlkHash
}

func (m *stubTimeManager) GetTranscoderPoolSize() *big.Int {
	return m.transcoderPoolSize
}

func (m *stubTimeManager) LastSeenL1Block() *big.Int {
	return m.lastSeenBlock
}

func (m *stubTimeManager) SubscribeRounds(sink chan<- types.Log) event.Subscription {
	return &stubSubscription{}
}

func (m *stubTimeManager) SubscribeL1Blocks(sink chan<- *big.Int) event.Subscription {
	m.blockNumSink = sink
	m.blockNumSub = &stubSubscription{errCh: make(chan error)}
	return m.blockNumSub
}
