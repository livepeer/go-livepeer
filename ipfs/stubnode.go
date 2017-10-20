package ipfs

import (
	"io"
)

type StubIpfsApi struct{}

func (s *StubIpfsApi) Add(r io.Reader) (string, error) { return "", nil }
