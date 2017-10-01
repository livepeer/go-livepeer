package eth

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
)

type LogMonitor struct {
	callbacks []func(j *Job)
}

func NewLogMonitor(eth LivepeerEthClient, broadcasterAddr, transcoderAddr common.Address) *LogMonitor {
	m := &LogMonitor{callbacks: make([]func(j *Job), 0)}

	go func() {
		logsCh := make(chan types.Log)
		logsSub, err := eth.SubscribeToJobEvent(context.Background(), logsCh, broadcasterAddr, transcoderAddr)
		if err != nil {
			glog.Errorf("Error subscribing to job event: %v", err)
		}

		defer close(logsCh)
		defer logsSub.Unsubscribe()

		for {
			select {
			case l, ok := <-logsCh:
				if !ok {
					glog.Infof("logsCh coming back with !ok, quitting...")
					return
				}
				_, _, jid := ParseNewJobLog(l)

				job, err := eth.GetJob(jid)
				if err != nil {
					glog.Errorf("Error getting job info: %v", err)
					continue
				}

				for _, cb := range m.callbacks {
					cb(job)
				}
			}
		}
	}()

	return m
}

func (m *LogMonitor) SubscribeToJobEvents(callback func(j *Job)) {
	glog.Infof("LogMonitor adding callback: %v", callback)
	m.callbacks = append(m.callbacks, callback)
}
