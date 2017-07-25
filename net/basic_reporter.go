package net

import (
	metrics "gx/ipfs/QmVjRAPfRtResCMCE4eBqr4Beoa6A89P1YweG9wUS6RqUL/go-libp2p-metrics"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
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
