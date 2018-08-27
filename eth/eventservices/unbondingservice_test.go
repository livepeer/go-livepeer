package eventservices

import (
	"errors"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
)

func TestProcessHistoricalEvents(t *testing.T) {
	dbh, dbraw, err := common.TempDB(t)
	if err != nil {
		return
	}
	defer dbh.Close()
	defer dbraw.Close()

	c := &eth.StubClient{}
	//Set the eth client to be nil
	s := NewUnbondingService(c, dbh)

	//Test that we exit early because LastSeenBlock is 0 in this case
	dbh.SetLastSeenBlock(big.NewInt(0))
	if err := s.processHistoricalEvents(); err != nil {
		t.Errorf("Error: %v", err)
	}

	//Set last seen block to be 10, this should result in an error because we don't exit early anymore and we set a stubbed error
	dbh.SetLastSeenBlock(big.NewInt(10))
	c.ProcessHistoricalUnbondError = errors.New("StubError")
	if err := s.processHistoricalEvents(); err.Error() != "StubError" {
		t.Errorf("Expecting stub error, but got none")
	}
}
