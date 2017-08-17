package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
)

const NetworkTimerFreq = time.Second * 60

type Network struct {
	Nodes map[string]*Node
	Timer map[string]time.Time
}

func NewNetwork() *Network {
	n := &Network{
		Nodes: make(map[string]*Node),
		Timer: make(map[string]time.Time)}
	n.startTimer(context.Background())
	return n
}

func (n *Network) startTimer(ctx context.Context) {
	ticker := time.NewTicker(NetworkTimerFreq)
	go func() {
		for {
			select {
			case <-ticker.C:
				for id, insertTime := range n.Timer {
					if time.Since(insertTime) > NetworkTimerFreq {
						delete(n.Timer, id)
						delete(n.Nodes, id)
					}
				}
			case <-ctx.Done():
				glog.Errorf("Monitor Network Timer Done")
				return
			}
		}
	}()
}

func (n *Network) SetNode(node *Node) {
	n.Nodes[node.ID] = node
	n.Timer[node.ID] = time.Now()
}

func (n *Network) ToD3Json() interface{} {
	nodes := make([]map[string]interface{}, 0)
	links := make([]map[string]string, 0)
	streams := make(map[string]map[string]interface{})

	for _, node := range n.Nodes {
		// glog.Infof("Node: %v", n.ID)
		//Add nodes
		nmap := map[string]interface{}{"id": node.ID, "bootnode": node.IsBootNode}
		nodes = append(nodes, nmap)

		//Add links
		for _, conn := range node.Conns {
			_, ok1 := n.Nodes[conn.N1]
			_, ok2 := n.Nodes[conn.N2]
			if ok1 && ok2 {
				links = append(links, map[string]string{"source": conn.N1, "target": conn.N2})
			}
		}

		//Add streams, broadcasters, relayers and subscribers
		checkStrm := func(id string, streams map[string]map[string]interface{}) {
			strm, ok := streams[id]
			if !ok {
				// glog.Infof("New Stream: %v", id)
				strm = map[string]interface{}{"broadcaster": "", "relayers": make([]string, 0), "subscribers": make([]map[string]string, 0)}
				streams[id] = strm
			}
		}

		for _, b := range node.Broadcasts {
			checkStrm(b.StrmID, streams)
			streams[b.StrmID]["broadcaster"] = node.ID
		}
		for _, sub := range node.Subs {
			// glog.Infof("n: %v, strmID: %v", n.ID, sub.StrmID)
			checkStrm(sub.StrmID, streams)
			streams[sub.StrmID]["subscribers"] = append(streams[sub.StrmID]["subscribers"].([]map[string]string), map[string]string{"id": node.ID, "buffer": string(sub.BufferCount)})
		}
		for _, r := range node.Relays {
			// glog.Infof("n: %v, strmID: %v", n.ID, r.StrmID)
			checkStrm(r.StrmID, streams)
			streams[r.StrmID]["relayers"] = append(streams[r.StrmID]["relayers"].([]string), node.ID)
		}

	}

	result := map[string]interface{}{
		"nodes":   nodes,
		"links":   links,
		"streams": streams,
	}

	b, err := json.Marshal(result)
	if err != nil {
		glog.Errorf("Error marshaling json: %v", err)
	}

	var res interface{}
	json.Unmarshal(b, &res)
	return res
}

type Node struct {
	ID                  string                   `json:"id"`
	IsBootNode          bool                     `json:"isBootNode"`
	Conns               []Conn                   `json:"conns"`
	Strms               map[string]Stream        `json:"strms"`
	Broadcasts          map[string]Broadcast     `json:"broadcasts"`
	Relays              map[string]Relay         `json:"relays"`
	Subs                map[string]*Subscription `json:"subs"`
	ConnsInLastHr       uint                     `json:"connsInLastHr"`
	StrmsInLastHr       uint                     `json:"strmsInLastHr"`
	TotalVidLenInLastHr uint                     `json:"totalVidLenInLastHr"`
	TotalRelayInLastHr  uint                     `json:"totalRelayInLastHr"`
	AvgCPU              float32                  `json:"avgCPU"`
	AvgMem              uint32                   `json:"avgMem"`
	AvgBandwidth        uint32                   `json:"avgBandwidth"`
	ConnWorker          map[Conn]time.Time       `json:"-"`
}

func NewNode(id string) *Node {
	n := &Node{
		ID:         id,
		Conns:      make([]Conn, 0),
		Strms:      make(map[string]Stream),
		Broadcasts: make(map[string]Broadcast),
		Relays:     make(map[string]Relay),
		Subs:       make(map[string]*Subscription),
		ConnWorker: make(map[Conn]time.Time),
	}

	return n
}

func (n *Node) AddConn(local, remote string) {
	newConn := NewConn(local, remote)
	n.Conns = append(n.Conns, newConn)
	n.ConnsInLastHr++
	n.ConnWorker[newConn] = time.Now()
}

func (n *Node) RemoveConn(local, remote string) {
	rmc := NewConn(local, remote)
	for i, c := range n.Conns {
		if c == rmc {
			n.Conns = append(n.Conns[:i], n.Conns[i+1:]...)
			return
		}
	}
}

func (n *Node) SetBootNode() {
	n.IsBootNode = true
}

func (n *Node) SetStream(id string, size, avgChunkSize uint) {

	// n.Strms = append(n.Strms, Stream{ID: id, Chunks: size, AvgChunkSize: avgChunkSize})
	n.Strms[id] = Stream{ID: id, Chunks: size, AvgChunkSize: avgChunkSize}
}

func (n *Node) RemoveStream(id string) {
	delete(n.Strms, id)
}

func (n *Node) SetBroadcast(strmID string) {
	// n.Broadcasts = append(n.Broadcasts, Broadcast{StrmID: id})
	n.Broadcasts[strmID] = Broadcast{StrmID: strmID}
}

func (n *Node) RemoveBroadcast(strmID string) {
	delete(n.Broadcasts, strmID)
}

func (n *Node) SetSub(strmID string) {
	// n.Subs = append(n.Subs, Subscription{StrmID: strmID})
	n.Subs[strmID] = &Subscription{StrmID: strmID}
}

func (n *Node) RemoveSub(strmID string) {
	delete(n.Subs, strmID)
}

func (n *Node) AddBufferEvent(strmID string) {
	// for i, sub := range n.Subs {
	// 	if sub.StrmID == strmID {
	// 		n.Subs[i].BufferCount++
	// 	}
	// }

	glog.Info("Logging buffer event")
	n.Subs[strmID].BufferCount = n.Subs[strmID].BufferCount + 1
}

func (n *Node) SetRelay(strmID string, remote string) {
	// n.Relays = append(n.Relays, Relay{StrmID: strmID, RemoteN: remote})
	n.Relays[strmID] = Relay{StrmID: strmID, RemoteN: remote}
}

func (n *Node) RemoveRelay(strmID string) {
	delete(n.Relays, strmID)
}

func (n *Node) SubmitToCollector(endpoint string) {
	if endpoint == "" {
		return
	}

	enc, err := json.Marshal(n)
	if err != nil {
		glog.Errorf("Error marshaling node status: %v", err)
		return
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(enc))
	if err != nil {
		glog.Errorf("Error creating new request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Couldn't connect to the event server", err)
		return
	}
	defer resp.Body.Close()
}

type Conn struct {
	N1 string `json:"n1"`
	N2 string `json:"n2"`
}

func NewConn(local, remote string) Conn {
	if strings.Compare(local, remote) > 0 {
		return Conn{N1: local, N2: remote}
	} else {
		return Conn{N1: remote, N2: local}
	}
}

type Stream struct {
	ID           string `json:"id"`
	Chunks       uint   `json:"chunks"`
	AvgChunkSize uint   `json:"avgChunkSize"`
}

type Broadcast struct {
	StrmID string
}

type Relay struct {
	StrmID  string
	RemoteN string
}

type Subscription struct {
	StrmID      string
	BufferCount uint
}
