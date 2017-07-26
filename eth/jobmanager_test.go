package eth

import (
	"math/big"
	"testing"
)

func TestSubscribeStrmID(t *testing.T) {

	//Create Job Manager
	jidChan := make(chan *big.Int)
	defer close(jidChan)
	stub := &StubClient{}
	j1 := NewJobManager(stub)
	// if err := j1.Start(); err != nil {
	// 	t.Errorf("Error starting job manager: %v", err)
	// }

	//Subscribe by streamID
	sub, err := j1.SubscribeJobIDByStrmID("strmID", jidChan)
	if err != nil {
		t.Errorf("Error subscribing to JobID: %v", err)
	}
	if len(j1.strmIDSubs) != 1 {
		t.Errorf("Expecting the subscription length to be 1, got %v:", len(j1.strmIDSubs))
	}
	if len(j1.strmIDSubs["strmID"]) != 1 {
		t.Errorf("Expecting the subscription length to be 1, got %v:", len(j1.strmIDSubs))
	}
	if j1.strmIDSubs["strmID"][jidChan] != sub {
		t.Errorf("Expecting to find sub, but got %v", j1.strmIDSubs["strmID"][jidChan])
	}

	// //Send Event
	// stub.SubLogsCh <- types.Log{}

	if err := sub.Unsubscribe(); err != nil {
		t.Errorf("Error unsubscribing: %v", err)
	}

	if len(j1.strmIDSubs["strmID"]) != 0 {
		t.Errorf("Expecting the subscription length to be 0, got %v", len(j1.strmIDSubs))
	}

	//Test for the error case
	jidChan2 := make(chan *big.Int)
	defer close(jidChan2)
	j2 := JobManager{strmIDSubs: map[string]map[chan *big.Int]*JobSubscription{"strmID": map[chan *big.Int]*JobSubscription{jidChan2: &JobSubscription{}}}}
	_, err = j2.SubscribeJobIDByStrmID("strmID", jidChan2)
	if err == nil {
		t.Errorf("Expecting error because subscription already exists.  Instead got nil")
	}

}
