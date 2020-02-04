/*
Core contains the main functionality of the Livepeer node.

The logical orgnization of the `core` module is as follows:

livepeernode.go: Main struct definition and code that is common to all node types.
broadcaster.go: Code that is called only when the node is in broadcaster mode.
orchestrator.go: Code that is called only when the node is in orchestrator mode.

*/
package core

import (
	"errors"
	"math/big"
	"math/rand"
	"net/url"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/pm"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
)

var ErrTranscoderAvail = errors.New("ErrTranscoderUnavailable")
var ErrTranscode = errors.New("ErrTranscode")

// LivepeerVersion node version
// content of this constant will be set at build time,
// using -ldflags, combining content of `VERSION` file and
// output of the `git describe` command.
var LivepeerVersion = "undefined"

var MaxSessions = 10

type NodeType int

const (
	BroadcasterNode NodeType = iota
	OrchestratorNode
	TranscoderNode
)

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {

	// Common fields
	Eth      eth.LivepeerEthClient
	WorkDir  string
	NodeType NodeType
	Database *common.DB

	// Transcoder public fields
	SegmentChans      map[ManifestID]SegmentChan
	Recipient         pm.Recipient
	OrchestratorPool  common.OrchestratorPool
	OrchSecret        string
	Transcoder        Transcoder
	TranscoderManager *RemoteTranscoderManager
	Balances          *AddressBalances

	// Broadcaster public fields
	Sender pm.Sender

	// Thread safety for config fields
	mu sync.RWMutex
	// Transcoder private fields
	priceInfo    *big.Rat
	serviceURI   url.URL
	segmentMutex *sync.RWMutex
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, wd string, dbh *common.DB) (*LivepeerNode, error) {
	rand.Seed(time.Now().UnixNano())
	return &LivepeerNode{
		Eth:          e,
		WorkDir:      wd,
		Database:     dbh,
		SegmentChans: make(map[ManifestID]SegmentChan),
		segmentMutex: &sync.RWMutex{},
	}, nil
}

func (n *LivepeerNode) GetServiceURI() *url.URL {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return &n.serviceURI
}

func (n *LivepeerNode) SetServiceURI(newUrl *url.URL) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.serviceURI = *newUrl
}

// SetBasePrice sets the base price for an orchestrator on the node
func (n *LivepeerNode) SetBasePrice(price *big.Rat) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.priceInfo = price
}

// GetBasePrice gets the base price for an orchestrator
func (n *LivepeerNode) GetBasePrice() *big.Rat {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.priceInfo
}
