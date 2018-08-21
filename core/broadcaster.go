package core

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golang/glog"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/livepeer/go-livepeer/common"
	ethTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/net"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var ErrNotFound = errors.New("ErrNotFound")

// Broadcaster RPC interface implementation

type broadcaster struct {
	node  *LivepeerNode
	httpc *http.Client
	job   *ethTypes.Job
	tinfo *net.TranscoderInfo
}

func (bcast *broadcaster) Sign(msg []byte) ([]byte, error) {
	if bcast.node == nil || bcast.node.Eth == nil {
		return []byte{}, fmt.Errorf("Cannot sign; missing eth client")
	}
	return bcast.node.Eth.Sign(crypto.Keccak256(msg))
}
func (bcast *broadcaster) Job() *ethTypes.Job {
	return bcast.job
}
func (bcast *broadcaster) GetHTTPClient() *http.Client {
	return bcast.httpc
}
func (bcast *broadcaster) SetHTTPClient(hc *http.Client) {
	bcast.httpc = hc
}
func (bcast *broadcaster) GetTranscoderInfo() *net.TranscoderInfo {
	return bcast.tinfo
}
func (bcast *broadcaster) SetTranscoderInfo(t *net.TranscoderInfo) {
	bcast.tinfo = t
}
func NewBroadcaster(node *LivepeerNode, job *ethTypes.Job) *broadcaster {
	return &broadcaster{
		node: node,
		job:  job,
	}
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
