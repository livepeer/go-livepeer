package common

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/net"
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
