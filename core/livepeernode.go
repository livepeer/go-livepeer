/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var DefaultJobLength = int64(5760) //Avg 1 day in 15 sec blocks
var LivepeerVersion = "0.2.4-unstable"

//NodeID can be converted from libp2p PeerID.
type NodeID string

type NodeType int

const (
	Broadcaster NodeType = iota
	Transcoder
)

//LivepeerNode handles videos going in and coming out of the Livepeer network.
type LivepeerNode struct {
	Identity        NodeID
	VideoNetwork    net.VideoNetwork
	VideoCache      VideoCache
	Eth             eth.LivepeerEthClient
	EthEventMonitor eth.EventMonitor
	EthServices     map[string]eth.EventService
	ClaimManagers   map[int64]eth.ClaimManager
	SegmentChans    map[int64]SegmentChan
	Ipfs            ipfs.IpfsApi
	WorkDir         string
	NodeType        NodeType
	Database        *common.DB

	claimMutex   *sync.Mutex
	segmentMutex *sync.Mutex
}

//NewLivepeerNode creates a new Livepeer Node. Eth can be nil.
func NewLivepeerNode(e eth.LivepeerEthClient, vn net.VideoNetwork, nodeId NodeID, wd string, dbh *common.DB) (*LivepeerNode, error) {
	if vn == nil {
		glog.Errorf("Cannot create a LivepeerNode without a VideoNetwork")
		return nil, ErrLivepeerNode
	}

	return &LivepeerNode{VideoCache: NewBasicVideoCache(vn), VideoNetwork: vn, Identity: nodeId, Eth: e, WorkDir: wd, Database: dbh, EthServices: make(map[string]eth.EventService), ClaimManagers: make(map[int64]eth.ClaimManager), SegmentChans: make(map[int64]SegmentChan), claimMutex: &sync.Mutex{}, segmentMutex: &sync.Mutex{}}, nil
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

func (n *LivepeerNode) GetClaimManager(job *ethTypes.Job) (eth.ClaimManager, error) {
	n.claimMutex.Lock()
	defer n.claimMutex.Unlock()
	if job == nil {
		glog.Error("Nil job")
		return nil, fmt.Errorf("Nil job")
	}
	jobId := job.JobId.Int64()
	// XXX we should clear entries after some period of inactivity
	if cm, ok := n.ClaimManagers[jobId]; ok {
		return cm, nil
	}
	// no claimmanager exists yet; check if we're assigned the job
	if n.Eth == nil {
		return nil, nil
	}
	glog.Infof("Creating new claim manager for job %v", jobId)
	cm := eth.NewBasicClaimManager(job, n.Eth, n.Ipfs, n.Database)
	n.ClaimManagers[jobId] = cm
	return cm, nil
}
