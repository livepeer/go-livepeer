package discovery

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"

	"github.com/golang/glog"
)

var whRefreshInterval = 1 * time.Minute

type webhookResponse struct {
	Address string
}

type webhookPool struct {
	node         *core.LivepeerNode
	pool         *orchestratorPool
	callback     *url.URL
	responseHash ethcommon.Hash
	lastRequest  time.Time
	mu           *sync.RWMutex
}

func NewWebhookPool(node *core.LivepeerNode, callback *url.URL) *webhookPool {
	p := &webhookPool{
		node:     node,
		callback: callback,
		mu:       &sync.RWMutex{},
	}
	go p.getURLs()
	return p
}

func (w *webhookPool) getURLs() ([]*url.URL, error) {
	w.mu.RLock()
	lastReq := w.lastRequest
	w.mu.RUnlock()

	// retrive addrs from cache if time since lastRequest is less than the refresh interval
	if time.Since(lastReq) < whRefreshInterval {
		return w.pool.GetURLs(), nil
	}

	// retrive addrs from webhook if time since lastRequest is more than the refresh interval
	body, err := getURLsfromWebhook(w.callback)
	if err != nil {
		return nil, err
	}

	hash := ethcommon.BytesToHash(crypto.Keccak256(body))
	if hash == w.responseHash {
		w.mu.Lock()
		w.lastRequest = time.Now()
		w.mu.Unlock()
		return w.pool.GetURLs(), nil
	}

	addrs, err := deserializeWebhookJSON(body)
	if err != nil {
		return nil, err
	}

	pool := NewOrchestratorPool(w.node, addrs)

	w.mu.Lock()
	w.responseHash = hash
	w.pool = pool
	w.lastRequest = time.Now()
	w.mu.Unlock()

	return addrs, nil
}

func (w *webhookPool) GetURLs() []*url.URL {
	uris, _ := w.getURLs()
	return uris
}

func (w *webhookPool) Size() int {
	return len(w.GetURLs())
}

func (w *webhookPool) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	_, err := w.getURLs()
	if err != nil {
		return nil, err
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.pool.GetOrchestrators(numOrchestrators)
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
