package client

import (
	"testing"
)

// Need to be running a server on port 8585 to run the below.
// For now treat it as an example usage
func testVizClient(t *testing.T) {
	host := "http://localhost:8585"
	doneC1 := make(chan bool)
	doneC2 := make(chan bool)
	doneC3 := make(chan bool)

	client := NewClient("A", true, host)
	client.ConsumeEvents(doneC1)

	client.LogPeers([]string{"B", "C"})
	client.LogBroadcast("stream1")

	client2 := NewClient("B", true, host)
	client2.ConsumeEvents(doneC2)

	client2.LogPeers([]string{"A", "D"})

	client3 := NewClient("D", true, host)
	client3.ConsumeEvents(doneC3)

	client3.LogPeers([]string{"B"})
	client3.LogConsume("stream1")

	client2.LogRelay("stream1")
	doneC1 <- true
	doneC2 <- true
	doneC3 <- true
}

func TestBiggerNetwork(t *testing.T) {
	host := "http://localhost:8585"
	streamID := "stream2"
	nodes := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "H",
		"L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}
	doneCs := make([]chan bool, len(nodes))
	clients := make([]*Client, len(nodes))

	for i := 0; i < len(nodes); i++ {
		doneCs[i] = make(chan bool)
		clients[i] = NewClient(nodes[i], true, host)
		clients[i].ConsumeEvents(doneCs[i])

		if i < len(nodes)-6 {
			clients[i].LogPeers(nodes[i+1 : i+6])
		}
	}

	clients[0].LogBroadcast(streamID)
	clients[2].LogConsume(streamID)
	clients[9].LogConsume(streamID)
	clients[13].LogConsume(streamID)
	clients[20].LogConsume(streamID)
	clients[7].LogRelay(streamID)
	clients[17].LogRelay(streamID)

	for i := 0; i < len(nodes); i++ {
		doneCs[i] <- true
	}
}
