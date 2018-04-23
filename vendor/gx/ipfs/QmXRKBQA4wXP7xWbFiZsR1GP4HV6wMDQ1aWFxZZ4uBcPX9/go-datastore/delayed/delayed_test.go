package delayed

import (
	"testing"
	"time"

	delay "gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	datastore "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
)

func TestDelayed(t *testing.T) {
	d := New(datastore.NewMapDatastore(), delay.Fixed(time.Second))
	now := time.Now()
	k := datastore.NewKey("test")
	err := d.Put(k, "value")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(now) < 2*time.Second {
		t.Fatal("There should have been a delay of 1 second in put and in get")
	}
}
