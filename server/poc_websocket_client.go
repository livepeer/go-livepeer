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

func (w *WebsocketPublish) Send(chunk *MpegtsChunk) error {
	withId := struct {
		MpegtsChunk
		Id string
	}{*chunk, "MpegtsChunk"}
	if err := w.connection.WriteJSON(&withId); err != nil {
		return err
	}
	if err := w.connection.WriteMessage(websocket.BinaryMessage, chunk.Bytes); err != nil {
		return err
	}
	return nil
}

func (w *WebsocketPublish) Recv() (int, []byte, error) {
	return w.connection.ReadMessage()
}

func (w *WebsocketPublish) SendJobEnd() error {
	message := MessageHeader{"EndOfInput"}
	if err := w.connection.WriteJSON(&message); err != nil {
		return err
	}
	return nil
}
