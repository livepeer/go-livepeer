package discovery

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

var whRefreshInterval = 1 * time.Minute

type webhookResponse struct {
	Address string
}

type webhookPool struct {
	pool        *orchestratorPool
	callback    *url.URL
	lastRequest time.Time
	*sync.RWMutex
	bcast common.Broadcaster
}

func NewWebhookPool(bcast common.Broadcaster, callback *url.URL) *webhookPool {
	p := &webhookPool{
		callback: callback,
		bcast:    bcast,
		RWMutex:  &sync.RWMutex{},
	}
	p.getURLs()
	return p
}

func (w *webhookPool) getURLs() ([]*url.URL, error) {
	w.RLock()
	lastReq := w.lastRequest
	pool := w.pool
	w.RUnlock()

	var addrs []*url.URL
	// retrive addrs from cache if time since lastRequest is less than the refresh interval
	if time.Since(lastReq) < whRefreshInterval {
		addrs = pool.GetURLs()
	} else {
		// retrive addrs from webhook if time since lastRequest is more than the refresh interval
		body, err := getURLsfromWebhook(w.callback)
		if err != nil {
			return nil, err
		}

		addrs, err = deserializeWebhookJSON(body)
		if err != nil {
			return nil, err
		}

		w.Lock()
		w.pool = NewOrchestratorPool(w.bcast, addrs)
		w.lastRequest = time.Now()
		w.Unlock()
	}

	return addrs, nil
}

func (w *webhookPool) GetURLs() []*url.URL {
	uris, err := w.getURLs()
	if err != nil {
		glog.Error(err)
		return nil
	}
	for _, uri := range uris {
		glog.Infof("using webhook pool URI: %s", uri.String())
	}
	return uris
}

func (w *webhookPool) Size() int {
	return len(w.GetURLs())
}

func (w *webhookPool) GetOrchestrators(numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator) ([]*net.OrchestratorInfo, error) {
	_, err := w.getURLs()
	if err != nil {
		return nil, err
	}

	w.RLock()
	defer w.RUnlock()

	return w.pool.GetOrchestrators(numOrchestrators, suspender, caps)
}

var getURLsfromWebhook = func(cbUrl *url.URL) ([]byte, error) {
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

	return body, nil
}

func deserializeWebhookJSON(body []byte) ([]*url.URL, error) {
	var addrs []webhookResponse
	if err := json.Unmarshal(body, &addrs); err != nil {
		glog.Error("Unable to unmarshal JSON ", err)
		return nil, err
	}
	var urls []*url.URL
	for _, addr := range addrs {
		uri, err := url.ParseRequestURI(addr.Address)
		if err != nil {
			glog.Errorf("Unable to parse address  %s : %s", addr, err)
			continue
		}
		urls = append(urls, uri)
	}

	return urls, nil
}
