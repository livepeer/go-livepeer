package monitor

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"
	"github.com/livepeer/streamingviz/data"
)

func TestMonitor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading r: %v", err)
		}

		node := &data.Node{}
		err = json.Unmarshal(rBody, node)
		if err != nil {
			t.Errorf("Error unmarshaling: %v", err)
		}

		glog.Infof("result: %v", node)
	}))
	defer ts.Close()

	m := Instance()
	m.LogNewConn("local1", "remote1")
	m.LogNewConn("local2", "remote2")
	m.RemoveConn("local1", "remote1")
	m.LogStream("sid", 10, 10)

	m.Node.SubmitToCollector(ts.URL)
}
