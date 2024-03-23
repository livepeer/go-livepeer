package watchers

import (
	"context"
	"errors"
	"math/big"
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

	w := &PriceFeedWatcher{
		priceFeed:     priceFeedMock,
		currencyBase:  "ETH",
		currencyQuote: "USD",
	}

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

func TestPriceFeedWatcher_Watch(t *testing.T) {
	require := require.New(t)
	priceFeedMock := new(mockPriceFeedEthClient)
	defer priceFeedMock.AssertExpectations(t)

	w := &PriceFeedWatcher{
		priceFeed:     priceFeedMock,
		currencyBase:  "ETH",
		currencyQuote: "USD",
	}

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

	w := &PriceFeedWatcher{
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

func TestNewTruncatedTicker(t *testing.T) {
	testTimeout := time.After(10 * time.Second)
	require := require.New(t)
	newTime := func(s string) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		require.NoError(err)
		return t
	}
	testCases := []struct {
		Period        time.Duration
		StartTime     time.Time
		ExpectedTicks []time.Time
	}{
		{
			Period:    1 * time.Minute,
			StartTime: newTime("2021-01-01T13:02:15Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T13:03:00Z"),
				newTime("2021-01-01T13:04:00Z"),
				newTime("2021-01-01T13:05:00Z"),
				newTime("2021-01-01T13:06:00Z"),
				newTime("2021-01-01T13:07:00Z"),
			},
		},
		{
			Period:    5 * time.Minute,
			StartTime: newTime("2021-01-01T16:19:45Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T16:20:00Z"),
				newTime("2021-01-01T16:25:00Z"),
				newTime("2021-01-01T16:30:00Z"),
				newTime("2021-01-01T16:35:00Z"),
				newTime("2021-01-01T16:40:00Z"),
			},
		},
		{
			Period:    15 * time.Minute,
			StartTime: newTime("2021-01-01T19:51:17Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T20:00:00Z"),
				newTime("2021-01-01T20:15:00Z"),
				newTime("2021-01-01T20:30:00Z"),
				newTime("2021-01-01T20:45:00Z"),
				newTime("2021-01-01T21:00:00Z"),
			},
		},
		{
			Period:    1 * time.Hour,
			StartTime: newTime("2021-01-01T22:30:00Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T23:00:00Z"),
				newTime("2021-01-02T00:00:00Z"),
				newTime("2021-01-02T01:00:00Z"),
				newTime("2021-01-02T02:00:00Z"),
				newTime("2021-01-02T03:00:00Z"),
			},
		},
		{
			Period:    4 * time.Hour,
			StartTime: newTime("2021-01-02T03:00:00Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-02T04:00:00Z"),
				newTime("2021-01-02T08:00:00Z"),
				newTime("2021-01-02T12:00:00Z"),
				newTime("2021-01-02T16:00:00Z"),
				newTime("2021-01-02T20:00:00Z"),
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Period.String(), func(t *testing.T) {
			var (
				currTime = testCase.StartTime
				mockNow  = func() time.Time {
					return currTime
				}
				mockAfter = func(d time.Duration) <-chan time.Time {
					// calculate it a little differently from the impl here
					expected := testCase.Period - time.Duration(currTime.UnixNano()%testCase.Period.Nanoseconds())*time.Nanosecond
					require.Equal(expected, d, "expected duration to be truncated")

					// time gets updated automatically when After is called
					currTime = currTime.Add(d)

					result := make(chan time.Time, 1)
					result <- currTime
					return result
				}
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ticker := newTruncatedTickerMockable(ctx, testCase.Period, mockNow, mockAfter)

			for _, expectedTick := range testCase.ExpectedTicks {
				select {
				case receivedTick := <-ticker:
					require.Equal(expectedTick, receivedTick)
				case <-testTimeout:
					require.Fail("timed out receiving tick")
				}
			}
		})
	}
}
