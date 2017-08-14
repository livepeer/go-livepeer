package streamingviz

import (
	"encoding/json"
	"fmt"
	"sync"
)

const (
	NodeIdle         = iota // 0
	NodeBroadcasting        // 1
	NodeConsuming           // 2
	NodeRelaying            // 3
)

const (
	LinkSending = iota
	LinkReceiveing
)

const (
	initialNetworkSize = 50
	initialStreams     = 10
)

type Node struct {
	ID    string
	Group map[string]int // For a string streamID, what group is it in
}

func (self *Node) GroupForStream(streamID string) int {
	val, ok := self.Group[streamID]
	if ok == false {
		self.Group[streamID] = NodeIdle
		return NodeIdle
	}
	return val
}

type Link struct {
	Source *Node
	Target *Node
	Value  map[string]int // For a string streamID, what Value does it have
}

type Network struct {
	Nodes     map[string]*Node
	Links     []*Link
	StreamIDs []string // All known streams
	lock      sync.Mutex
}

func NewNetwork() *Network {
	return &Network{
		Nodes:     make(map[string]*Node),
		Links:     make([]*Link, 0),
		StreamIDs: make([]string, initialStreams),
	}
}

// Does the node exist in the network already
func (self *Network) hasNode(id string) bool {
	_, ok := self.Nodes[id]
	return ok
}

// Does the link exist in the network already
func (self *Network) hasLink(src string, target string) bool {
	for _, link := range self.Links {
		if (link.Source.ID == src && link.Target.ID == target) ||
			(link.Target.ID == src && link.Source.ID == target) {
			return true
		}
	}
	return false
}

func (self *Network) addNode(id string) {
	self.Nodes[id] = &Node{
		ID:    id,
		Group: make(map[string]int),
	}
}

func (self *Network) addLink(src string, target string) {
	sNode, ok1 := self.findNode(src)
	tNode, ok2 := self.findNode(target)

	if ok1 && ok2 {
		self.Links = append(self.Links, &Link{
			Source: sNode,
			Target: tNode,
			Value:  make(map[string]int),
		})
	}
}

func (self *Network) findNode(id string) (*Node, bool) {
	node, ok := self.Nodes[id]
	return node, ok
}

// Reconstructs the links array removing links for this node
func (self *Network) removeLinksForNode(nodeID string) {
	node, ok := self.findNode(nodeID)
	if ok {
		links := make([]*Link, 0)
		for _, link := range self.Links {
			if link.Source != node && link.Target != node {
				links = append(links, link)
			}
		}
		self.Links = links
	}
}

// Messages that may be received
// 1. Node sends its peers - can construct the peer graph
// 2. Node says it's publishing a stream
// 3. Node says it's requesting a stream
// 4. Node says it's relaying a stream
// 5. Finish publishing, requesting, relaying. TBD whether this is explicit or on a timeout

// Add this node to the network and add its peers as links. Remove existing links first to keep state consistent
func (self *Network) ReceivePeersForNode(nodeID string, peerIDs []string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if !self.hasNode(nodeID) {
		self.addNode(nodeID)
		fmt.Println("Adding node:", nodeID)
	}

	self.removeLinksForNode(nodeID)

	for _, p := range peerIDs {
		if !self.hasNode(p) {
			self.addNode(p)
			fmt.Println("Adding node from peer:", p)
		}

		if !self.hasLink(nodeID, p) {
			self.addLink(nodeID, p)
			fmt.Println("Adding link", nodeID, p)
		}
	}
}

func (self *Network) StartBroadcasting(nodeID string, streamID string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	node, ok := self.findNode(nodeID)
	if ok {
		node.Group[streamID] = NodeBroadcasting
	}
}

func (self *Network) StartConsuming(nodeID string, streamID string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	node, ok := self.findNode(nodeID)
	if ok {
		node.Group[streamID] = NodeConsuming
	}
}

func (self *Network) StartRelaying(nodeID string, streamID string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	node, ok := self.findNode(nodeID)
	if ok {
		node.Group[streamID] = NodeRelaying
	}
}

func (self *Network) DoneWithStream(nodeID string, streamID string) {
	self.lock.Lock()
	defer self.lock.Unlock()

	node, ok := self.findNode(nodeID)
	if ok {
		node.Group[streamID] = NodeIdle
	}
}

func (self *Network) String() string {
	self.lock.Lock()
	defer self.lock.Unlock()

	b, err := json.Marshal(self)
	if err != nil {
		return fmt.Sprintf("Error creating json from this network %v", err)
	}
	return fmt.Sprintf("%s", b)
}
