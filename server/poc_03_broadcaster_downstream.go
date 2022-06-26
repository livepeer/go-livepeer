package server

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

// Envelope for network message, B << O, that is sent via channel.
// Multiple Os sent stream messages to same channel.
type MessageFromO struct {
	message    interface{}
	downstream *BroadcasterOutConnection
}

// Sends input stream from Mist to single O.
// B sends duplicated stream to multiple Os.
// To duplicate stream messages we use channels > SendFrame and < sendResult.
type BroadcasterOutConnection struct {
	oAddress     url.URL
	oSignCheck   SignatureChecker
	sendResult   chan *MessageFromO
	connection   *websocket.Conn
	httpResponse *http.Response
	SendFrame    chan interface{} // * MessageType

	deleteFromBooks func()
}

// Used whenever error is encountered.
func (c *BroadcasterOutConnection) Close() {
	// TODO: what else is included in cleanup?
	c.connection.Close()
	// Remove myself from BroadcasterConnection's books
	c.deleteFromBooks()
}

func (c *BroadcasterOutConnection) Init() {
	c.SendFrame = make(chan interface{}, 1000)
}

// Receive resulting stream from network and post to B's channel.
// Verifies signature of received media frames.
func (c *BroadcasterOutConnection) receiveLoop() {
	for {
		switch message := recvMessage(c.connection).(type) {
		case *OutputChunk:
			// Verify O signature
			signatureValid := c.oSignCheck.Check(message.Bytes, message.Signature)
			if !signatureValid {
				fmt.Printf("BroadcasterOutConnection.receiveLoop O %s signature invalid \n", c.oAddress.String())
				c.operationInterrupted("signature check failed")
				return
			}
			c.sendResult <- &MessageFromO{message, c}
		case error:
			c.operationInterrupted(fmt.Sprintf("receiveLoop() <- %v", message))
			return
		default:
			// Forwarding all messages with envelope containing our pointer
			c.sendResult <- &MessageFromO{message, c}
		}
	}
}

// Read duplicated input stream via channel and send to O via network.
func (c *BroadcasterOutConnection) Run() {
	var err error
	c.connection, c.httpResponse, err = websocket.DefaultDialer.Dial(c.oAddress.String(), nil)
	if err != nil {
		c.operationInterrupted(err.Error())
		return
	}
	go c.receiveLoop()
	// Send loop follows
	for {
		// BroadcasterConnection sends us duplicated message pointers to forward.
		// Media data is not copied. We don't modify message content, no muxtex is needed.
		message, ok := <-c.SendFrame
		if !ok {
			return
		}
		// B forwards all messages from Mist to designated O.
		// B prepared frame struc on recv-from-Mist, including signature.
		// We pass all frames, VirtualSegmentBoundary & EndOfInput as we receive them.
		// All connected Os get same stream.
		if err := sendMessage(c.connection, message); err != nil {
			c.operationInterrupted(err.Error())
			return
		}
	}
}

// TODO: .. cleanup logic missing ..
func (c *BroadcasterOutConnection) operationInterrupted(reason string) {
	fmt.Printf("BroadcasterOutConnection %s breaking connection on error: %s \n", c.oAddress.String(), reason)
	c.Close()
}

type BDownstreamDecision struct {
	FramesInBuffer   FrameCount
	inputConnection  *BroadcasterConnection
	outputConnection *BroadcasterOutConnection
}

func (d *BDownstreamDecision) BreakConnection() {
	d.inputConnection.remove(d.outputConnection)
	d.outputConnection.Close()
}
