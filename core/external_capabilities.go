package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
)

type ExternalCapability struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Url           string                 `json:"url"`
	Capacity      int                    `json:"capacity"`
	PricePerUnit  int64                  `json:"price_per_unit"`
	PriceScaling  int64                  `json:"price_scaling"`
	PriceCurrency string                 `json:"currency"`
	Requirements  CapabilityRequirements `json:"requirements"`
	price         *AutoConvertedPrice

	mu   sync.RWMutex
	Load int
}

type CapabilityRequirements struct {
	VideoIngress bool `json:"video_ingress"`
	VideoEgress  bool `json:"video_egress"`
	DataOutput   bool `json:"data_output"`
}

type StreamData struct {
	StreamID   string
	Capability string
	//Gateway fields
	orchToken        interface{}
	OrchUrl          string
	ExcludeOrchs     []string
	OrchPublishUrl   string
	OrchSubscribeUrl string
	OrchControlUrl   string
	OrchEventsUrl    string
	OrchDataUrl      string

	//Orchestrator fields
	Sender ethcommon.Address

	//Stream fields
	Params            interface{}
	ControlPub        interface{}
	StreamCtx         context.Context
	CancelStream      context.CancelFunc
	StreamStartedTime time.Time
}
type ExternalCapabilities struct {
	capm         sync.Mutex
	Capabilities map[string]*ExternalCapability
	Streams      map[string]*StreamData
}

func NewExternalCapabilities() *ExternalCapabilities {
	return &ExternalCapabilities{Capabilities: make(map[string]*ExternalCapability),
		Streams: make(map[string]*StreamData),
	}
}

func (extCaps *ExternalCapabilities) AddStream(streamID string, params interface{}, orchUrl, publishUrl, subscribeUrl, controlUrl, eventsUrl, dataUrl string) error {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()
	_, ok := extCaps.Streams[streamID]
	if ok {
		return fmt.Errorf("stream already exists: %s", streamID)
	}

	//add to streams
	ctx, cancel := context.WithCancel(context.Background())
	extCaps.Streams[streamID] = &StreamData{
		StreamID:         streamID,
		Params:           params,
		StreamCtx:        ctx,
		CancelStream:     cancel,
		OrchUrl:          orchUrl,
		OrchPublishUrl:   publishUrl,
		OrchSubscribeUrl: subscribeUrl,
		OrchEventsUrl:    eventsUrl,
		OrchDataUrl:      dataUrl,
	}

	return nil
}

func (extCaps *ExternalCapabilities) RemoveStream(streamID string) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	delete(extCaps.Streams, streamID)
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

func (extCap *ExternalCapability) ToCapabilities() *Capabilities {
	capConstraints := make(PerCapabilityConstraints)
	capConstraints[Capability_LiveAI].Models = make(ModelConstraints)
	capConstraints[Capability_LiveAI].Models[extCap.Name] = &ModelConstraint{Capacity: extCap.Capacity}

	caps := NewCapabilities([]Capability{Capability_LiveAI}, MandatoryOCapabilities())
	caps.SetPerCapabilityConstraints(capConstraints)

	return caps
}
