package watchers

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/eth/contracts/chainlink"
)

const (
	priceUpdateMaxRetries     = 5
	priceUpdateBaseRetryDelay = 30 * time.Second
)

type PriceFeedWatcher struct {
	ctx context.Context

	client *ethclient.Client
	proxy  *chainlink.AggregatorV3Interface

	currencyBase, currencyQuote string

	current      PriceData
	priceUpdated chan PriceData
}

type PriceData struct {
	RoundId   int64
	Price     *big.Rat
	UpdatedAt time.Time
}

func NewPriceFeedWatcher(ctx context.Context, rpcUrl, proxyAddrStr string) (*PriceFeedWatcher, error) {
	// Initialize client instance using the rpcUrl.
	client, err := ethclient.DialContext(ctx, rpcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	// Test if it is a contract address.
	ok := isContractAddress(proxyAddrStr, client)
	if !ok {
		return nil, fmt.Errorf("not a contract address: %s", proxyAddrStr)
	}

	proxyAddr := common.HexToAddress(proxyAddrStr)
	proxy, err := chainlink.NewAggregatorV3Interface(proxyAddr, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create mock aggregator proxy: %w", err)
	}

	description, err := proxy.Description(&bind.CallOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to get description: %w", err)
	}

	currencyFrom, currencyTo, err := parseCurrencies(description)
	if err != nil {
		return nil, err
	}

	w := &PriceFeedWatcher{
		ctx:           ctx,
		client:        client,
		proxy:         proxy,
		currencyBase:  currencyFrom,
		currencyQuote: currencyTo,
		priceUpdated:  make(chan PriceData, 1),
	}

	err = w.fetchPrice()
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

func (w *PriceFeedWatcher) Current() PriceData {
	return w.current
}

func (w *PriceFeedWatcher) PriceUpdated() <-chan PriceData {
	return w.priceUpdated
}

func (w *PriceFeedWatcher) fetchPrice() error {
	roundData, err := w.proxy.LatestRoundData(&bind.CallOpts{})
	if err != nil {
		return fmt.Errorf("failed to get latest round data: %w", err)
	}

	decimals, err := w.proxy.Decimals(&bind.CallOpts{})
	if err != nil {
		return fmt.Errorf("failed to get decimals: %w", err)
	}

	w.updatePrice(roundData.Answer, decimals, roundData.RoundId, roundData.UpdatedAt)
	return nil
}

func (w *PriceFeedWatcher) updatePrice(current *big.Int, decimals uint8, roundId, updatedAt *big.Int) {
	// Compute a big.int which is 10^decimals.
	divisor := new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(decimals)),
		nil)

	newPrice := PriceData{
		RoundId:   roundId.Int64(),
		Price:     new(big.Rat).SetFrac(current, divisor),
		UpdatedAt: time.Unix(updatedAt.Int64(), 0),
	}

	if newPrice.UpdatedAt.After(w.current.UpdatedAt) {
		w.current = newPrice
		select {
		case w.priceUpdated <- newPrice:
		default:
		}
	}
}

func (w *PriceFeedWatcher) watch() {
	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()
	ticker := newTruncatedTicker(ctx, 1*time.Hour)

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker:
			retryDelay := priceUpdateBaseRetryDelay
			for attempt := 1; attempt <= priceUpdateMaxRetries; attempt++ {
				err := w.fetchPrice()
				if err == nil {
					break
				}

				clog.Warningf(ctx, "Failed to fetch updated price from PriceFeed, retrying after retryDelay=%d attempt=%d err=%v", retryDelay, attempt, err)
				time.Sleep(retryDelay)
				retryDelay *= 2
			}
		}
	}
}

func isContractAddress(addr string, client *ethclient.Client) bool {
	if len(addr) == 0 {
		return false
	}

	// Ensure it is an Ethereum address: 0x followed by 40 hexadecimal characters.
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	if !re.MatchString(addr) {
		return false
	}

	// Ensure it is a contract address.
	address := common.HexToAddress(addr)
	bytecode, err := client.CodeAt(context.Background(), address, nil) // nil is latest block
	if err != nil {
		return false
	}
	isContract := len(bytecode) > 0
	return isContract
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
