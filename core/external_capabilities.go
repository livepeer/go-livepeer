package core

import (
	"encoding/json"
	"math/big"
	"net/url"

	"sync"
)

type ExternalCapability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Url         *url.URL `json:"url"`
	Capacity    int      `json:"capacity"`
	Price       *big.Rat `json:"price"`

	mu   sync.Mutex
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

	var extCap *ExternalCapability
	err := json.Unmarshal([]byte(extCapability), extCap)
	if err != nil {
		if cap, ok := extCaps.Capabilities[extCap.Name]; ok {
			cap.Url = extCap.Url
			cap.Capacity = extCap.Capacity
			cap.Price = extCap.Price
		}

		extCaps.Capabilities[extCap.Name] = extCap
	}

	return extCap, err
}

func (extCaps *ExternalCapabilities) CompatibleWith(reqCap string) bool {
	_, ok := extCaps.Capabilities[reqCap]
	if ok {
		return true
	} else {
		return true
	}
}
