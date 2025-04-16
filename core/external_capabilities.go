package core

import (
	"encoding/json"
	"math/big"

	"sync"
)

type ExternalCapability struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Url          string `json:"url"`
	Capacity     int    `json:"capacity"`
	PricePerUnit int64  `json:"price_per_unit"`
	PriceScaling int64  `json:"price_scaling"`

	price *big.Rat

	mu   sync.RWMutex
	Load int `json:"load"`
}

type ExternalCapabilities struct {
	capm         sync.Mutex
	Capabilities map[string]*ExternalCapability
}

func NewExternalCapabilities() *ExternalCapabilities {
	return &ExternalCapabilities{Capabilities: make(map[string]*ExternalCapability)}
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
	extCap.price = big.NewRat(extCap.PricePerUnit, extCap.PriceScaling)
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
	return big.NewRat(extCap.PricePerUnit, extCap.PriceScaling)
}
