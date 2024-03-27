package core

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/watchers"
)

// PriceFeedWatcher is a global instance of a PriceFeedWatcher. It must be
// initialized before creating an AutoConvertedPrice instance.
var PriceFeedWatcher watchers.PriceFeedWatcher

// Number of wei in 1 ETH
var weiPerETH = big.NewRat(1e18, 1)

// AutoConvertedPrice represents a price that is automatically converted to wei
// based on the current price of ETH in a given currency. It uses the static
// PriceFeedWatcher that must be configured before creating an instance.
type AutoConvertedPrice struct {
	cancelSubscription func()
	onUpdate           func(*big.Rat)
	basePrice          *big.Rat

	mu      sync.RWMutex
	current *big.Rat
}

// NewFixedPrice creates a new AutoConvertedPrice with a fixed price in wei.
func NewFixedPrice(price *big.Rat) *AutoConvertedPrice {
	return &AutoConvertedPrice{current: price}
}

// NewAutoConvertedPrice creates a new AutoConvertedPrice instance with the given
// currency and base price. The onUpdate function is optional and gets called
// whenever the price is updated (also with the initial price). The Stop function
// must be called to free resources when the price is no longer needed.
func NewAutoConvertedPrice(currency string, basePrice *big.Rat, onUpdate func(*big.Rat)) (*AutoConvertedPrice, error) {
	if onUpdate == nil {
		onUpdate = func(*big.Rat) {}
	}

	// Default currency (wei/eth) doesn't need the conversion loop
	if lcurr := strings.ToLower(currency); lcurr == "" || lcurr == "wei" || lcurr == "eth" {
		price := basePrice
		if lcurr == "eth" {
			price = new(big.Rat).Mul(basePrice, weiPerETH)
		}
		onUpdate(price)
		return NewFixedPrice(price), nil
	}

	if PriceFeedWatcher == nil {
		return nil, fmt.Errorf("PriceFeedWatcher is not initialized")
	}

	base, quote, err := PriceFeedWatcher.Currencies()
	if err != nil {
		return nil, fmt.Errorf("error getting price feed currencies: %v", err)
	}
	base, quote, currency = strings.ToUpper(base), strings.ToUpper(quote), strings.ToUpper(currency)
	if base != "ETH" && quote != "ETH" {
		return nil, fmt.Errorf("price feed does not have ETH as a currency (%v/%v)", base, quote)
	}
	if base != currency && quote != currency {
		return nil, fmt.Errorf("price feed does not have %v as a currency (%v/%v)", currency, base, quote)
	}

	currencyPrice, err := PriceFeedWatcher.Current()
	if err != nil {
		return nil, fmt.Errorf("error getting current price data: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	price := &AutoConvertedPrice{
		cancelSubscription: cancel,
		onUpdate:           onUpdate,
		basePrice:          basePrice,
		current:            new(big.Rat).Mul(basePrice, currencyToWeiMultiplier(currencyPrice, base)),
	}
	// Trigger the initial update with the current price
	onUpdate(price.current)

	price.startAutoConvertLoop(ctx, base)

	return price, nil
}

// Value returns the current price in wei.
func (a *AutoConvertedPrice) Value() *big.Rat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

// Stop unsubscribes from the price feed and frees resources from the
// auto-conversion loop.
func (a *AutoConvertedPrice) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelSubscription != nil {
		a.cancelSubscription()
		a.cancelSubscription = nil
	}
}

func (a *AutoConvertedPrice) startAutoConvertLoop(ctx context.Context, baseCurrency string) {
	priceUpdated := make(chan eth.PriceData, 1)
	PriceFeedWatcher.Subscribe(ctx, priceUpdated)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case currencyPrice := <-priceUpdated:
				a.mu.Lock()
				a.current = new(big.Rat).Mul(a.basePrice, currencyToWeiMultiplier(currencyPrice, baseCurrency))
				a.mu.Unlock()

				a.onUpdate(a.current)
			}
		}
	}()
}

// currencyToWeiMultiplier calculates the multiplier to convert the value
// specified in the custom currency to wei.
func currencyToWeiMultiplier(data eth.PriceData, baseCurrency string) *big.Rat {
	ethMultipler := data.Price
	if baseCurrency == "ETH" {
		// Invert the multiplier if the quote is in the form ETH / X
		ethMultipler = new(big.Rat).Inv(ethMultipler)
	}
	return new(big.Rat).Mul(ethMultipler, weiPerETH)
}
