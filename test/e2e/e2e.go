package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/cmd/devtool/devtool"
	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// Start Geth Docker container helpers
type gethContainer struct {
	testcontainers.Container
	URI          string
	webServerURI string
}

func setupGeth(t *testing.T) *gethContainer {
	ctx := context.TODO()
	req := testcontainers.ContainerRequest{
		Image:        "livepeer/geth-with-livepeer-protocol:confluence",
		ExposedPorts: []string{"8546/tcp", "8545/tcp"},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	ip, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "8545")
	require.NoError(t, err)

	mappedPortWebServer, err := container.MappedPort(ctx, "8546")
	require.NoError(t, err)

	uri := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	webServerUri := fmt.Sprintf("http://%s:%s", ip, mappedPortWebServer.Port())

	return &gethContainer{Container: container, URI: uri, webServerURI: webServerUri}
}

func terminateGeth(t *testing.T, geth *gethContainer) {
	err := geth.Terminate(context.TODO())
	require.NoError(t, err)
}

// Start Livepeer helpers
var (
	httpPort = 8935
	cliPort  = 7935
	rtmpPort = 1935
	mu       sync.Mutex
)

type livepeer struct {
	dev   *devtool.Devtool
	cfg   *starter.LivepeerConfig
	ready chan struct{}
}

type OrchestratorConfig struct {
	PricePerUnit   int64
	PixelsPerUnit  int64
	BlockRewardCut float64
	FeeShare       float64
	LptStake       int64
	ServiceURI     string
}

func lpCfg() starter.LivepeerConfig {
	mu.Lock()
	serviceAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	httpPort++
	cliAddr := fmt.Sprintf("127.0.0.1:%d", cliPort)
	cliPort++
	rtmpAddr := fmt.Sprintf("127.0.0.1:%d", rtmpPort)
	rtmpPort++
	mu.Unlock()

	ethPassword := ""
	network := "devnet"
	blockPollingInterval := 1
	pricePerUnit := 1
	initializeRound := true

	cfg := starter.DefaultLivepeerConfig()
	cfg.ServiceAddr = &serviceAddr
	cfg.HttpAddr = &serviceAddr
	cfg.CliAddr = &cliAddr
	cfg.RtmpAddr = &rtmpAddr
	cfg.EthPassword = &ethPassword
	cfg.Network = &network
	cfg.BlockPollingInterval = &blockPollingInterval
	cfg.PricePerUnit = &pricePerUnit
	cfg.InitializeRound = &initializeRound
	return cfg
}

func startLivepeer(t *testing.T, lpCfg starter.LivepeerConfig, geth *gethContainer, ctx context.Context) *livepeer {
	datadir := t.TempDir()
	keystoreDir := filepath.Join(datadir, "keystore")
	acc := devtool.CreateKey(keystoreDir)
	devCfg := devtool.NewDevtoolConfig()
	devCfg.Endpoint = geth.URI
	devCfg.Account = acc
	devCfg.KeystoreDir = keystoreDir

	dev, err := devtool.Init(devCfg)
	require.NoError(t, err)

	err = dev.RequestTokens()
	require.NoError(t, err)

	err = dev.InitializeRound()
	require.NoError(t, err)

	lpCfg.EthUrl = &geth.URI
	lpCfg.Datadir = &datadir
	lpCfg.EthController = &dev.EthController
	lpCfg.EthAcctAddr = &devCfg.Account

	go func() {
		starter.StartLivepeer(ctx, lpCfg)
	}()

	ready := make(chan struct{})
	go func() {
		statusEndpoint := fmt.Sprintf("http://%s/status", *lpCfg.CliAddr)
		var statusCode int
		for statusCode != 200 {
			time.Sleep(200 * time.Millisecond)
			resp, err := http.Get(statusEndpoint)
			if err == nil {
				statusCode = resp.StatusCode
			}
		}
		ready <- struct{}{}
	}()

	return &livepeer{dev: &dev, cfg: &lpCfg, ready: ready}
}

func requireOrchestratorRegisteredAndActivated(t *testing.T, lpEth eth.LivepeerEthClient, params *OrchestratorConfig) {
	require := require.New(t)

	transPool, err := lpEth.TranscoderPool()

	require.NoError(err)
	require.Len(transPool, 1)
	trans := transPool[0]
	require.True(trans.Active)
	require.Equal("Registered", trans.Status)
	require.Equal(big.NewInt(params.LptStake), trans.DelegatedStake)
	require.Equal(eth.FromPerc(params.FeeShare), trans.FeeShare)
	require.Equal(eth.FromPerc(params.BlockRewardCut), trans.RewardCut)
}

func startOrchestrator(t *testing.T, geth *gethContainer, ctx context.Context) *livepeer {
	lpCfg := lpCfg()
	lpCfg.Orchestrator = boolPointer(true)
	lpCfg.Transcoder = boolPointer(true)
	return startLivepeer(t, lpCfg, geth, ctx)
}

func registerOrchestrator(o *livepeer, params *OrchestratorConfig) {
	val := url.Values{
		"pricePerUnit":   {fmt.Sprintf("%d", params.PricePerUnit)},
		"pixelsPerUnit":  {fmt.Sprintf("%d", params.PixelsPerUnit)},
		"blockRewardCut": {fmt.Sprintf("%v", params.BlockRewardCut)},
		"feeShare":       {fmt.Sprintf("%v", params.FeeShare)},
		"serviceURI":     {fmt.Sprintf("http://%v", o.cfg.HttpAddr)},
		"amount":         {fmt.Sprintf("%d", params.LptStake)},
	}

	for {
		if _, ok := httpPostWithParams(fmt.Sprintf("http://%s/activateOrchestrator", *o.cfg.CliAddr), val); ok {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (l *livepeer) stop() {
	l.dev.Close()
}

// Other helpers
func waitForNextRound(t *testing.T, lpEth eth.LivepeerEthClient) {
	r, err := lpEth.CurrentRound()
	require.NoError(t, err)

	for {
		nr, err := lpEth.CurrentRound()
		require.NoError(t, err)

		if nr.Cmp(r) > 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func boolPointer(b bool) *bool {
	return &b
}

func httpPostWithParams(url string, val url.Values) (string, bool) {
	return httpPostWithParamsHeaders(url, val, map[string]string{})
}

func httpPostWithParamsHeaders(url string, val url.Values, headers map[string]string) (string, bool) {
	var body *bytes.Buffer
	if val != nil {
		body = bytes.NewBufferString(val.Encode())
	} else {
		body = bytes.NewBufferString("")
	}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", false
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}

	return string(result), resp.StatusCode >= 200 && resp.StatusCode < 300
}
