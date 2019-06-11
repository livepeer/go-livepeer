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
	err := gpm.Start(context.Background())
	assert.EqualError(err, expErr.Error())

	// Switch back to no errors for SuggestGasPrice
	gpo.SetErr(nil)

	// Test success

	err = gpm.Start(context.Background())
	assert.Nil(err)
	defer gpm.Stop()

	assert.Equal(gasPrice, gpm.GasPrice())

	// Test error when already polling

	err = gpm.Start(context.Background())
	assert.EqualError(err, "already polling")
}

func TestStart_Polling(t *testing.T) {
	gasPrice1 := big.NewInt(777)
	gasPrice2 := big.NewInt(555)
	gasPrice3 := big.NewInt(888)
	gpo := newStubGasPriceOracle(gasPrice1)

	pollingInterval := 1 * time.Second
	gpm := NewGasPriceMonitor(gpo, pollingInterval)

	assert := assert.New(t)

	err := gpm.Start(context.Background())
	require.Nil(t, err)
	defer gpm.Stop()

	// Async update gas price so when the
	// sync sleep finishes, the monitor
	// should have decreased its own gas price
	go func() {
		time.Sleep(1 * pollingInterval)
		gpo.SetGasPrice(gasPrice2)
	}()

	time.Sleep(2 * pollingInterval)

	assert.Greater(gpo.queries, 0)
	assert.Equal(gasPrice2, gpm.GasPrice())

	queries := gpo.queries

	// Async update gas price so when the
	// sync sleep finishes, the monitor
	// should have increased its own gas price
	go func() {
		time.Sleep(1 * pollingInterval)
		gpo.SetGasPrice(gasPrice3)
	}()

	time.Sleep(2 * pollingInterval)

	// There should be more queries now
	assert.Greater(gpo.queries, queries)
	assert.Equal(gasPrice3, gpm.GasPrice())
}

func TestStart_Polling_ContextCancel(t *testing.T) {
	gasPrice1 := big.NewInt(777)
	gpo := newStubGasPriceOracle(gasPrice1)

	pollingInterval := 1 * time.Second
	gpm := NewGasPriceMonitor(gpo, pollingInterval)

	ctx, cancel := context.WithCancel(context.Background())
	err := gpm.Start(ctx)
	require.Nil(t, err)

	queries := gpo.queries

	// Cancel polling loop
	cancel()

	time.Sleep(1 * time.Second)

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

	err = gpm.Start(context.Background())
	require.Nil(t, err)

	queries := gpo.queries

	err = gpm.Stop()
	assert.Nil(err)

	time.Sleep(1 * time.Second)

	// Ensure there are no more queries
	assert.Equal(queries, gpo.queries)
}
