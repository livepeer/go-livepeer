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

	priceFeed eth.PriceFeedEthClient

	mu                          sync.RWMutex
	current                     eth.PriceData
	cancelWatch                 func()

	priceEventFeed event.Feed
	subscriptions  event.SubscriptionScope
}

// NewPriceFeedWatcher creates a new PriceFeedWatcher instance. It will already
// fetch the current price and start a goroutine to watch for updates.
func NewPriceFeedWatcher(ethClient *ethclient.Client, priceFeedAddr string) (*PriceFeedWatcher, error) {
	priceFeed, err := eth.NewPriceFeedEthClient(ethClient, priceFeedAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create price feed client: %w", err)
	}
	w := &PriceFeedWatcher{
		baseRetryDelay: priceUpdateBaseRetryDelay,
		priceFeed:      priceFeed,
	}
	return w, nil
}

// Currencies returns the base and quote currencies of the price feed.
// i.e. base = CurrentPrice() * quote
func (w *PriceFeedWatcher) Currencies() (base string, quote string, err error) {
	description, err := w.priceFeed.Description()
	if err != nil {
		return "", "", fmt.Errorf("failed to get description: %w", err)
	}

	base, quote, err = parseCurrencies(description)
	if err != nil {
		return "", "", err
	}
	return
}

// Current returns the latest fetched price data, or fetches it in case it has
// not been fetched yet.
func (w *PriceFeedWatcher) Current() (eth.PriceData, error) {
	w.mu.RLock()
	current := w.current
	w.mu.RUnlock()
	if current.UpdatedAt.IsZero() {
		return w.updatePrice()
	}
	return current, nil
}

// Subscribe allows one to subscribe to price updates emitted by the Watcher.
// To unsubscribe, simply call `Unsubscribe` on the returned subscription.
// The sink channel should have ample buffer space to avoid blocking other
// subscribers. Slow subscribers are not dropped.
func (w *PriceFeedWatcher) Subscribe(ctx context.Context, sink chan<- eth.PriceData) {
	w.mu.Lock()
	sub := w.subscriptions.Track(w.priceEventFeed.Subscribe(sink))
	w.mu.Unlock()

	go func() {
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case <-sub.Err():
				clog.Errorf(ctx, "PriceFeedWatcher subscription error: %v", sub.Err())
			}
		}
		sub.Unsubscribe()

		w.mu.Lock()
		defer w.mu.Unlock()
		if w.subscriptions.Count() == 0 {
			w.cancelWatch()
			w.cancelWatch = nil
		}
	}()

	w.ensureWatch()
}

// updatePrice fetches the latest price data from the price feed and updates the
// current price if it is newer. If the price is updated, it will also send the
// updated price to the price event feed.
func (w *PriceFeedWatcher) updatePrice() (eth.PriceData, error) {
	newPrice, err := w.priceFeed.FetchPriceData()
	if err != nil {
		return eth.PriceData{}, fmt.Errorf("failed to fetch price data: %w", err)
	}

	if newPrice.UpdatedAt.After(w.current.UpdatedAt) {
		w.mu.Lock()
		w.current = newPrice
		w.mu.Unlock()
		w.priceEventFeed.Send(newPrice)
	}

	return newPrice, nil
}

// ensureWatch makes sure that the watch process is runnin. The watch process
// itself will run in background and periodically poll the price feed for
// updates until the price feed context is cancelled.
func (w *PriceFeedWatcher) ensureWatch() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancelWatch != nil {
		// already running
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelWatch = cancel

	ticker := newTruncatedTicker(ctx, priceUpdatePeriod)
	go w.watchTicker(ctx, ticker)
}

func (w *PriceFeedWatcher) watchTicker(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			attempt, retryDelay := 1, w.baseRetryDelay
			for {
				_, err := w.updatePrice()
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
