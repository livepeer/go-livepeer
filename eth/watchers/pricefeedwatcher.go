package watchers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/eth"
)

const (
	priceUpdateMaxRetries     = 5
	priceUpdateBaseRetryDelay = 30 * time.Second
	priceUpdatePeriod         = 1 * time.Hour
)

// PriceFeedWatcher monitors a Chainlink PriceFeed for updated pricing info. It
// allows fetching the current price as well as listening for updates on the
// PriceUpdated channel.
type PriceFeedWatcher struct {
	baseRetryDelay time.Duration

	priceFeed                   eth.PriceFeedEthClient
	currencyBase, currencyQuote string

	mu             sync.RWMutex
	current        eth.PriceData
	priceEventFeed event.Feed
}

// NewPriceFeedWatcher creates a new PriceFeedWatcher instance. It will already
// fetch the current price and start a goroutine to watch for updates.
func NewPriceFeedWatcher(ethClient *ethclient.Client, priceFeedAddr string) (*PriceFeedWatcher, error) {
	priceFeed, err := eth.NewPriceFeedEthClient(ethClient, priceFeedAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create price feed client: %w", err)
	}

	description, err := priceFeed.Description()
	if err != nil {
		return nil, fmt.Errorf("failed to get description: %w", err)
	}

	currencyFrom, currencyTo, err := parseCurrencies(description)
	if err != nil {
		return nil, err
	}

	w := &PriceFeedWatcher{
		baseRetryDelay: priceUpdateBaseRetryDelay,
		priceFeed:      priceFeed,
		currencyBase:   currencyFrom,
		currencyQuote:  currencyTo,
	}

	err = w.updatePrice()
	if err != nil {
		return nil, fmt.Errorf("failed to update price: %w", err)
	}

	return w, nil
}

// Currencies returns the base and quote currencies of the price feed.
// i.e. base = CurrentPrice() * quote
func (w *PriceFeedWatcher) Currencies() (base string, quote string) {
	return w.currencyBase, w.currencyQuote
}

// Current returns the latest fetched price data.
func (w *PriceFeedWatcher) Current() eth.PriceData {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

// Subscribe allows one to subscribe to price updates emitted by the Watcher.
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other
// subscribers. Slow subscribers are not dropped.
func (w *PriceFeedWatcher) Subscribe(sub chan<- eth.PriceData) event.Subscription {
	return w.priceEventFeed.Subscribe(sub)
}

func (w *PriceFeedWatcher) updatePrice() error {
	newPrice, err := w.priceFeed.FetchPriceData()
	if err != nil {
		return fmt.Errorf("failed to fetch price data: %w", err)
	}

	if newPrice.UpdatedAt.After(w.current.UpdatedAt) {
		w.mu.Lock()
		w.current = newPrice
		w.mu.Unlock()
		w.priceEventFeed.Send(newPrice)
	}

	return nil
}

// Watch starts the watch process. It will periodically poll the price feed for
// price updates until the given context is canceled. Typically, you want to
// call Watch inside a goroutine.
func (w *PriceFeedWatcher) Watch(ctx context.Context) {
	ticker := newTruncatedTicker(ctx, priceUpdatePeriod)
	w.watchTicker(ctx, ticker)
}

func (w *PriceFeedWatcher) watchTicker(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			attempt, retryDelay := 1, w.baseRetryDelay
			for {
				err := w.updatePrice()
				if err == nil {
					break
				} else if attempt >= priceUpdateMaxRetries {
					clog.Errorf(ctx, "Failed to fetch updated price from PriceFeed attempts=%d err=%q", attempt, err)
					break
				}

				clog.Warningf(ctx, "Failed to fetch updated price from PriceFeed, retrying after retryDelay=%d attempt=%d err=%q", retryDelay, attempt, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
				}
				attempt, retryDelay = attempt+1, retryDelay*2
			}
		}
	}
}

// parseCurrencies parses the base and quote currencies from a price feed based
// on Chainlink PriceFeed description pattern "FROM / TO".
func parseCurrencies(description string) (currencyBase string, currencyQuote string, err error) {
	currencies := strings.Split(description, "/")
	if len(currencies) != 2 {
		return "", "", fmt.Errorf("aggregator description must be in the format 'FROM / TO' but got: %s", description)
	}

	currencyBase = strings.TrimSpace(currencies[0])
	currencyQuote = strings.TrimSpace(currencies[1])
	return
}

// newTruncatedTicker creates a ticker that ticks at the next time that is a
// multiple of d, starting from the current time.
func newTruncatedTicker(ctx context.Context, d time.Duration) <-chan time.Time {
	return newTruncatedTickerMockable(ctx, d, time.Now, time.After)
}

// Test helper so we can mock the time functions in tests.
func newTruncatedTickerMockable(ctx context.Context, d time.Duration, now func() time.Time, tickAfter func(d time.Duration) <-chan time.Time) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		defer close(ch)

		nextTick := now().UTC().Truncate(d)
		for {
			nextTick = nextTick.Add(d)
			untilNextTick := nextTick.Sub(now().UTC())
			if untilNextTick <= 0 {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case t := <-tickAfter(untilNextTick):
				ch <- t
			}
		}
	}()

	return ch
}
