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
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
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

// LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {

	// Common fields
	Eth      eth.LivepeerEthClient
	WorkDir  string
	NodeType NodeType
	Database *common.DB

	// Transcoder public fields
	SegmentChans       map[ManifestID]SegmentChan
	Recipient          pm.Recipient
	RecipientAddr      string
	SelectionAlgorithm common.SelectionAlgorithm
	OrchestratorPool   common.OrchestratorPool
	OrchPerfScore      *common.PerfScore
	OrchSecret         string
	Transcoder         Transcoder
	TranscoderManager  *RemoteTranscoderManager
	Balances           *AddressBalances
	Capabilities       *Capabilities
	AutoAdjustPrice    bool
	AutoSessionLimit   bool
	// Broadcaster public fields
	Sender pm.Sender

	// Thread safety for config fields
	mu             sync.RWMutex
	StorageConfigs map[string]*transcodeConfig
	storageMutex   *sync.RWMutex
	// Transcoder private fields
	priceInfo    map[string]*AutoConvertedPrice
	serviceURI   url.URL
	segmentMutex *sync.RWMutex
}

// NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, wd string, dbh *common.DB) (*LivepeerNode, error) {
	rand.Seed(time.Now().UnixNano())
	return &LivepeerNode{
		Eth:             e,
		WorkDir:         wd,
		Database:        dbh,
		AutoAdjustPrice: true,
		SegmentChans:    make(map[ManifestID]SegmentChan),
		segmentMutex:    &sync.RWMutex{},
		Capabilities:    &Capabilities{capacities: map[Capability]int{}, version: LivepeerVersion},
		priceInfo:       make(map[string]*AutoConvertedPrice),
		StorageConfigs:  make(map[string]*transcodeConfig),
		storageMutex:    &sync.RWMutex{},
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
func (n *LivepeerNode) SetBasePrice(b_eth_addr string, price *AutoConvertedPrice) {
	addr := strings.ToLower(b_eth_addr)
	n.mu.Lock()
	defer n.mu.Unlock()

	prevPrice := n.priceInfo[addr]
	n.priceInfo[addr] = price
	if prevPrice != nil {
		prevPrice.Stop()
	}
}

// GetBasePrice gets the base price for an orchestrator
func (n *LivepeerNode) GetBasePrice(b_eth_addr string) *big.Rat {
	addr := strings.ToLower(b_eth_addr)
	n.mu.RLock()
	defer n.mu.RUnlock()

	price := n.priceInfo[addr]
	if price == nil {
		return nil
	}
	return price.Value()
}

func (n *LivepeerNode) GetBasePrices() map[string]*big.Rat {
	n.mu.RLock()
	defer n.mu.RUnlock()

	prices := make(map[string]*big.Rat)
	for addr, price := range n.priceInfo {
		prices[addr] = price.Value()
	}
	return prices
}

// SetMaxFaceValue sets the faceValue upper limit for tickets received
func (n *LivepeerNode) SetMaxFaceValue(maxfacevalue *big.Int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Recipient.SetMaxFaceValue(maxfacevalue)
}

func (n *LivepeerNode) SetMaxSessions(s int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	MaxSessions = s

	//update metrics reporting
	if lpmon.Enabled {
		lpmon.MaxSessions(MaxSessions)
	}

	glog.Infof("Updated session limit to %d", MaxSessions)
}

func (n *LivepeerNode) GetCurrentCapacity() int {
	n.TranscoderManager.RTmutex.Lock()
	defer n.TranscoderManager.RTmutex.Unlock()
	_, totalCapacity, _ := n.TranscoderManager.totalLoadAndCapacity()
	return totalCapacity
}
