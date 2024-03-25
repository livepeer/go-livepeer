package core

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewAutoConvertedPrice(t *testing.T) {
	t.Run("PriceFeedWatcher not initialized", func(t *testing.T) {
		_, err := NewAutoConvertedPrice("USD", big.NewRat(1, 1), nil)
		require.Error(t, err)
	})

	watcherMock := NewPriceFeedWatcherMock(t)
	PriceFeedWatcher = watcherMock
	watcherMock.On("Currencies").Return("ETH", "USD", nil)

	t.Run("Fixed price for wei", func(t *testing.T) {
		price, err := NewAutoConvertedPrice("wei", big.NewRat(1, 1), nil)
		require.NoError(t, err)
		require.Equal(t, big.NewRat(1, 1), price.Value())
		require.Nil(t, price.cancelSubscription)
	})

	t.Run("Auto-converted price for ETH", func(t *testing.T) {
		price, err := NewAutoConvertedPrice("ETH", big.NewRat(2, 1), nil)
		require.NoError(t, err)
		require.Equal(t, big.NewRat(2e18, 1), price.Value()) // 2 ETH in wei
		require.Nil(t, price.cancelSubscription)
	})

	t.Run("Auto-converted price for USD", func(t *testing.T) {
		watcherMock.On("Current").Return(eth.PriceData{Price: big.NewRat(100, 1)}, nil)
		watcherMock.On("Subscribe", mock.Anything, mock.Anything).Once()
		price, err := NewAutoConvertedPrice("USD", big.NewRat(2, 1), nil)
		require.NoError(t, err)
		require.Equal(t, big.NewRat(2e16, 1), price.Value()) // 2 USD * 1/100 ETH/USD
		require.NotNil(t, price.cancelSubscription)
		price.Stop()
	})

	t.Run("Currency not supported by feed", func(t *testing.T) {
		_, err := NewAutoConvertedPrice("GBP", big.NewRat(1, 1), nil)
		require.Error(t, err)
	})

	t.Run("Currency ETH not supported by feed", func(t *testing.T) {
		// set up a new mock to change the currencies returned
		watcherMock := NewPriceFeedWatcherMock(t)
		PriceFeedWatcher = watcherMock
		watcherMock.On("Currencies").Return("wei", "USD", nil)

		_, err := NewAutoConvertedPrice("USD", big.NewRat(1, 1), nil)
		require.Error(t, err)
	})

	t.Run("Auto-converted price for inverted quote", func(t *testing.T) {
		// set up a new mock to change the currencies returned
		watcherMock := NewPriceFeedWatcherMock(t)
		PriceFeedWatcher = watcherMock
		watcherMock.On("Currencies").Return("USD", "ETH", nil)
		watcherMock.On("Current").Return(eth.PriceData{Price: big.NewRat(1, 420)}, nil)
		watcherMock.On("Subscribe", mock.Anything, mock.Anything).Once()
		price, err := NewAutoConvertedPrice("USD", big.NewRat(66, 1), nil)
		require.NoError(t, err)
		require.Equal(t, big.NewRat(11e17, 7), price.Value()) // 66 USD * 1/420 ETH/USD
		require.NotNil(t, price.cancelSubscription)
		price.Stop()
	})
}

func TestAutoConvertedPrice_Update(t *testing.T) {
	require := require.New(t)
	watcherMock := NewPriceFeedWatcherMock(t)
	PriceFeedWatcher = watcherMock

	watcherMock.On("Currencies").Return("ETH", "USD", nil)
	watcherMock.On("Current").Return(eth.PriceData{Price: big.NewRat(3000, 1)}, nil)

	priceUpdatedChan := make(chan *big.Rat, 1)
	onUpdate := func(price *big.Rat) {
		priceUpdatedChan <- price
	}

	var sink chan<- eth.PriceData
	watcherMock.On("Subscribe", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sink = args.Get(1).(chan<- eth.PriceData)
	}).Once()

	price, err := NewAutoConvertedPrice("USD", big.NewRat(50, 1), onUpdate)
	require.NoError(err)
	require.NotNil(t, price.cancelSubscription)
	defer price.Stop()
	watcherMock.AssertExpectations(t)

	require.Equal(big.NewRat(5e16, 3), price.Value())      // 50 USD * 1/3000 ETH/USD
	require.Equal(big.NewRat(5e16, 3), <-priceUpdatedChan) // initial update must be sent

	// Simulate a price update
	sink <- eth.PriceData{Price: big.NewRat(6000, 1)}

	select {
	case updatedPrice := <-priceUpdatedChan:
		require.Equal(big.NewRat(5e16, 6), updatedPrice)  // 50 USD * 1/6000 USD/ETH
		require.Equal(big.NewRat(5e16, 6), price.Value()) // must also udpate current value
	case <-time.After(time.Second):
		t.Fatal("Expected price update not received")
	}
}

func TestAutoConvertedPrice_Stop(t *testing.T) {
	require := require.New(t)
	watcherMock := NewPriceFeedWatcherMock(t)
	PriceFeedWatcher = watcherMock

	watcherMock.On("Currencies").Return("ETH", "USD", nil)
	watcherMock.On("Current").Return(eth.PriceData{Price: big.NewRat(100, 1)}, nil)

	var subsCtx context.Context
	watcherMock.On("Subscribe", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		subsCtx = args.Get(0).(context.Context)
	}).Once()

	price, err := NewAutoConvertedPrice("USD", big.NewRat(50, 1), nil)
	require.NoError(err)
	require.NotNil(t, price.cancelSubscription)

	price.Stop()
	require.Nil(price.cancelSubscription)
	require.Error(subsCtx.Err())
}

func TestCurrencyToWeiMultiplier(t *testing.T) {
	tests := []struct {
		name         string
		data         eth.PriceData
		baseCurrency string
		expectedWei  *big.Rat
	}{
		{
			name:         "Base currency is ETH",
			data:         eth.PriceData{Price: big.NewRat(500, 1)}, // 500 USD per ETH
			baseCurrency: "ETH",
			expectedWei:  big.NewRat(1e18, 500), // (1 / 500 USD/ETH) * 1e18 wei/ETH
		},
		{
			name:         "Base currency is not ETH",
			data:         eth.PriceData{Price: big.NewRat(1, 2000)}, // 1/2000 ETH per USD
			baseCurrency: "USD",
			expectedWei:  big.NewRat(5e14, 1), // (1 * 1/2000 ETH/USD) * 1e18 wei/ETH
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := currencyToWeiMultiplier(tt.data, tt.baseCurrency)
			assert.Equal(t, 0, tt.expectedWei.Cmp(result))
		})
	}
}

// Auto-generated code from here down.
//
// Code generated by mockery v2.42.1. DO NOT EDIT.

// PriceFeedWatcherMock is an autogenerated mock type for the PriceFeedWatcher type
type PriceFeedWatcherMock struct {
	mock.Mock
}

// Currencies provides a mock function with given fields:
func (_m *PriceFeedWatcherMock) Currencies() (string, string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Currencies")
	}

	var r0 string
	var r1 string
	var r2 error
	if rf, ok := ret.Get(0).(func() (string, string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() string); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(string)
	}

	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Current provides a mock function with given fields:
func (_m *PriceFeedWatcherMock) Current() (eth.PriceData, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Current")
	}

	var r0 eth.PriceData
	var r1 error
	if rf, ok := ret.Get(0).(func() (eth.PriceData, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() eth.PriceData); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(eth.PriceData)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Subscribe provides a mock function with given fields: ctx, sink
func (_m *PriceFeedWatcherMock) Subscribe(ctx context.Context, sink chan<- eth.PriceData) {
	_m.Called(ctx, sink)
}

// NewPriceFeedWatcherMock creates a new instance of PriceFeedWatcherMock. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPriceFeedWatcherMock(t interface {
	mock.TestingT
	Cleanup(func())
}) *PriceFeedWatcherMock {
	mock := &PriceFeedWatcherMock{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
