package common

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/net"
	"go.uber.org/goleak"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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

type testAddr struct {
}

func (t *testAddr) String() string {
	return "TestAddress"
}

func (t *testAddr) Network() string {
	return "TestNetwork"
}

type StubServerStream struct {
}

func (s *StubServerStream) Context() context.Context {
	p := &peer.Peer{
		Addr: &testAddr{},
	}
	return peer.NewContext(context.Background(), p)
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
func (s *StubServerStream) Send(n *net.NotifySegment) error {
	return nil
}

// IgnoreRoutines goroutines to ignore in tests
func IgnoreRoutines() []goleak.Option {
	// goleak works by making list of all running goroutines and reporting error if it finds any
	// this list tells goleak to ignore these goroutines - we're not interested in these particular goroutines
	funcs2ignore := []string{"github.com/golang/glog.(*loggingT).flushDaemon", "go.opencensus.io/stats/view.(*worker).start",
		"github.com/rjeczalik/notify.(*recursiveTree).dispatch", "github.com/rjeczalik/notify._Cfunc_CFRunLoopRun", "github.com/ethereum/go-ethereum/metrics.(*meterArbiter).tick",
		"github.com/ethereum/go-ethereum/consensus/ethash.(*Ethash).remote", "github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache",
		"internal/poll.runtime_pollWait", "github.com/livepeer/go-livepeer/core.(*RemoteTranscoderManager).Manage", "github.com/livepeer/lpms/core.(*LPMS).Start",
		"github.com/livepeer/go-livepeer/server.(*LivepeerServer).StartMediaServer", "github.com/livepeer/go-livepeer/core.(*RemoteTranscoderManager).Manage.func1",
		"github.com/livepeer/go-livepeer/server.(*LivepeerServer).HandlePush.func1", "github.com/rjeczalik/notify.(*nonrecursiveTree).dispatch",
		"github.com/rjeczalik/notify.(*nonrecursiveTree).internal", "github.com/livepeer/lpms/stream.NewBasicRTMPVideoStream.func1", "github.com/patrickmn/go-cache.(*janitor).Run",
		"github.com/golang/glog.(*fileSink).flushDaemon",
	}

	res := make([]goleak.Option, 0, len(funcs2ignore))
	for _, f := range funcs2ignore {
		res = append(res, goleak.IgnoreTopFunction(f))
	}
	return res
}
