package ipfs

import (
	"io"
)

type StubIpfsApi struct{}

func (s *StubIpfsApi) Add(r io.Reader) (string, error)                            { return "", nil }
func (s *StubIpfsApi) AddToDir(dir, fileName string, r io.Reader) (string, error) { return "", nil }
