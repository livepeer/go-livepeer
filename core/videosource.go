package core

import (
	"github.com/ericxtang/m3u8"
	"github.com/livepeer/go-livepeer/drivers"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/lpms/stream"
)

type VideoSource interface {
	// Implicitly creates master play list
	AddStream(manifestID ManifestID, streamID StreamID, profile string, osSession drivers.OSSession,
		doNotSaveMasterPlaylist, doNotSaveMediaPlaylist bool)
	InsertHLSSegment(streamID StreamID, seg *stream.HLSSegment) ([]*net.TypedURI, error)
	// InsertLink link to segment that already in the storage
	// (Transcoder wrote segment in B's owned storage and returned link, B adds this link to video source)
	// returns true if it was acutally inserted
	InsertLink(turi *net.TypedURI, seqNo uint64) (bool, error)
	// EvictHLSMasterPlaylist removes master playlist and calls EndSession on all the streams
	EvictHLSMasterPlaylist(manifestID ManifestID)

	GetLiveHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist
	GetFullHLSMasterPlaylist(manifestID ManifestID) *m3u8.MasterPlaylist


	GetLiveHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist
	GetFullHLSMediaPlaylist(streamID StreamID) *m3u8.MediaPlaylist

	GetNodeStatus() *net.NodeStatus
}
