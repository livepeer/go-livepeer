package eth

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/golang/glog"
)

// GasPriceOracle defines methods for fetching a suggested gas price
// for submitting transactions
type GasPriceOracle interface {
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// GasPriceMonitor polls for gas price updates and updates its
// own view of the current gas price that can be used by others
type GasPriceMonitor struct {
	gpo             GasPriceOracle
	polling         bool
	pollingInterval time.Duration
	cancel          context.CancelFunc

	// mu is a mutex that protects access to gasPrice
	mu sync.Mutex
	// gasPrice is the current gas price to be returned
	// to users
	gasPrice *big.Int
}

// NewGasPriceMonitor returns a GasPriceMonitor
func NewGasPriceMonitor(gpo GasPriceOracle, pollingInterval time.Duration) *GasPriceMonitor {
	return &GasPriceMonitor{
		gpo:             gpo,
		pollingInterval: pollingInterval,
		gasPrice:        big.NewInt(0),
	}
}

// GasPrice returns the current gas price
func (gpm *GasPriceMonitor) GasPrice() *big.Int {
	gpm.mu.Lock()
	defer gpm.mu.Unlock()

	return gpm.gasPrice
}

// Start starts polling for gas price updates
func (gpm *GasPriceMonitor) Start(ctx context.Context) error {
	if gpm.polling {
		return errors.New("already polling")
	}

	// Initialize gasPrice before starting to poll
	if err := gpm.fetchAndUpdateGasPrice(ctx); err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	gpm.cancel = cancel

	ticker := time.NewTicker(gpm.pollingInterval)

	go func(ctx context.Context) {
		for {
			select {
			case <-ticker.C:
				if err := gpm.fetchAndUpdateGasPrice(ctx); err != nil {
					glog.Errorf("error getting gas price: %v", err)
				}
			case <-ctx.Done():
				gpm.cancel = nil
				gpm.polling = false
				return
			}
		}
	}(cctx)

	gpm.polling = true

	return nil
}

// Stop stops polling for gas price updates
func (gpm *GasPriceMonitor) Stop() error {
	if !gpm.polling {
		return errors.New("not polling")
	}

	gpm.cancel()
	gpm.cancel = nil
	gpm.polling = false

	return nil
}

func (gpm *GasPriceMonitor) fetchAndUpdateGasPrice(ctx context.Context) error {
	gasPrice, err := gpm.gpo.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	gpm.updateGasPrice(gasPrice)

	return nil
}

func (gpm *GasPriceMonitor) updateGasPrice(gasPrice *big.Int) {
	gpm.mu.Lock()
	defer gpm.mu.Unlock()

	gpm.gasPrice = gasPrice
}
