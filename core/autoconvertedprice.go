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
var PriceFeedWatcher *watchers.PriceFeedWatcher

// Number of wei in 1 ETH
var weiPerETH = big.NewRat(1e18, 1)

type AutoConvertedPrice struct {
	cancelSubscription func()
	onUpdate           func(*big.Rat)
	basePrice          *big.Rat

	mu      sync.RWMutex
	current *big.Rat
}

func NewFixedPrice(price *big.Rat) *AutoConvertedPrice {
	return &AutoConvertedPrice{current: price}
}

func NewAutoConvertedPrice(currency string, basePrice *big.Rat, onUpdate func(*big.Rat)) (*AutoConvertedPrice, error) {
	if PriceFeedWatcher == nil {
		return nil, fmt.Errorf("PriceFeedWatcher is not initialized")
	}
	if strings.ToLower(currency) == "wei" {
		return NewFixedPrice(basePrice), nil
	} else if strings.ToUpper(currency) == "ETH" {
		return NewFixedPrice(new(big.Rat).Mul(basePrice, weiPerETH)), nil
	}

	base, quote, err := PriceFeedWatcher.Currencies()
	if err != nil {
		return nil, fmt.Errorf("error getting price feed currencies: %v", err)
	}
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
	go price.watch(ctx, base)
	onUpdate(price.current)

	return price, nil
}

func (a *AutoConvertedPrice) Value() *big.Rat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

func (a *AutoConvertedPrice) Stop() {
	if a.cancelSubscription != nil {
		a.cancelSubscription()
	}
}

func (a *AutoConvertedPrice) watch(ctx context.Context, baseCurrency string) {
	priceUpdated := make(chan eth.PriceData, 1)
	PriceFeedWatcher.Subscribe(ctx, priceUpdated)
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
}

func currencyToWeiMultiplier(data eth.PriceData, baseCurrency string) *big.Rat {
	ethMultipler := data.Price
	if baseCurrency == "ETH" {
		// Invert the multiplier if the quote is in the form ETH / X
		ethMultipler = new(big.Rat).Inv(ethMultipler)
	}
	return new(big.Rat).Mul(ethMultipler, weiPerETH)
}
