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
	lpmon "github.com/livepeer/go-livepeer/monitor"
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
	DefaultNode NodeType = iota
	BroadcasterNode
	OrchestratorNode
	TranscoderNode
	RedeemerNode
)

var nodeTypeStrs = map[NodeType]string{
	DefaultNode:      "default",
	BroadcasterNode:  "broadcaster",
	OrchestratorNode: "orchestrator",
	TranscoderNode:   "transcoder",
	RedeemerNode:     "redeemer",
}

func (t NodeType) String() string {
	str, ok := nodeTypeStrs[t]
	if !ok {
		return "unknown"
	}
	return str
}

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
	Capabilities      *Capabilities
	AutoAdjustPrice   bool

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
		Eth:             e,
		WorkDir:         wd,
		Database:        dbh,
		AutoAdjustPrice: true,
		SegmentChans:    make(map[ManifestID]SegmentChan),
		segmentMutex:    &sync.RWMutex{},
		Capabilities:    &Capabilities{capacities: map[Capability]int{}},
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

// SetMaxFaceValue sets the faceValue upper limit for tickets received
func (n *LivepeerNode) SetMaxFaceValue(maxfacevalue *big.Int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Recipient.SetMaxFaceValue(maxfacevalue)
}

func (n *LivepeerNode) SetTranscoderSortMethod(m int) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	n.TranscoderManager.sortMethod = m
	if lpmon.Enabled {
		lpmon.SetTranscoderSortMethod(m)
	}
}

func (n *LivepeerNode) SetTranscoderCapacity(t_uri string, c int) {
	n.TranscoderManager.RTmutex.Lock()
	defer n.TranscoderManager.RTmutex.Unlock()
	for _, transcoder := range n.TranscoderManager.liveTranscoders {
		if transcoder.addr == t_uri {
			//update transcoder capacity
			transcoder.capacity = c
			//update metrics reporting
			if lpmon.Enabled {
				var totalLoad, totalCapacity, liveTranscodersNum int
				totalLoad, totalCapacity, liveTranscodersNum = n.TranscoderManager.totalLoadAndCapacity()
				lpmon.SetTranscodersNumberAndLoad(totalLoad, totalCapacity, liveTranscodersNum)
				lpmon.SetTranscoderStats(transcoder.addr, transcoder.load, transcoder.capacity, transcoder.ppns)
			}
		}
	}
}

func (n *LivepeerNode) SetTranscoderPriority(t_uri string, p int) {
	n.TranscoderManager.RTmutex.Lock()
	defer n.TranscoderManager.RTmutex.Unlock()
	for _, transcoder := range n.TranscoderManager.liveTranscoders {
		if transcoder.addr == t_uri {
			//update transcoder priority
			transcoder.priority = p
			//update metrics reporting
			if lpmon.Enabled {
				lpmon.SetTranscoderPriority(transcoder.addr, transcoder.priority)
			}
		}
	}
}

func (n *LivepeerNode) SetMaxSessions(s int) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	MaxSessions = s

	//update metrics reporting
	if lpmon.Enabled {
		lpmon.MaxSessions(MaxSessions)
	}
}
