package server

import (
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// Encapsulates Mist>>Transcoder protocol
type WebsocketPublish struct {
	url          url.URL
	connection   *websocket.Conn
	httpResponse *http.Response
	segmentHash  *StreamingHash
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

func (w *WebsocketPublish) sendVirtualSegmentBoundary(lastFrame *MpegtsChunk) error {
	var err error
	// We calculated hash of all preceding frames.
	// `lastFrame.Bytes` is not part of current segment, rather first frame of next segment.
	boundary := VirtualSegmentBoundary{MessageHeader: MessageHeader{"VirtualSegmentBoundary"}}
	boundary.Hash = w.segmentHash.GetHash()
	// We use .SignMediaData() here, i suspect we should add new .SignSegment() method ?
	//   because of `accountManager.signHash(accounts.TextHash(msg))` <<= additional TextHash call.
	//   Or maybe also use same code in .SignMediaData() ?
	if boundary.Signature, err = Notary.SignMediaData(boundary.Hash); err != nil {
		return err
	}
	// .SequenceNumber here is previous frame's SequenceNumber, gets incremented before frame is sent
	boundary.SequenceNumber = lastFrame.Info.SequenceNumber
	// .BytesProduced here is previous frame's BytesProduced, gets incremented before frame is sent
	boundary.BytesProduced = lastFrame.Info.BytesProduced
	// Signal virtual segment boundary, including entire segment hash and signature.
	if err := w.connection.WriteJSON(&boundary); err != nil {
		return err
	}
	return nil
}

func (w *WebsocketPublish) Send(chunk *MpegtsChunk) error {
	// Calculate signature of frame bytes.
	// We sign media bytes for frame signature. Segment signature happens on hash of data.
	var err error
	if chunk.Signature, err = Notary.SignMediaData(chunk.Bytes); err != nil {
		return err
	}

	// Virtual segment boundary is not found on first chunk of first frame
	if chunk.Info.FirstFrameInSegment {
		hashingOngoing := w.segmentHash != nil
		if hashingOngoing {
			w.sendVirtualSegmentBoundary(chunk)
		}
		// Starting new hash calculation for new virtual segment:
		w.segmentHash = &StreamingHash{}
		w.segmentHash.Init()
	}

	// Real Mist server would update all metadata.
	// For transcoder operation only media bytes are *required*. For our pixel & price
	//   calculation we depend on metadata. Transcoder node would double check using ffmpeg.
	chunk.Info.SequenceNumber += 1
	chunk.Info.BytesProduced += len(chunk.Bytes)

	chunk.Id = "MpegtsChunk" // golang: how to set this as default/cosntant member value?
	if err := w.connection.WriteJSON(&chunk); err != nil {
		return err
	}
	if err := w.connection.WriteMessage(websocket.BinaryMessage, chunk.Bytes); err != nil {
		return err
	}
	w.segmentHash.Append(chunk.Bytes)
	// // fmt.Printf(" > %d hash=%x\n", chunk.Info.SequenceNumber, w.segmentHash.GetHash()) causing panic?

	// Frame sent, next frame won't be first frame of a segment. Unless caller signals this again.
	chunk.Info.FirstFrameInSegment = false

	return nil
}

func (w *WebsocketPublish) Recv() (int, []byte, error) {
	return w.connection.ReadMessage()
}

func (w *WebsocketPublish) SendJobEnd(lastFrame *MpegtsChunk) error {
	// We are mindfull to send signal that entire segment is complete.
	// Consider doing this same step downstream even when only EndOfInput arrives.
	// Also does transcoder charge for work even if segment was interrupted??
	// TODO: why `w.segmentHash.GetHash()` called twice raise panic? We need to fix this problem
	w.sendVirtualSegmentBoundary(lastFrame)
	message := EndOfInput{MessageHeader{"EndOfInput"}}
	if err := w.connection.WriteJSON(&message); err != nil {
		return err
	}
	return nil
}
