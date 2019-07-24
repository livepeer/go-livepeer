package discovery

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

type webhookResponse struct {
	Address string
}

type webhookPool struct {
	node     *core.LivepeerNode
	callback *url.URL
}

func NewWebhookPool(node *core.LivepeerNode, callback *url.URL) *webhookPool {
	return &webhookPool{node: node, callback: callback}
}

func getURLs(cbUrl *url.URL) ([]*url.URL, error) {
	// TODO cache results for some time to avoid repeated callouts
	var httpc = &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := httpc.Get(cbUrl.String())
	if err != nil {
		glog.Error("Unable to make webhook request ", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error("Unable to read response body ", err)
		return nil, err
	}
	var addrs []webhookResponse
	if err := json.Unmarshal(body, &addrs); err != nil {
		glog.Error("Unable to unmarshal JSON ", err)
		return nil, err
	}
	var urls []*url.URL
	for i := range addrs {
		uri, err := url.ParseRequestURI(addrs[i].Address)
		if err != nil {
			glog.Errorf("Unable to parse address  %s : %s", addrs[i].Address, err)
			continue
		}
		urls = append(urls, uri)
	}
	return urls, nil
}

func (w *webhookPool) GetURLs() []*url.URL {
	uris, _ := getURLs(w.callback)
	return uris
}

func (w *webhookPool) Size() int {
	return len(w.GetURLs())
}

func (w *webhookPool) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	addrs, err := getURLs(w.callback)
	if err != nil {
		return nil, err
	}
	pool := NewOrchestratorPool(w.node, addrs)
	if pool == nil {
		return nil, errors.New("Nil orchestrator pool")
	}
	return pool.GetOrchestrators(numOrchestrators)
}
