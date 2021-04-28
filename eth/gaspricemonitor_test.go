package eth

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGasPriceOracle struct {
	mu       sync.Mutex
	gasPrice *big.Int
	queries  int
	err      error
}

func newStubGasPriceOracle(gasPrice *big.Int) *stubGasPriceOracle {
	return &stubGasPriceOracle{gasPrice: gasPrice}
}

func (s *stubGasPriceOracle) SetGasPrice(gasPrice *big.Int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gasPrice = gasPrice
}

func (s *stubGasPriceOracle) SetErr(err error) {
	s.err = err
}

func (s *stubGasPriceOracle) Queries() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.queries
}

func (s *stubGasPriceOracle) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return nil, s.err
	}

	s.queries++

	return s.gasPrice, nil
}

func TestStart(t *testing.T) {
	gasPrice := big.NewInt(777)
	gpo := newStubGasPriceOracle(gasPrice)

	gpm := NewGasPriceMonitor(gpo, 1*time.Hour)

	assert := assert.New(t)

	// Test error from first attempt to fetch gas price

	expErr := errors.New("SuggestGasPrice error")
	gpo.SetErr(expErr)
	update, err := gpm.Start(context.Background())
	assert.Nil(update)
	assert.EqualError(err, expErr.Error())

	// Switch back to no errors for SuggestGasPrice
	gpo.SetErr(nil)

	// Test success

	update, err = gpm.Start(context.Background())
	assert.NotNil(update)
	assert.Nil(err)
	defer gpm.Stop()

	assert.Equal(gasPrice, gpm.GasPrice())

	// Test error when already polling

	update, err = gpm.Start(context.Background())
	assert.Nil(update)
	assert.EqualError(err, "already polling")
}

func TestStart_Polling(t *testing.T) {
	gasPrice1 := big.NewInt(777)
	gasPrice2 := big.NewInt(555)
	gasPrice3 := big.NewInt(888)
	gpo := newStubGasPriceOracle(gasPrice1)

	pollingInterval := 1 * time.Millisecond
	gpm := NewGasPriceMonitor(gpo, pollingInterval)

	assert := assert.New(t)

	update, err := gpm.Start(context.Background())
	require.NotNil(t, update)
	require.Nil(t, err)
	defer gpm.Stop()

	// Async update gas price so when the
	// sync sleep finishes, the monitor
	// should have decreased its own gas price
	go func() {
		gpo.SetGasPrice(gasPrice2)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Greater(gpo.Queries(), 0)
	mean := new(big.Int).Div(new(big.Int).Add(gasPrice1, gasPrice2), big.NewInt(2))
	assert.Equal(mean, gpm.GasPrice())

	queries := gpo.Queries()

	// Async update gas price so when the
	// sync sleep finishes, the monitor
	// should have increased its own gas price
	go func() {
		gpo.SetGasPrice(gasPrice3)
	}()

	time.Sleep(100 * time.Millisecond)

	// There should be more queries now
	assert.Greater(gpo.Queries(), queries)
	sum := new(big.Int).Add(new(big.Int).Add(gasPrice1, gasPrice2), gasPrice3)
	mean = new(big.Int).Div(sum, big.NewInt(3))
	assert.Equal(mean, gpm.GasPrice())
}

func TestStart_Polling_ContextCancel(t *testing.T) {
	gasPrice1 := big.NewInt(777)
	gpo := newStubGasPriceOracle(gasPrice1)

	pollingInterval := 1 * time.Second
	gpm := NewGasPriceMonitor(gpo, pollingInterval)

	ctx, cancel := context.WithCancel(context.Background())
	update, err := gpm.Start(ctx)
	require.NotNil(t, update)
	require.Nil(t, err)

	queries := gpo.queries

	// Cancel polling loop
	cancel()

	time.Sleep(100 * time.Millisecond)

	// Ensure there are no more queries
	assert.Equal(t, gpo.queries, queries)
}

func TestStop(t *testing.T) {
	gasPrice := big.NewInt(777)
	gpo := newStubGasPriceOracle(gasPrice)
	gpo.SetGasPrice(gasPrice)

	gpm := NewGasPriceMonitor(gpo, 1*time.Hour)

	assert := assert.New(t)

	// Test error when not polling

	err := gpm.Stop()
	assert.EqualError(err, "not polling")

	// Test success

	update, err := gpm.Start(context.Background())
	require.NotNil(t, update)
	require.Nil(t, err)

	queries := gpo.queries

	err = gpm.Stop()
	assert.Nil(err)

	time.Sleep(100 * time.Millisecond)

	// Ensure there are no more queries
	assert.Equal(queries, gpo.queries)

	// check gasPriceUpdate channel is closed
	_, ok := (<-gpm.update)
	assert.False(ok)
}

func TestGasPrices(t *testing.T) {
	assert := assert.New(t)
	q := newGasPrices(gasPricesLength)

	// queue is empty
	assert.False(q.isFull())
	assert.Nil(q.front())
	assert.Nil(q.back())
	assert.Equal(q.mean(), big.NewInt(0))
	assert.False(q.isOutlier(big.NewInt(9999999)))

	// Push 1 element
	el_0 := big.NewInt(10)
	q.push(el_0)
	assert.False(q.isFull())
	assert.Equal(el_0, q.front())
	assert.Equal(el_0, q.back())
	assert.Equal(q.mean(), el_0)
	assert.False(q.isOutlier(big.NewInt(9999999)))

	// Push 2nd element
	el_1 := big.NewInt(20)
	q.push(el_1)
	assert.False(q.isFull())
	assert.Equal(el_0, q.front())
	assert.Equal(el_1, q.back())
	assert.Equal(q.mean(), new(big.Int).Div(new(big.Int).Add(el_0, el_1), big.NewInt(2)))
	assert.True(q.isOutlier(big.NewInt(9999999)))
	assert.False(q.isOutlier(big.NewInt(15)))

	// Push 3rd element
	el_2 := big.NewInt(30)
	q.push(el_2)
	assert.True(q.isFull())
	mean := new(big.Int).Div(
		big.NewInt(el_0.Int64()+el_1.Int64()+el_2.Int64()),
		big.NewInt(3),
	)
	assert.Equal(el_0, q.front())
	assert.Equal(el_2, q.back())
	assert.Equal(q.mean(), mean)
	assert.True(q.isOutlier(big.NewInt(9999999)))
	assert.False(q.isOutlier(big.NewInt(15)))

	// Push 4th element, current front is removed
	el_3 := big.NewInt(20)
	q.push(el_3)
	mean = new(big.Int).Div(
		big.NewInt(el_1.Int64()+el_2.Int64()+el_3.Int64()),
		big.NewInt(3),
	)
	assert.Equal(el_1, q.front())
	assert.Equal(el_3, q.back())
	assert.Equal(q.mean(), mean)
	assert.True(q.isOutlier(big.NewInt(9999999)))
	assert.False(q.isOutlier(big.NewInt(15)))
}
