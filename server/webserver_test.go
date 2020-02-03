package server

import (
	"math/big"
	"strings"
	"testing"
	"github.com/livepeer/go-livepeer/core"
	"github.com/stretchr/testify/assert"
)

func TestSetOrchestratorPriceInfo(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "", nil)
	s := &LivepeerServer{
		LivepeerNode: n,
	}

	// pricePerUnit is not an integer
	err := s.setOrchestratorPriceInfo("nil", "1")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Error converting pricePerUnit string to int64"))

	// pixelsPerUnit is not an integer
	err = s.setOrchestratorPriceInfo("1", "nil")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Error converting pixelsPerUnit string to int64"))

	err = s.setOrchestratorPriceInfo("1", "1")
	assert.Nil(t, err)
	assert.Zero(t, s.LivepeerNode.GetBasePrice().Cmp(big.NewRat(1, 1)))

	//Price per unit <= 0
	err = s.setOrchestratorPriceInfo("0", "1")
	assert.EqualErrorf(t, err, err.Error(), "price unit must be greater than 0, provided %d\n", 0)
	err = s.setOrchestratorPriceInfo("-5", "1")
	assert.EqualErrorf(t, err, err.Error(), "price unit must be greater than 0, provided %d\n", -5)

	// pixels per unit <= 0
	err = s.setOrchestratorPriceInfo("1", "0")
	assert.EqualErrorf(t, err, err.Error(), "pixels per unit must be greater than 0, provided %d\n", 0)
	err = s.setOrchestratorPriceInfo("1", "-5")
	assert.EqualErrorf(t, err, err.Error(), "pixels per unit must be greater than 0, provided %d\n", -5)
}
