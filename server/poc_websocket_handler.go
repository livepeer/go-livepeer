package server

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{} // use default options

// Turns HTTP request into websocket connection - transcoding "session".
//
// My hope is to remove existing concept of transcoding sessions and bind
//   transcoding session & GPU resources to websocket connection, making
//   websocket connection single-source-of-truth.
// While websocket connection is alive all associated resources are kept allocated.
// When data transfer is slow upstream drops connection.
// When latency keeps increasing upstream drops connection.
// When signatures don't match upstream drops connection.
// When downstream breaks connection we do failover to other node. Keeping last
//   GOP-sized number of input frames to switch to other transcoder without
//   any interruption.
//
func WebsocketIngest(w http.ResponseWriter, r *http.Request) {
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("webbsocket upgrade error\n")
		return
	}
	transcoding := TranscodingConnection{connection: connection}
	defer transcoding.Close()

	fmt.Printf("ws connected, doing handshake\n")
	if err := transcoding.Handshake(r); err != nil {
		fmt.Printf("Handshake error %v", err)
		return
	}

	if err := transcoding.RunUntilCompletion(); err != nil {
		fmt.Printf("Transcoding error %v", err)
		return
	}
	fmt.Printf("ws complete\n")
}

// For testing purposes
func ServeHttp(port int) error {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("serving HTTP on %s\n", address)
	http.HandleFunc("/wslive/", WebsocketIngest)
	err := http.ListenAndServe(address, nil)
	fmt.Printf("HTTP server exited\n")
	return err
}
