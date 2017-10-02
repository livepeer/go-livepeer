package eth

import (
	"math/big"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

func TestMonitor(t *testing.T) {
	eth := &StubClient{JobsMap: make(map[string]*Job)}
	lm := NewLogMonitor(eth)
	jobs := make(map[string]*Job)

	//Set up the subscriber for the job event
	lm.SubscribeToJobEvents(func(j *Job) {
		glog.Infof("j: %v", j)
		jobs[j.StreamId] = j
	})

	common.WaitUntil(time.Millisecond*500, func() bool {
		return eth.SubLogsCh != nil
	})

	//Set up eth.JobsMap so eth.GetJob() gets something back.
	eth.JobsMap["0"] = &Job{JobId: big.NewInt(10), StreamId: "testid"}

	//Insert a log into the channel - should call the subscriber we set up
	l := types.Log{Address: ethcommon.HexToAddress("0x94107cb2261e722f9f4908115546eeee17decada"), Topics: []ethcommon.Hash{ethcommon.HexToHash("0x94107cb2261e722f9f4908115546eeee17decada"), ethcommon.HexToHash("0x94107cb2261e722f9f4908115546eeee17decada")}, Data: []byte(ethcommon.Hex2BytesFixed("0x0", 32))}
	eth.SubLogsCh <- l

	common.WaitUntil(time.Millisecond*500, func() bool {
		return len(jobs) > 0
	})

	if _, ok := jobs["0"]; !ok {
		t.Errorf("Expecting job to come through the log monitor")
	}
}
