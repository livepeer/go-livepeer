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

type PriceFeedWatcher interface {
	Currencies() (base string, quote string, err error)
	Current() (eth.PriceData, error)
	Subscribe(ctx context.Context, sink chan<- eth.PriceData)
}

// PriceFeedWatcher monitors a Chainlink PriceFeed for updated pricing info. It
// allows fetching the current price as well as listening for updates on the
// PriceUpdated channel.
type priceFeedWatcher struct {
	baseRetryDelay time.Duration

	priceFeed eth.PriceFeedEthClient

	mu          sync.RWMutex
	current     eth.PriceData
	cancelWatch func()

	priceEventFeed event.Feed
	subscriptions  event.SubscriptionScope
}

// NewPriceFeedWatcher creates a new PriceFeedWatcher instance. It will already
// fetch the current price and start a goroutine to watch for updates.
func NewPriceFeedWatcher(ethClient *ethclient.Client, priceFeedAddr string) (PriceFeedWatcher, error) {
	priceFeed, err := eth.NewPriceFeedEthClient(ethClient, priceFeedAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create price feed client: %w", err)
	}
	w := &priceFeedWatcher{
		baseRetryDelay: priceUpdateBaseRetryDelay,
		priceFeed:      priceFeed,
	}
	return w, nil
}

// Currencies returns the base and quote currencies of the price feed.
// i.e. base = CurrentPrice() * quote
func (w *priceFeedWatcher) Currencies() (base string, quote string, err error) {
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
func (w *priceFeedWatcher) Current() (eth.PriceData, error) {
	w.mu.RLock()
	current := w.current
	w.mu.RUnlock()
	if current.UpdatedAt.IsZero() {
		return w.updatePrice()
	}
	return current, nil
}

// Subscribe allows one to subscribe to price updates emitted by the Watcher.
// The sink channel should have ample buffer space to avoid blocking other
// subscribers. Slow subscribers are not dropped. The subscription is kept alive
// until the passed Context is cancelled.
//
// The watch loop is run automatically while there are active subscriptions. It
// will be started when the first subscription is made and is automatically
// stopped when the last subscription is closed.
func (w *priceFeedWatcher) Subscribe(ctx context.Context, sink chan<- eth.PriceData) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ensureWatchLocked()

	sub := w.subscriptions.Track(w.priceEventFeed.Subscribe(sink))
	go w.handleUnsubscribe(ctx, sub)
}

// updatePrice fetches the latest price data from the price feed and updates the
// current price if it is newer. If the price is updated, it will also send the
// updated price to the price event feed.
func (w *priceFeedWatcher) updatePrice() (eth.PriceData, error) {
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

// ensureWatchLocked makes sure that the watch process is running. It assumes it
// is already running in a locked context (w.mu). The watch process itself will
// run in background and periodically poll the price feed for updates until the
// `w.cancelWatch` function is called.
func (w *priceFeedWatcher) ensureWatchLocked() {
	if w.cancelWatch != nil {
		// already running
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelWatch = cancel

	ticker := newTruncatedTicker(ctx, priceUpdatePeriod)
	go w.watchTicker(ctx, ticker)
}

// handleUnsubscribe waits for the provided Context to be done and then closes
// the given subscription. It then stops the watch process if there are no more
// active subscriptions.
func (w *priceFeedWatcher) handleUnsubscribe(ctx context.Context, sub event.Subscription) {
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-sub.Err():
			clog.Errorf(ctx, "PriceFeedWatcher subscription error: %v", sub.Err())
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	sub.Unsubscribe()
	if w.subscriptions.Count() == 0 && w.cancelWatch != nil {
		w.cancelWatch()
		w.cancelWatch = nil
	}
}

// watchTicker is the main loop that periodically fetches the latest price data
// from the price feed. It's lifecycle is handled through the ensureWatch and
// handleUnsubscribe functions.
func (w *priceFeedWatcher) watchTicker(ctx context.Context, ticker <-chan time.Time) {
	clog.V(6).Infof(ctx, "Starting PriceFeed watch loop")
	for {
		select {
		case <-ctx.Done():
			clog.V(6).Infof(ctx, "Stopping PriceFeed watch loop")
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
// multiple of d, starting from the current time. This is a best-effort approach
// to ensure that nodes update their prices around the same time to avoid too
// big price discrepancies.
func newTruncatedTicker(ctx context.Context, d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		// Do not close the channel, to prevent a concurrent goroutine reading from
		// the channel from seeing an erroneous "tick" after its closed.

		nextTick := time.Now().UTC().Truncate(d)
		for {
			nextTick = nextTick.Add(d)
			untilNextTick := nextTick.Sub(time.Now().UTC())
			if untilNextTick <= 0 {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case t := <-time.After(untilNextTick):
				ch <- t
			}
		}
	}()

	return ch
}
