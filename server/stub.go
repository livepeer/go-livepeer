package server

import (
	"github.com/livepeer/go-livepeer/net"
)

type StubCapabilityComparator struct {
	NetCaps  *net.Capabilities
	IsLegacy bool
}

func (s *StubCapabilityComparator) ToNetCapabilities() *net.Capabilities {
	return s.NetCaps
}

func (s *StubCapabilityComparator) CompatibleWith(other *net.Capabilities) bool {
	// Implement the logic for compatibility check if needed
	return true
}

func (s *StubCapabilityComparator) LegacyOnly() bool {
	return s.IsLegacy
}
