package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	NodeID        string
	Enabled       bool
	Endpoint      string
	PeersChan     chan []string
	BroadcastChan chan string
	ConsumeChan   chan string
	RelayChan     chan string
	DoneChan      chan string
}

func NewClient(nodeID string, enabled bool, host string) *Client {
	return &Client{
		NodeID:        nodeID,
		Enabled:       enabled,
		Endpoint:      fmt.Sprintf("%s/event", host), // Default. Override if you'd like to change,
		PeersChan:     make(chan []string),
		BroadcastChan: make(chan string),
		ConsumeChan:   make(chan string),
		RelayChan:     make(chan string),
		DoneChan:      make(chan string),
	}
}

func (self *Client) LogPeers(peers []string) {
	self.PeersChan <- peers
}

func (self *Client) LogBroadcast(streamID string) {
	self.BroadcastChan <- streamID
}

func (self *Client) LogConsume(streamID string) {
	self.ConsumeChan <- streamID
}

func (self *Client) LogRelay(streamID string) {
	self.RelayChan <- streamID
}

func (self *Client) LogDone(streamID string) {
	self.DoneChan <- streamID
}

func (self *Client) InitData(eventName string) (data map[string]interface{}) {
	data = make(map[string]interface{})
	data["name"] = eventName
	data["node"] = self.NodeID
	return
}

func (self *Client) PostEvent(data map[string]interface{}) {
	// For now just don't actually post the data to a server if viz is not enabled
	if self.Enabled {
		enc, _ := json.Marshal(data)

		req, err := http.NewRequest("POST", self.Endpoint, bytes.NewBuffer(enc))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Couldn't connect to the event server", err)
			return
		}
		defer resp.Body.Close()
	}
}

// Starts consuming events. NodeID must be set or else this will error.
// Pass doneC <- true from the calling thread when you wish to stop the event loop.
func (self *Client) ConsumeEvents(doneC chan bool) error {
	if self.NodeID == "" {
		return fmt.Errorf("NodeID must be set before consuming events")
	}

	go self.consumeLoop(doneC)
	return nil
}

func (self *Client) consumeLoop(doneC chan bool) {
	for {
		select {
		case peers := <-self.PeersChan:
			data := self.InitData("peers")
			data["peers"] = peers
			self.PostEvent(data)
		case streamID := <-self.BroadcastChan:
			data := self.InitData("broadcast")
			data["streamId"] = streamID
			self.PostEvent(data)
		case streamID := <-self.ConsumeChan:
			data := self.InitData("consume")
			data["streamId"] = streamID
			self.PostEvent(data)
		case streamID := <-self.RelayChan:
			data := self.InitData("relay")
			data["streamId"] = streamID
			self.PostEvent(data)
		case streamID := <-self.DoneChan:
			data := self.InitData("done")
			data["streamId"] = streamID
			self.PostEvent(data)
		case <-doneC:
			return
		}
	}
}
