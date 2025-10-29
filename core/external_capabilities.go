package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
)

type JobToken struct {
	SenderAddress *JobSender        `json:"sender_address,omitempty"`
	TicketParams  *net.TicketParams `json:"ticket_params,omitempty"`
	Balance       int64             `json:"balance,omitempty"`
	Price         *net.PriceInfo    `json:"price,omitempty"`
	ServiceAddr   string            `json:"service_addr,omitempty"`

	LastNonce uint32
}
type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

type ExternalCapability struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Url           string `json:"url"`
	Capacity      int    `json:"capacity"`
	PricePerUnit  int64  `json:"price_per_unit"`
	PriceScaling  int64  `json:"price_scaling"`
	PriceCurrency string `json:"currency"`

	price *AutoConvertedPrice

	mu   sync.RWMutex
	Load int
}

type StreamInfo struct {
	StreamID   string
	Capability string

	//Orchestrator fields
	Sender         ethcommon.Address
	StreamRequest  []byte
	pubChannel     *trickle.TrickleLocalPublisher
	subChannel     *trickle.TrickleLocalPublisher
	controlChannel *trickle.TrickleLocalPublisher
	eventsChannel  *trickle.TrickleLocalPublisher
	dataChannel    *trickle.TrickleLocalPublisher
	//Stream fields
	JobParams    string
	StreamCtx    context.Context
	CancelStream context.CancelFunc

	sdm sync.Mutex
}

func (sd *StreamInfo) IsActive() bool {
	sd.sdm.Lock()
	defer sd.sdm.Unlock()
	if sd.StreamCtx.Err() != nil {
		return false
	}

	if sd.controlChannel == nil {
		return false
	}

	return true
}

func (sd *StreamInfo) UpdateParams(params string) {
	sd.sdm.Lock()
	defer sd.sdm.Unlock()
	sd.JobParams = params
}

func (sd *StreamInfo) SetChannels(pub, sub, control, events, data *trickle.TrickleLocalPublisher) {
	sd.sdm.Lock()
	defer sd.sdm.Unlock()
	sd.pubChannel = pub
	sd.subChannel = sub
	sd.controlChannel = control
	sd.eventsChannel = events
	sd.dataChannel = data
}

type ExternalCapabilities struct {
	capm         sync.Mutex
	Capabilities map[string]*ExternalCapability
	Streams      map[string]*StreamInfo
}

func NewExternalCapabilities() *ExternalCapabilities {
	return &ExternalCapabilities{Capabilities: make(map[string]*ExternalCapability),
		Streams: make(map[string]*StreamInfo),
	}
}

func (extCaps *ExternalCapabilities) AddStream(streamID string, capability string, streamReq []byte) (*StreamInfo, error) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()
	_, ok := extCaps.Streams[streamID]
	if ok {
		return nil, fmt.Errorf("stream already exists: %s", streamID)
	}

	//add to streams
	ctx, cancel := context.WithCancel(context.Background())
	stream := StreamInfo{
		StreamID:      streamID,
		Capability:    capability,
		StreamRequest: streamReq,
		StreamCtx:     ctx,
		CancelStream:  cancel,
	}
	extCaps.Streams[streamID] = &stream

	//clean up when stream ends
	go func() {
		<-ctx.Done()

		//orchestrator channels shutdown
		if stream.pubChannel != nil {
			if err := stream.pubChannel.Close(); err != nil {
				glog.Errorf("error closing pubChannel for stream=%s: %v", streamID, err)
			}
		}
		if stream.subChannel != nil {
			if err := stream.subChannel.Close(); err != nil {
				glog.Errorf("error closing subChannel for stream=%s: %v", streamID, err)
			}
		}
		if stream.controlChannel != nil {
			if err := stream.controlChannel.Close(); err != nil {
				glog.Errorf("error closing controlChannel for stream=%s: %v", streamID, err)
			}
		}
		if stream.eventsChannel != nil {
			if err := stream.eventsChannel.Close(); err != nil {
				glog.Errorf("error closing eventsChannel for stream=%s: %v", streamID, err)
			}
		}
		if stream.dataChannel != nil {
			if err := stream.dataChannel.Close(); err != nil {
				glog.Errorf("error closing dataChannel for stream=%s: %v", streamID, err)
			}
		}
		return
	}()

	return &stream, nil
}

func (extCaps *ExternalCapabilities) RemoveStream(streamID string) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	streamInfo, ok := extCaps.Streams[streamID]
	if ok {
		//confirm stream context is canceled before deleting
		if streamInfo.StreamCtx.Err() == nil {
			streamInfo.CancelStream()
		}
	}

	delete(extCaps.Streams, streamID)
}

func (extCaps *ExternalCapabilities) GetStream(streamID string) (*StreamInfo, bool) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	streamInfo, ok := extCaps.Streams[streamID]
	return streamInfo, ok
}

func (extCaps *ExternalCapabilities) StreamExists(streamID string) bool {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	_, ok := extCaps.Streams[streamID]
	return ok
}

func (extCaps *ExternalCapabilities) RemoveCapability(extCap string) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	delete(extCaps.Capabilities, extCap)
}

func (extCaps *ExternalCapabilities) RegisterCapability(extCapability string) (*ExternalCapability, error) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()
	if extCaps.Capabilities == nil {
		extCaps.Capabilities = make(map[string]*ExternalCapability)
	}
	var extCap ExternalCapability
	err := json.Unmarshal([]byte(extCapability), &extCap)
	if err != nil {
		return nil, err
	}

	//ensure PriceScaling is not 0
	if extCap.PriceScaling == 0 {
		extCap.PriceScaling = 1
	}
	extCap.price, err = NewAutoConvertedPrice(extCap.PriceCurrency, big.NewRat(extCap.PricePerUnit, extCap.PriceScaling), func(price *big.Rat) {
		glog.V(6).Infof("Capability %s price set to %s wei per compute unit", extCap.Name, price.FloatString(3))
	})

	if err != nil {
		panic(fmt.Errorf("error converting price: %v", err))
	}
	if cap, ok := extCaps.Capabilities[extCap.Name]; ok {
		cap.Url = extCap.Url
		cap.Capacity = extCap.Capacity
		cap.price = extCap.price
	}

	extCaps.Capabilities[extCap.Name] = &extCap

	return &extCap, err
}

func (extCap *ExternalCapability) GetPrice() *big.Rat {
	extCap.mu.RLock()
	defer extCap.mu.RUnlock()
	return extCap.price.Value()
}
