package net

import (
	metrics "github.com/libp2p/go-libp2p-metrics"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
)

type BasicReporter struct{}

func (br *BasicReporter) LogSentMessage(num int64) {
	// glog.Infof("Reporter - Message Sent: %v", num)
}
func (br *BasicReporter) LogRecvMessage(num int64) {
	// glog.Infof("Reporter - Message Received: %v", num)
}
func (br *BasicReporter) LogSentMessageStream(num int64, prot protocol.ID, p peer.ID) {
	// glog.Infof("Reporter - SentMessageStream: %v, %v %v", num, prot, peer.IDHexEncode(p))
}
func (br *BasicReporter) LogRecvMessageStream(num int64, prot protocol.ID, p peer.ID) {
	// glog.Infof("Reporter - RecvMessageStream: %v, %v %v", num, prot, peer.IDHexEncode(p))
}
func (br *BasicReporter) GetBandwidthForPeer(peer.ID) metrics.Stats {
	return metrics.Stats{}
}
func (br *BasicReporter) GetBandwidthForProtocol(protocol.ID) metrics.Stats {
	return metrics.Stats{}
}
func (br *BasicReporter) GetBandwidthTotals() metrics.Stats {
	return metrics.Stats{}
}
