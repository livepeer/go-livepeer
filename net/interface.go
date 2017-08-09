package net

import (
	"context"
	"math/big"

	"github.com/ericxtang/m3u8"
	"github.com/livepeer/go-livepeer/types"
)

//VideoNetwork describes the interface for a Livepeer node network-layer library.
type VideoNetwork interface {
	GetNodeID() string
	GetMasterPlaylist(nodeID string, strmID string) (chan *m3u8.MasterPlaylist, error)
	UpdateMasterPlaylist(strmID string, mpl *m3u8.MasterPlaylist) error
	GetBroadcaster(strmID string) (Broadcaster, error)
	GetSubscriber(strmID string) (Subscriber, error)
	Connect(nodeID, nodeAddr string) error
	SetupProtocol() error
	SendTranscodeResponse(nodeID string, strmID string, transcodeResult map[string]string) error
	ReceivedTranscodeResponse(strmID string, gotResult func(transcodeResult map[string]string))
	String() string
}

//Broadcaster takes a streamID and a reader, and broadcasts the data to whatever underlining network.
//Note the data param doesn't have to be the raw data.  The implementation can choose to encode any struct.
//Example:
// 	s := GetStream("StrmID")
// 	b := ppspp.NewBroadcaster("StrmID", s.Metadata())
// 	for seqNo, data := range s.Segments() {
// 		b.Broadcast(seqNo, data)
// 	}
//	b.Finish()
type Broadcaster interface {
	Broadcast(seqNo uint64, data []byte) error
	IsWorking() bool
	Finish() error
	String() string
}

//Subscriber subscribes to a stream defined by strmID.  It returns a reader that contains the stream.
//Example 1:
//	sub, metadata := ppspp.NewSubscriber("StrmID")
//	stream := NewStream("StrmID", metadata)
//	ctx, cancel := context.WithCancel(context.Background()
//	err := sub.Subscribe(ctx, func(seqNo uint64, data []byte){
//		stream.WriteSeg(seqNo, data)
//	})
//	time.Sleep(time.Second * 5)
//	cancel()
//
//Example 2:
//	sub.Unsubscribe() //This is the same with calling cancel()
type Subscriber interface {
	Subscribe(ctx context.Context, gotData func(seqNo uint64, data []byte, eof bool)) error
	IsWorking() bool
	Unsubscribe() error
	String() string
}

type TranscodeConfig struct {
	StrmID              string
	Profiles            []types.VideoProfile
	PerformOnchainClaim bool
	JobID               *big.Int
}

type Transcoder interface {
	Transcode(strmID string, config TranscodeConfig, gotPlaylist func(masterPlaylist []byte)) error
}
