package monitor

import (
	"context"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/streamingviz/data"
)

type Monitor struct {
	Node *data.Node
}

var monitor *Monitor
var Endpoint string

const PostFreq = time.Second * 10

func Instance() *Monitor {
	if monitor == nil {
		monitor = newMonitor()
		monitor.StartWorker(context.Background())
	}

	return monitor
}

func newMonitor() *Monitor {
	return &Monitor{Node: data.NewNode("")}
}

func (m *Monitor) SetBootNode() {
	m.Node.SetBootNode()
}

func (m *Monitor) LogNewConn(local, remote string) {
	if m.Node.ID == "" {
		m.Node.ID = local
	}

	m.Node.AddConn(local, remote)
}

func (m *Monitor) RemoveConn(local, remote string) {
	m.Node.RemoveConn(local, remote)
}

func (m *Monitor) LogStream(strmID string, size uint, avgChunkSize uint) {
	m.Node.SetStream(strmID, size, avgChunkSize)
}

func (m *Monitor) RemoveStream(strmID string) {
	m.Node.RemoveStream(strmID)
}

func (m *Monitor) LogBroadcaster(strmID string) {
	m.Node.SetBroadcast(strmID)
}

func (m *Monitor) RemoveBroadcast(strmID string) {
	m.Node.RemoveBroadcast(strmID)
}

func (m *Monitor) LogRelay(strmID, remote string) {
	m.Node.SetRelay(strmID, remote)
}

func (m *Monitor) RemoveRelay(strmID string) {
	m.Node.RemoveRelay(strmID)
}

func (m *Monitor) LogSub(strmID string) {
	m.Node.SetSub(strmID)
}

func (m *Monitor) RemoveSub(strmID string) {
	m.Node.RemoveSub(strmID)
}

func (m *Monitor) LogBuffer(strmID string) {
	m.Node.AddBufferEvent(strmID)
}

func (m *Monitor) GetPeerCount() int {
	return len(m.Node.Conns)
}

func (m *Monitor) StartWorker(ctx context.Context) {
	ticker := time.NewTicker(PostFreq)
	go func() {
		for {
			select {
			case <-ticker.C:
				// glog.Infof("Posting node status to monitor")
				if m.Node.ID != "" {
					m.Node.SubmitToCollector(Endpoint)
				}
			case <-ctx.Done():
				glog.Errorf("Monitor Worker Done")
				return
			}
		}
	}()
}
