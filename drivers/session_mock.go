package drivers

import (
	"context"

	"github.com/livepeer/go-livepeer/net"
	"github.com/stretchr/testify/mock"
)

type MockOSSession struct {
	mock.Mock
}

func (s *MockOSSession) SaveData(name string, data []byte, meta map[string]string) (string, error) {
	args := s.Called()
	return args.String(0), args.Error(1)
}

func (s *MockOSSession) EndSession() {
	s.Called()
}

func (s *MockOSSession) GetInfo() *net.OSInfo {
	args := s.Called()
	if args.Get(0) != nil {
		return args.Get(0).(*net.OSInfo)
	}
	return nil
}

func (s *MockOSSession) IsExternal() bool {
	args := s.Called()
	return args.Bool(0)
}

func (s *MockOSSession) IsOwn(url string) bool {
	args := s.Called()
	return args.Bool(0)
}

func (s *MockOSSession) ListFiles(ctx context.Context, prefix, delim string) (PageInfo, error) {
	return nil, nil
}

func (s *MockOSSession) ReadData(ctx context.Context, name string) (*FileInfoReader, error) {
	args := s.Called(ctx, name)
	var fi *FileInfoReader
	if args.Get(0) != nil {
		fi = args.Get(0).(*FileInfoReader)
	}
	return fi, args.Error(1)
}
func (s *MockOSSession) OS() OSDriver {
	return nil
}
