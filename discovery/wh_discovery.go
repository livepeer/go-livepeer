package discovery

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
)

type webhookResponse struct {
	Address string  `json:"address,omitempty"`
	Score   float32 `json:"score,omitempty"`
}

type webhookPool struct {
	pool             *orchestratorPool
	callback         *url.URL
	responseHash     ethcommon.Hash
	lastRequest      time.Time
	mu               *sync.RWMutex
	bcast            common.Broadcaster
	discoveryTimeout time.Duration
}

func NewWebhookPool(bcast common.Broadcaster, callback *url.URL, discoveryTimeout time.Duration) *webhookPool {
	p := &webhookPool{
		callback:         callback,
		mu:               &sync.RWMutex{},
		bcast:            bcast,
		discoveryTimeout: discoveryTimeout,
	}
	go p.getInfos()
	return p
}

func (w *webhookPool) getInfos() ([]common.OrchestratorLocalInfo, error) {
	w.mu.RLock()
	lastReq := w.lastRequest
	pool := w.pool
	w.mu.RUnlock()

	// retrive addrs from cache if time since lastRequest is less than the refresh interval
	if time.Since(lastReq) < common.WebhookDiscoveryRefreshInterval {
		return pool.GetInfos(), nil
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
		pool = w.pool // may have been reset since beginning
		w.mu.Unlock()
		return pool.GetInfos(), nil
	}

	infos, err := deserializeWebhookJSON(body)
	if err != nil {
		return nil, err
	}

	// pool = NewOrchestratorPool(w.bcast, addrs)
	pool = &orchestratorPool{infos: infos, bcast: w.bcast, discoveryTimeout: w.discoveryTimeout}

	w.mu.Lock()
	w.responseHash = hash
	w.pool = pool
	w.lastRequest = time.Now()
	w.mu.Unlock()

	return infos, nil
}

func (w *webhookPool) GetInfos() []common.OrchestratorLocalInfo {
	infos, _ := w.getInfos()
	return infos
}

func (w *webhookPool) Size() int {
	return len(w.GetInfos())
}

func (w *webhookPool) SizeWith(scorePred common.ScorePred) int {
	// Refresh pool
	w.GetInfos()

	var size int
	w.mu.RLock()
	if w.pool != nil {
		size = w.pool.SizeWith(scorePred)
	}
	w.mu.RUnlock()
	return size
}

func (w *webhookPool) GetOrchestrators(ctx context.Context, numOrchestrators int, suspender common.Suspender, caps common.CapabilityComparator,
	scorePred common.ScorePred) (common.OrchestratorDescriptors, error) {

	_, err := w.getInfos()
	if err != nil {
		return nil, err
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.pool.GetOrchestrators(ctx, numOrchestrators, suspender, caps, scorePred)
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

func deserializeWebhookJSON(body []byte) ([]common.OrchestratorLocalInfo, error) {
	var addrs []webhookResponse
	if err := json.Unmarshal(body, &addrs); err != nil {
		glog.Error("Unable to unmarshal JSON ", err)
		return nil, err
	}
	var infos []common.OrchestratorLocalInfo
	for _, addr := range addrs {
		if addr.Address == "" {
			continue
		}
		uri, err := url.ParseRequestURI(addr.Address)
		if err != nil {
			glog.Errorf("Unable to parse address  %q : %s", addr.Address, err)
			continue
		}
		infos = append(infos, common.OrchestratorLocalInfo{URL: uri, Score: addr.Score})
	}

	return infos, nil
}
