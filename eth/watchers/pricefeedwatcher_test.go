package watchers

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockPriceFeedEthClient struct {
	mock.Mock
}

func (m *mockPriceFeedEthClient) FetchPriceData() (eth.PriceData, error) {
	args := m.Called()
	return args.Get(0).(eth.PriceData), args.Error(1)
}

func (m *mockPriceFeedEthClient) Description() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestPriceFeedWatcher_UpdatePrice(t *testing.T) {
	priceFeedMock := new(mockPriceFeedEthClient)
	defer priceFeedMock.AssertExpectations(t)

	priceData := eth.PriceData{
		RoundID:   10,
		Price:     big.NewRat(3, 2),
		UpdatedAt: time.Now(),
	}
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()

	w := &priceFeedWatcher{priceFeed: priceFeedMock}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	priceUpdated := make(chan eth.PriceData, 1)
	w.Subscribe(ctx, priceUpdated)

	newPrice, err := w.updatePrice()
	require.NoError(t, err)
	require.Equal(t, priceData, newPrice)

	select {
	case updatedPrice := <-priceUpdated:
		require.Equal(t, priceData, updatedPrice)
	case <-time.After(2 * time.Second):
		t.Error("Updated price hasn't been received on channel")
	}
}

func TestPriceFeedWatcher_Subscribe(t *testing.T) {
	require := require.New(t)
	priceFeedMock := new(mockPriceFeedEthClient)
	defer priceFeedMock.AssertExpectations(t)

	w := &priceFeedWatcher{priceFeed: priceFeedMock}

	// Start a bunch of subscriptions and make sure only 1 watch loop gets started
	observedCancelWatch := []context.CancelFunc{}
	cancelSub := []context.CancelFunc{}
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		w.Subscribe(ctx, make(chan eth.PriceData, 1))

		observedCancelWatch = append(observedCancelWatch, w.cancelWatch)
		cancelSub = append(cancelSub, cancel)
	}

	require.NotNil(w.cancelWatch)
	for i := range observedCancelWatch {
		require.Equal(reflect.ValueOf(w.cancelWatch).Pointer(), reflect.ValueOf(observedCancelWatch[i]).Pointer())
	}

	// Stop all but the last subscription and ensure watch loop stays running
	for i := 0; i < 4; i++ {
		cancelSub[i]()
		require.NotNil(w.cancelWatch)
	}

	// Now stop the last subscription and ensure watch loop gets stopped
	cancelSub[4]()
	time.Sleep(1 * time.Second)
	require.Nil(w.cancelWatch)

	// Finally, just make sure it can be started again after having been stopped
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Subscribe(ctx, make(chan eth.PriceData, 1))
	require.NotNil(w.cancelWatch)
}

func TestPriceFeedWatcher_Watch(t *testing.T) {
	require := require.New(t)
	priceFeedMock := new(mockPriceFeedEthClient)
	defer priceFeedMock.AssertExpectations(t)

	w := &priceFeedWatcher{priceFeed: priceFeedMock}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	priceUpdated := make(chan eth.PriceData, 1)
	w.Subscribe(ctx, priceUpdated)

	priceData := eth.PriceData{
		RoundID:   10,
		Price:     big.NewRat(9, 2),
		UpdatedAt: time.Now(),
	}
	checkPriceUpdated := func() {
		select {
		case updatedPrice := <-priceUpdated:
			require.Equal(priceData, updatedPrice)
			require.Equal(priceData, w.current)
		case <-time.After(1 * time.Second):
			require.Fail("Updated price hasn't been received on channel in a timely manner")
		}
		priceFeedMock.AssertExpectations(t)
	}
	checkNoPriceUpdate := func() {
		select {
		case <-priceUpdated:
			require.Fail("Unexpected price update given it hasn't changed")
		case <-time.After(1 * time.Second):
			// all good
		}
		priceFeedMock.AssertExpectations(t)
	}

	// Start the watch loop
	fakeTicker := make(chan time.Time, 10)
	go func() {
		w.watchTicker(ctx, fakeTicker)
	}()

	// First time should trigger an update
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()
	fakeTicker <- time.Now()
	checkPriceUpdated()

	// Trigger a dummy update given price hasn't changed
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()
	fakeTicker <- time.Now()
	checkNoPriceUpdate()

	// still shouldn't update given UpdatedAt stayed the same
	priceData.Price = big.NewRat(1, 1)
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()
	fakeTicker <- time.Now()
	checkNoPriceUpdate()

	// bump the UpdatedAt time to trigger an update
	priceData.UpdatedAt = priceData.UpdatedAt.Add(1 * time.Minute)
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()
	fakeTicker <- time.Now()
	checkPriceUpdated()

	priceData.UpdatedAt = priceData.UpdatedAt.Add(1 * time.Hour)
	priceData.Price = big.NewRat(3, 2)
	priceFeedMock.On("FetchPriceData").Return(priceData, nil).Once()
	fakeTicker <- time.Now()
	checkPriceUpdated()
}

func TestPriceFeedWatcher_WatchErrorRetries(t *testing.T) {
	priceFeedMock := new(mockPriceFeedEthClient)
	defer priceFeedMock.AssertExpectations(t)

	// First 4 calls should fail then succeed on the 5th
	for i := 0; i < 4; i++ {
		priceFeedMock.On("FetchPriceData").Return(eth.PriceData{}, errors.New("error")).Once()
	}
	priceData := eth.PriceData{
		RoundID:   10,
		Price:     big.NewRat(3, 2),
		UpdatedAt: time.Now(),
	}
	priceFeedMock.On("FetchPriceData").Return(priceData, nil)

	w := &priceFeedWatcher{
		baseRetryDelay: 5 * time.Millisecond,
		priceFeed:      priceFeedMock,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	priceUpdated := make(chan eth.PriceData, 1)
	w.Subscribe(ctx, priceUpdated)

	// Start watch loop
	fakeTicker := make(chan time.Time, 10)
	go func() {
		w.watchTicker(ctx, fakeTicker)
	}()

	fakeTicker <- time.Now()
	select {
	case updatedPrice := <-priceUpdated:
		require.Equal(t, priceData, updatedPrice)
	case <-time.After(2 * time.Second):
		t.Error("Updated price hasn't been received on channel")
	}
}

func TestParseCurrencies(t *testing.T) {
	t.Run("Valid currencies", func(t *testing.T) {
		description := "ETH / USD"
		currencyBase, currencyQuote, err := parseCurrencies(description)

		require.NoError(t, err)
		require.Equal(t, "ETH", currencyBase)
		require.Equal(t, "USD", currencyQuote)
	})

	t.Run("Missing separator", func(t *testing.T) {
		description := "ETHUSD"
		_, _, err := parseCurrencies(description)

		require.Error(t, err)
		require.Contains(t, err.Error(), "aggregator description must be in the format 'FROM / TO'")
	})

	t.Run("Extra spaces", func(t *testing.T) {
		description := "   ETH    /  USD  "
		currencyBase, currencyQuote, err := parseCurrencies(description)

		require.NoError(t, err)
		require.Equal(t, "ETH", currencyBase)
		require.Equal(t, "USD", currencyQuote)
	})

	t.Run("Lowercase currency", func(t *testing.T) {
		description := "eth / usd"
		currencyBase, currencyQuote, err := parseCurrencies(description)

		require.NoError(t, err)
		require.Equal(t, "eth", currencyBase)
		require.Equal(t, "usd", currencyQuote)
	})
}
