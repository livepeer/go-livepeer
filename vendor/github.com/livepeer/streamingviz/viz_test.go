package streamingviz

import (
	"fmt"
	"testing"
)

func TestNetwork(t *testing.T) {
	sID := "teststream"
	network := NewNetwork()

	// Set up peers
	network.ReceivePeersForNode("A", []string{"B", "D"})
	network.ReceivePeersForNode("B", []string{"A", "D"})
	network.ReceivePeersForNode("C", []string{"D"})

	network.StartBroadcasting("A", sID)
	network.StartConsuming("C", sID)
	network.StartRelaying("D", sID)
	network.StartConsuming("B", sID)
	network.DoneWithStream("B", sID)

	fmt.Println(network.String())
}
