package eth

import (
	"context"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// RemoteNonceReader is an interface that describes an object
// capable of reading transaction nonces for ETH address from a remote source
type RemoteNonceReader interface {
	PendingNonceAt(ctx context.Context, addr ethcommon.Address) (uint64, error)
}

type nonceLock struct {
	nonce uint64
	mu    sync.Mutex
}

// NonceManager manages transaction nonces for multiple ETH addresses
type NonceManager struct {
	nonces map[ethcommon.Address]*nonceLock
	mu     sync.Mutex

	remoteReader RemoteNonceReader
}

// NewNonceManager creates an instance of a NonceManager
func NewNonceManager(remoteReader RemoteNonceReader) *NonceManager {
	return &NonceManager{
		nonces:       make(map[ethcommon.Address]*nonceLock),
		remoteReader: remoteReader,
	}
}

// Lock locks the provided address. The caller should always call Lock before
// calling Next or Update
func (m *NonceManager) Lock(addr ethcommon.Address) {
	m.getNonceLock(addr).mu.Lock()
}

// Unlock unlocks the provided address. The caller should always call Unlock
// after finishing calls to Next or Update
func (m *NonceManager) Unlock(addr ethcommon.Address) {
	m.getNonceLock(addr).mu.Unlock()
}

// Next returns the next transaction nonce to be used for the provided address
func (m *NonceManager) Next(addr ethcommon.Address) (uint64, error) {
	nonceLock := m.getNonceLock(addr)
	localNonce := nonceLock.nonce

	remoteNonce, err := m.remoteReader.PendingNonceAt(context.Background(), addr)
	if err != nil {
		return 0, err
	}

	// If remote nonce > local nonce, another client was likely used
	// to submit transactions such that the local nonce does not capture
	// transactions submitted by other clients
	if remoteNonce > localNonce {
		return remoteNonce, nil
	}

	return localNonce, nil
}

// Update uses the last nonce for the provided address to update the next transaction nonce
func (m *NonceManager) Update(addr ethcommon.Address, lastNonce uint64) {
	nonceLock := m.getNonceLock(addr)
	localNonce := nonceLock.nonce

	if lastNonce == localNonce {
		nonceLock.nonce = localNonce + 1
		return
	}

	nonceLock.nonce = lastNonce + 1
}

func (m *NonceManager) getNonceLock(addr ethcommon.Address) *nonceLock {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nonces[addr]; !ok {
		m.nonces[addr] = new(nonceLock)
	}

	return m.nonces[addr]
}
