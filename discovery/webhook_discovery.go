package discovery

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/server"

	"github.com/golang/glog"
)

var webhookCacheRefreshInterval = 1 * time.Minute
var getWebhookTicker = func() *time.Ticker {
	return time.NewTicker(webhookCacheRefreshInterval)
}

type WHOrchestratorPoolCache struct {
	node       *core.LivepeerNode
	webhookURL string
}

type orchAddress struct {
	address string `json:address`
}

func NewWebhookOrchestratorPoolCache(node *core.LivepeerNode, orchWebhookUrl string) *WHOrchestratorPoolCache {
	_ = cacheWebhookTranscoders(node, orchWebhookUrl)

	ticker := getWebhookTicker()
	go func(node *core.LivepeerNode) {
		for _ = range ticker.C {
			err := cacheWebhookTranscoders(node, orchWebhookUrl)
			if err != nil {
				continue
			}
		}
	}(node)

	return &WHOrchestratorPoolCache{node: node, webhookURL: orchWebhookUrl}
}

func getOrchAddresses(orchWebhookUrl string) ([]string, error) {
	// Get data from the Orchestrator Webhook URL
	resp, err := http.Get(orchWebhookUrl)
	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("Http.Get error: %v ", err)
	}
	defer resp.Body.Close()

	// Read json info on available orchestrators
	orchAddrBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("File ReadAll error: %v", err)
	}

	var orchAddresses []orchAddress
	err = json.Unmarshal(orchAddrBytes, &orchAddresses)
	if err != nil {
		return nil, fmt.Errorf("Json unmarshal error: %v", err)
	}

	var stringAddr []string
	for _, addr := range orchAddresses {
		stringAddr = append(stringAddr, addr.address)
	}

	return stringAddr, err
}

func (who *WHOrchestratorPoolCache) GetOrchestrators(numOrchestrators int) ([]*net.OrchestratorInfo, error) {
	orchs, err := who.node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	if err != nil || len(orchs) <= 0 {
		return nil, err
	}

	var uris []string
	for _, orch := range orchs {
		uri := orch.ServiceURI
		uris = append(uris, uri)
	}

	orchPool := NewOrchestratorPool(who.node, uris)

	orchInfos, err := orchPool.GetOrchestrators(numOrchestrators)
	if err != nil || len(orchInfos) <= 0 {
		return nil, err
	}

	return orchInfos, nil
}

func cacheWebhookTranscoders(node *core.LivepeerNode, webhookURL string) error {
	orchAddresses, err := getOrchAddresses(webhookURL)
	if err != nil {
		glog.Error("Could not refresh webhook list of orchestrators: ", err)
		return err
	}

	for _, addr := range orchAddresses {
		DBOrch := common.NewDBOrch(addr, addr)
		if err := node.Database.UpdateOrch(DBOrch); err != nil {
			glog.Error("Error updating Orchestrator in webhook: ", err)
		}
	}

	return nil
}

func (who *WHOrchestratorPoolCache) GetURLs() []*url.URL {
	orchs, err := who.node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	if err != nil || len(orchs) <= 0 {
		return nil
	}

	var uris []*url.URL
	for _, orch := range orchs {
		if uri, err := url.Parse(orch.ServiceURI); err == nil {
			uris = append(uris, uri)
		}
	}
	return uris
}

func (who *WHOrchestratorPoolCache) Size() int {
	orchs, err := who.node.Database.SelectOrchs(&common.DBOrchFilter{MaxPrice: server.BroadcastCfg.MaxPrice()})
	if err != nil {
		return 0
	}
	return len(orchs)
}
