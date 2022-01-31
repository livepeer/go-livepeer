package blockwatch

import (
	"math/big"
	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// MiniHeader is a succinct representation of an Ethereum block header
type MiniHeader struct {
	Hash          ethcommon.Hash
	Parent        ethcommon.Hash
	Number        *big.Int
	L1BlockNumber *big.Int
	Logs          []types.Log
}

// MiniHeaderStore is an interface for a store that manages the state of a MiniHeader collection
type MiniHeaderStore interface {
	FindLatestMiniHeader() (*MiniHeader, error)
	FindAllMiniHeadersSortedByNumber() ([]*MiniHeader, error)
	InsertMiniHeader(header *MiniHeader) error
	DeleteMiniHeader(hash ethcommon.Hash) error
}

// Stack allows performing basic stack operations on a stack of MiniHeaders.
type Stack struct {
	// TODO(albrow): Use Transactions when db supports them instead of a mutex
	// here. There are cases where we need to make sure no modifications are made
	// to the database in between a read/write or read/delete.
	mut   sync.Mutex
	store MiniHeaderStore
	limit int
}

// NewStack instantiates a new stack with the specified size limit. Once the size limit
// is reached, adding additional blocks will evict the deepest block.
func NewStack(store MiniHeaderStore, limit int) *Stack {
	return &Stack{
		store: store,
		limit: limit,
	}
}

// Pop removes and returns the latest block header on the block stack. It
// returns nil if the stack is empty.
func (s *Stack) Pop() (*MiniHeader, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	latestMiniHeader, err := s.store.FindLatestMiniHeader()
	if err != nil {
		return nil, err
	}
	if latestMiniHeader == nil {
		return nil, nil
	}
	if err := s.store.DeleteMiniHeader(latestMiniHeader.Hash); err != nil {
		return nil, err
	}
	return latestMiniHeader, nil
}

// Push pushes a block header onto the block stack. If the stack limit is
// reached, it will remove the oldest block header.
func (s *Stack) Push(header *MiniHeader) error {
	s.mut.Lock()
	defer s.mut.Unlock()
	miniHeaders, err := s.store.FindAllMiniHeadersSortedByNumber()
	if err != nil {
		return err
	}
	if len(miniHeaders) == s.limit {
		oldestMiniHeader := miniHeaders[0]
		if err := s.store.DeleteMiniHeader(oldestMiniHeader.Hash); err != nil {
			return err
		}
	}
	if err := s.store.InsertMiniHeader(header); err != nil {
		return err
	}
	return nil
}

// Peek returns the latest block header from the block stack without removing
// it. It returns nil if the stack is empty.
func (s *Stack) Peek() (*MiniHeader, error) {
	return s.store.FindLatestMiniHeader()
}

// Inspect returns all the block headers currently on the stack
func (s *Stack) Inspect() ([]*MiniHeader, error) {
	return s.store.FindAllMiniHeadersSortedByNumber()
}
