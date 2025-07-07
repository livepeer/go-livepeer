package core

import (
	"fmt"
	"math/big"

	"sync"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
)

type ExternalCapability struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Type          string `json:"type"` // e.g. "batch", "live"
	Url           string `json:"url"`
	Capacity      int    `json:"capacity"`
	PricePerUnit  int64  `json:"price_per_unit"`
	PriceScaling  int64  `json:"price_scaling"`
	PriceCurrency string `json:"currency"`

	price *AutoConvertedPrice

	mu   sync.RWMutex
	Load int
}

type ExternalCapabilities struct {
	capm         sync.Mutex
	Capabilities map[string]*ExternalCapability
	Streams      map[string]*StreamData
}

type StreamData struct {
	StreamID   string
	Capability string
	//Gateway fields
	OrchUrl      string
	ExcludeOrchs []string

	//Orchestrator fields
	Sender ethcommon.Address
	//source stream
	//StreamCtx         context.Context
	//CancelStream      context.CancelFunc
	//StreamRelayServer interface{}
	//streamActiveTime  time.Time
	//result stream
	//WorkerStreamCtx    context.Context
	//WorkerCancelStream context.CancelFunc
	//WorkerRelayServer  interface{}
}

func NewExternalCapabilities() *ExternalCapabilities {
	extCaps := &ExternalCapabilities{
		Capabilities: make(map[string]*ExternalCapability),
		Streams:      make(map[string]*StreamData),
	}
	return extCaps
}

func (extCaps *ExternalCapabilities) RemoveCapability(extCap string) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()

	delete(extCaps.Capabilities, extCap)
}

func (extCaps *ExternalCapabilities) RegisterCapability(extCap ExternalCapability) (*ExternalCapability, error) {
	extCaps.capm.Lock()
	defer extCaps.capm.Unlock()
	if extCaps.Capabilities == nil {
		extCaps.Capabilities = make(map[string]*ExternalCapability)
	}

	//ensure PriceScaling is not 0
	if extCap.PriceScaling == 0 {
		extCap.PriceScaling = 1
	}
	var err error
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
