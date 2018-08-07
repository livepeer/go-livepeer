/*
Core contains the main functionality of the Livepeer node.
*/
package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/ipfs"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var ErrLivepeerNode = errors.New("ErrLivepeerNode")
var ErrTranscode = errors.New("ErrTranscode")
var ErrNotFound = errors.New("ErrNotFound")
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

//CreateTranscodeJob creates the on-chain transcode job.
func (n *LivepeerNode) CreateTranscodeJob(strmID StreamID, profiles []ffmpeg.VideoProfile, price *big.Int) (*ethTypes.Job, error) {
	if n.Eth == nil {
		glog.Errorf("Cannot create transcode job, no eth client found")
		return nil, ErrNotFound
	}

	transOpts := common.ProfilesToTranscodeOpts(profiles)

	//Call eth client to create the job
	blknum, err := n.Eth.LatestBlockNum()
	if err != nil {
		return nil, err
	}

	_, err = n.Eth.Job(strmID.String(), ethcommon.ToHex(transOpts)[2:], price, big.NewInt(0).Add(blknum, big.NewInt(DefaultJobLength)))
	if err != nil {
		glog.Errorf("Error creating transcode job: %v", err)
		return nil, err
	}

	job, err := n.Eth.WatchForJob(strmID.String())
	if err != nil {
		glog.Error("Unable to monitor for job ", err)
		return nil, err
	}
	glog.V(common.DEBUG).Info("Got a new job from the blockchain: ", job.JobId)

	assignedTranscoder := func() error {
		tca, err := n.Eth.AssignedTranscoder(job)
		if err == nil && (tca == ethcommon.Address{}) {
			glog.Error("A transcoder was not assigned! Ensure the broadcast price meets the minimum for the transcoder pool")
			err = fmt.Errorf("EmptyTranscoder")
		}
		if err != nil {
			glog.Error("Retrying transcoder assignment lookup because of ", err)
			return err
		}
		job.TranscoderAddress = tca
		return nil
	}
	boff := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*2), 30)
	err = backoff.Retry(assignedTranscoder, boff) // retry for 1 minute max
	if err != nil {
		// not fatal at this point; continue
		glog.Error("Error getting assigned transcoder ", err)
	}

	err = n.Database.InsertBroadcast(job)
	if err != nil {
		glog.Error("Unable to insert broadcast ", err)
		// not fatal; continue
	}

	glog.Infof("Created broadcast job. Id: %v, Price: %v, Transcoder:%v, Type: %v", job.JobId, job.MaxPricePerSegment, job.TranscoderAddress.Hex(), ethcommon.ToHex(transOpts)[2:])

	return job, nil
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
