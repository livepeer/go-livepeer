package eventservices

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
)

func TestProcessHistoricalJobs(t *testing.T) {
	seth := &eth.StubClient{}
	seth.WatchJobError = fmt.Errorf("Historical job error")
	n, err := core.NewLivepeerNode(seth, ".", nil)
	if err != nil {
		t.Error(err)
	}
	js := NewJobService(n)

	//Test that we exit early because startBlock is 0 in this case
	err = js.processHistoricalEvents(context.Background(), big.NewInt(0))
	if err != nil {
		t.Error("Unexpected error ", err)
	}

	//Set last seen block to be 10, this should result in an error because we don't exit early anymore and we set a stubbed error
	err = js.processHistoricalEvents(context.Background(), big.NewInt(10))
	if err != seth.WatchJobError {
		t.Error("Expected WatchJobError, got ", err)
	}
}
