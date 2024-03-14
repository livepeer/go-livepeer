package watchers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/eth"
)

const (
	priceUpdateMaxRetries     = 5
	priceUpdateBaseRetryDelay = 30 * time.Second
)

type PriceFeedWatcher struct {
	ctx context.Context

	updatePeriod                time.Duration
	priceFeed                   eth.PriceFeedEthClient
	currencyBase, currencyQuote string

	current      eth.PriceData
	priceUpdated chan eth.PriceData
}

func NewPriceFeedWatcher(ctx context.Context, rpcUrl, priceFeedAddr string, updatePeriod time.Duration) (*PriceFeedWatcher, error) {
	if updatePeriod <= 0 {
		updatePeriod = 1 * time.Hour
	}

	priceFeed, err := eth.NewPriceFeedEthClient(ctx, rpcUrl, priceFeedAddr)
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
		ctx:           ctx,
		updatePeriod:  updatePeriod,
		priceFeed:     priceFeed,
		currencyBase:  currencyFrom,
		currencyQuote: currencyTo,
		priceUpdated:  make(chan eth.PriceData, 1),
	}

	err = w.updatePrice()
	if err != nil {
		return nil, fmt.Errorf("failed to update price: %w", err)
	}
	go w.watch()

	return w, nil
}

// Currencies returns the base and quote currencies of the price feed.
// i.e. base = CurrentPrice() * quote
func (w *PriceFeedWatcher) Currencies() (base string, quote string) {
	return w.currencyBase, w.currencyQuote
}

func (w *PriceFeedWatcher) Current() eth.PriceData {
	return w.current
}

func (w *PriceFeedWatcher) PriceUpdated() <-chan eth.PriceData {
	return w.priceUpdated
}

func (w *PriceFeedWatcher) updatePrice() error {
	newPrice, err := w.priceFeed.FetchPriceData()
	if err != nil {
		return fmt.Errorf("failed to fetch price data: %w", err)
	}

	if newPrice.UpdatedAt.After(w.current.UpdatedAt) {
		w.current = newPrice
		select {
		case w.priceUpdated <- newPrice:
		default:
		}
	}

	return nil
}

func (w *PriceFeedWatcher) watch() {
	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()
	ticker := newTruncatedTicker(ctx, w.updatePeriod)

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker:
			attempt, retryDelay := 1, priceUpdateBaseRetryDelay
			for {
				err := w.updatePrice()
				if err == nil {
					break
				} else if attempt >= priceUpdateMaxRetries {
					clog.Errorf(ctx, "Failed to fetch updated price from PriceFeed attempts=%d err=%q", attempt, err)
					break
				}

				clog.Warningf(ctx, "Failed to fetch updated price from PriceFeed, retrying after retryDelay=%d attempt=%d err=%q", retryDelay, attempt, err)
				time.Sleep(retryDelay)
				attempt, retryDelay = attempt+1, retryDelay*2
			}
		}
	}
}

func parseCurrencies(description string) (currencyBase string, currencyQuote string, err error) {
	currencies := strings.Split(description, "/")
	if len(currencies) != 2 {
		return "", "", fmt.Errorf("aggregator description must be in the format 'FROM / TO' but got: %s", description)
	}

	currencyBase = strings.TrimSpace(currencies[0])
	currencyQuote = strings.TrimSpace(currencies[1])
	return
}

func newTruncatedTicker(ctx context.Context, d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	go func() {
		defer close(ch)

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
