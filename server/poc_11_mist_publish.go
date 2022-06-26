package server

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// Helper for state in Mist>>B transport
type WebsocketPublish struct {
	url              url.URL
	connection       *websocket.Conn
	httpResponse     *http.Response
	streamingStarted bool

	// segmentHash  *StreamingHash - removed
	// TODO: discuss is frame signing enough?
}

func (w *WebsocketPublish) Dial(url url.URL) error {
	var err error
	w.url = url
	w.connection, w.httpResponse, err = websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		return err
	}
	return nil
}

func (w *WebsocketPublish) sendVirtualSegmentBoundary(lastFrame *InputChunk) error {
	// `lastFrame.Bytes` is not part of current segment, rather first frame of next segment.
	boundary := newVirtualSegmentBoundary()
	// .SequenceNumber here is previous frame's SequenceNumber, gets incremented before frame is sent
	// TODO: We need segment sequence number
	boundary.FrameSequenceNumber = lastFrame.Info.SequenceNumber
	// .BytesProduced here is previous frame's BytesProduced, gets incremented before frame is sent
	boundary.BytesProduced = lastFrame.Info.BytesProduced
	return sendMessage(w.connection, boundary)
}

func (w *WebsocketPublish) Send(chunk *InputChunk) error {
	// Mist does not sign the media data. B takes care of signing before forwarding frames to Os.
	// Virtual segment boundary is not found on first chunk of first frame
	if chunk.Info.FirstFrameInSegment {
		if w.streamingStarted {
			w.sendVirtualSegmentBoundary(chunk)
		}
		w.streamingStarted = true
	}
	// Real Mist server would update all metadata.
	// For transcoder operation only media bytes are *required*. For our pixel & price
	//   calculation we depend on metadata. Transcoder node would double check using ffmpeg.
	chunk.Info.SequenceNumber += 1
	chunk.Info.BytesProduced += len(chunk.Bytes)
	if err := sendMessage(w.connection, chunk); err != nil {
		return err
	}
	// Frame sent, next frame won't be first frame of a segment. Unless caller signals this again.
	chunk.Info.FirstFrameInSegment = false
	return nil
}

func (w *WebsocketPublish) Recv() interface{} {
	return recvMessage(w.connection)
}

func (w *WebsocketPublish) SendJobEnd(lastFrame *InputChunk) error {
	// We are mindfull to send signal that entire segment is complete.
	// Consider doing this same step downstream even when only EndOfInput arrives.
	// Also does transcoder charge for work even if segment was interrupted??
	// TODO: why `w.segmentHash.GetHash()` called twice raise panic? We need to fix this problem
	w.sendVirtualSegmentBoundary(lastFrame)
	return sendMessage(w.connection, newEndOfInput())
}
