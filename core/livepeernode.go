/*
Core contains the main functionality of the Livepeer node.

The logical orgnization of the `core` module is as follows:

livepeernode.go: Main struct definition and code that is common to all node types.
broadcaster.go: Code that is called only when the node is in broadcaster mode.
orchestrator.go: Code that is called only when the node is in orchestrator mode.

*/
package core

import (
	"context"
	"errors"
	"net/url"
	"sync"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/ipfs"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var DefaultJobLength = int64(5760) //Avg 1 day in 15 sec blocks
var LivepeerVersion = "0.3.0-unstable"

type NodeType int

const (
	Broadcaster NodeType = iota
	Transcoder
)

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {

	// Common fields
	VideoSource     VideoSource
	Eth             eth.LivepeerEthClient
	EthEventMonitor eth.EventMonitor
	EthServices     map[string]eth.EventService
	WorkDir         string
	NodeType        NodeType
	Database        *common.DB

	// Transcoder public fields
	ClaimManagers map[int64]eth.ClaimManager
	SegmentChans  map[int64]SegmentChan
	Ipfs          ipfs.IpfsApi
	ServiceURI    *url.URL

	// Transcoder private fields
	claimMutex   *sync.Mutex
	segmentMutex *sync.Mutex
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, wd string, dbh *common.DB) (*LivepeerNode, error) {

	return &LivepeerNode{VideoSource: NewBasicVideoSource(), Eth: e, WorkDir: wd, Database: dbh, EthServices: make(map[string]eth.EventService), ClaimManagers: make(map[int64]eth.ClaimManager), SegmentChans: make(map[int64]SegmentChan), claimMutex: &sync.Mutex{}, segmentMutex: &sync.Mutex{}}, nil
}

func (n *LivepeerNode) StartEthServices() error {
	var err error
	for k, s := range n.EthServices {
		// Skip BlockService until the end
		if k == "BlockService" {
			continue
		}
		err = s.Start(context.Background())
		if err != nil {
			return err
		}
	}

	// Make sure to initialize BlockService last so other services can
	// create filters starting from the last seen block
	if s, ok := n.EthServices["BlockService"]; ok {
		if err := s.Start(context.Background()); err != nil {
			return err
		}
	}

	return nil
}

func (n *LivepeerNode) StopEthServices() error {
	var err error
	for _, s := range n.EthServices {
		err = s.Stop()
		if err != nil {
			return err
		}
	}

	return nil
}
