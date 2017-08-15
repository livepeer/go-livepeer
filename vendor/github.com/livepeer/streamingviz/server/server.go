package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/livepeer/streamingviz"
	"github.com/livepeer/streamingviz/data"
)

var networkDB *streamingviz.Network
var network *data.Network

func init() {
	networkDB = streamingviz.NewNetwork()
	network = data.NewNetwork()
}

func handler(w http.ResponseWriter, r *http.Request) {
	abs, _ := filepath.Abs("./server/static/index.html")
	view, err := template.ParseFiles(abs)

	// data := getData()

	// network := getNetwork()
	// data := networkToData(network, "teststream")

	// streamID := r.URL.Query().Get("streamid")
	// data := networkToData(networkDB, streamID)
	data := network.ToD3Json()

	if err != nil {
		fmt.Fprintf(w, "error: %v", err)
	} else {
		view.Execute(w, data)
	}
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fmt.Fprintf(w, "Error. You must POST events")
		return
	}

	body, _ := ioutil.ReadAll(r.Body)
	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		fmt.Fprintf(w, "Error unmarshalling request: %v", err)
		return
	}

	eventName := event["name"].(string)
	node := event["node"].(string)

	switch eventName {
	case "peers":
		peers := event["peers"].([]interface{})
		peerList := make([]string, 0)
		for _, v := range peers {
			peerList = append(peerList, v.(string))
		}
		networkDB.ReceivePeersForNode(node, peerList)
	case "broadcast":
		streamID := event["streamId"].(string)
		fmt.Println("Got a BROADCAST event for", node, streamID)
		networkDB.StartBroadcasting(node, streamID)
	case "consume":
		streamID := event["streamId"].(string)
		fmt.Println("Got a CONSUME event for", node, streamID)
		networkDB.StartConsuming(node, streamID)
	case "relay":
		streamID := event["streamId"].(string)
		fmt.Println("Got a RELAY event for", node, streamID)
		networkDB.StartRelaying(node, streamID)
	case "done":
		streamID := event["streamId"].(string)
		fmt.Println("Got a DONE event for", node, streamID)
		networkDB.DoneWithStream(node, streamID)
	case "default":
		fmt.Fprintf(w, "Error, eventName %v is unknown", eventName)
		return
	}
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	rBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Errorf("Error reading r: %v", err)
	}

	node := &data.Node{}
	err = json.Unmarshal(rBody, node)
	if err != nil {
		glog.Errorf("Error unmarshaling: %v", err)
	}
	glog.Infof("Got Node: %v", string(rBody))

	network.SetNode(node)
}

func main() {
	// http.HandleFunc("/data.json", handleJson)
	http.HandleFunc("/event", handleEvent)
	http.HandleFunc("/metrics", handleMetrics)
	http.HandleFunc("/", handler)
	glog.Infof("Listening on 8081")
	http.ListenAndServe(":8081", nil)
}

func getNetwork() *streamingviz.Network {
	sID := "teststream"
	network := streamingviz.NewNetwork()

	// Set up peers
	network.ReceivePeersForNode("A", []string{"B", "D", "F"})
	network.ReceivePeersForNode("B", []string{"D", "E"})
	network.ReceivePeersForNode("C", []string{"I", "F", "G"})
	network.ReceivePeersForNode("E", []string{"I", "H"})
	network.ReceivePeersForNode("F", []string{"G"})
	network.ReceivePeersForNode("G", []string{"H"})
	network.ReceivePeersForNode("H", []string{"I"})

	network.StartBroadcasting("A", sID)
	network.StartConsuming("I", sID)
	network.StartConsuming("G", sID)
	network.StartRelaying("F", sID)
	network.StartRelaying("C", sID)
	network.DoneWithStream("B", sID)

	return network
}

func networkToData(network *streamingviz.Network, streamID string) interface{} {
	/*type Node struct {
		ID string
		Group int
	}

	type Link struct {
		Source string
		Target string
		Value int
	}*/

	res := make(map[string]interface{})
	nodes := make([]map[string]interface{}, 0)

	for _, v := range network.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"id":    v.ID,
			"group": v.GroupForStream(streamID),
		})
	}

	links := make([]map[string]interface{}, 0)

	for _, v := range network.Links {
		links = append(links, map[string]interface{}{
			"source": v.Source.ID,
			"target": v.Target.ID,
			"value":  2, //v.Value[streamID],
		})
	}

	res["nodes"] = nodes
	res["links"] = links

	b, _ := json.Marshal(res)
	fmt.Println(fmt.Sprintf("The output network is: %s", b))

	var genResult interface{}

	json.Unmarshal(b, &genResult)
	return genResult
}

func getData() map[string]*json.RawMessage {
	var objmap map[string]*json.RawMessage
	abs, _ := filepath.Abs("./server/static/data.json")
	f, err := ioutil.ReadFile(abs)
	if err != nil {
		glog.Errorf("Error reading file: %v", err)
	}
	if err := json.Unmarshal(f, &objmap); err != nil {
		glog.Errorf("Error unmarshaling data: %v", err)
	}
	return objmap
}
