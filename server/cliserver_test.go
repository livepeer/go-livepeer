package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer() *httptest.Server {
	n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
	n.NodeType = core.TranscoderNode
	n.TranscoderManager = core.NewRemoteTranscoderManager()
	strm := &common.StubServerStream{}
	go func() { n.TranscoderManager.Manage(strm, 5) }()
	time.Sleep(1 * time.Millisecond)
	n.Transcoder = n.TranscoderManager
	s := NewLivepeerServer("127.0.0.1:1938", n, true)
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	return srv
}

func TestGetStatus(t *testing.T) {
	srv := newMockServer()
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/status", srv.URL))
	assert := assert.New(t)
	req := require.New(t)
	req.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	req.Nil(err)
	expected := fmt.Sprintf(`{"Manifests":{},"OrchestratorPool":[],"Version":"undefined","GolangRuntimeVersion":"%s","GOArch":"%s","GOOS":"%s","RegisteredTranscodersNumber":1,"RegisteredTranscoders":[{"Address":"TestAddress","Capacity":5}],"LocalTranscoding":false}`,
		runtime.Version(), runtime.GOARCH, runtime.GOOS)
	assert.Equal(expected, string(body))
}

func TestGetEthChainID(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	// test offchain
	srv := newMockServer()
	res, err := http.Get(fmt.Sprintf("%s/EthChainID", srv.URL))
	require.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	require.Nil(err)
	assert.Equal("0", string(body))
	defer res.Body.Close()
	defer srv.Close()

	// test onchain
	dbh, dbraw, err := common.TempDB(t)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)
	err = dbh.SetChainID(big.NewInt(1))
	require.Nil(err)
	n, _ := core.NewLivepeerNode(&eth.StubClient{}, "./tmp", dbh)
	s := NewLivepeerServer("127.0.0.1:1938", n, true)
	mux := s.cliWebServerHandlers("addr")
	srv = httptest.NewServer(mux)
	defer srv.Close()
	res, err = http.Get(fmt.Sprintf("%s/EthChainID", srv.URL))
	require.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	require.Nil(err)
	assert.Equal("1", string(body))
}

func TestGetContractAddresses(t *testing.T) {
	srv := newMockServer()
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/contractAddresses", srv.URL))
	assert := assert.New(t)
	req := require.New(t)
	req.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	req.Nil(err)
	assert.Equal("{}", string(body))
}

func TestGetDelegatorInfo(t *testing.T) {
	srv := newMockServer()
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/delegatorInfo", srv.URL))
	assert := assert.New(t)
	req := require.New(t)
	req.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	req.Nil(err)
	assert.Equal("{}", string(body))
}

func TestRegisteredOrchestrators(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	dbh, dbraw, err := common.TempDB(t)
	require.Nil(err)
	defer dbh.Close()
	defer dbraw.Close()

	addr := pm.RandAddress()

	addr2 := pm.RandAddress()

	orch := &common.DBOrch{
		ServiceURI:        "foo",
		EthereumAddr:      addr.Hex(),
		PricePerPixel:     5000,
		ActivationRound:   100,
		DeactivationRound: 1000,
		Stake:             100000,
	}

	err = dbh.UpdateOrch(orch)
	require.Nil(err)

	eth := &eth.StubClient{
		Orchestrators: []*lpTypes.Transcoder{
			{
				Address:        addr,
				Active:         true,
				DelegatedStake: big.NewInt(100000),
				RewardCut:      big.NewInt(5),
				FeeShare:       big.NewInt(10),
				ServiceURI:     orch.ServiceURI,
			},
			{
				Address:        addr2,
				Active:         true,
				DelegatedStake: big.NewInt(100000),
				RewardCut:      big.NewInt(5),
				FeeShare:       big.NewInt(10),
				ServiceURI:     "foo.bar.baz:quux",
			},
		},
	}

	n, _ := core.NewLivepeerNode(eth, "./tmp", dbh)

	s := NewLivepeerServer("127.0.0.1:1938", n, true)
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := http.Get(fmt.Sprintf("%s/registeredOrchestrators", srv.URL))
	assert.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	require.Nil(err)
	var orchestrators []lpTypes.Transcoder
	err = json.Unmarshal(body, &orchestrators)
	require.Nil(err)
	expPrice, err := common.PriceToFixed(orchestrators[0].PricePerPixel)
	require.Nil(err)
	assert.Equal(expPrice, orch.PricePerPixel)
	assert.Equal(orchestrators[0].Address.Hex(), orch.EthereumAddr)
	expPrice, err = common.PriceToFixed(orchestrators[1].PricePerPixel)
	require.Nil(err)
	assert.Equal(expPrice, int64(0))
	assert.Equal(orchestrators[1].Address, addr2)

	eth.TranscoderPoolError = errors.New("error")
	res, err = http.Get(fmt.Sprintf("%s/registeredOrchestrators", srv.URL))
	assert.Nil(err)
	assert.Equal(http.StatusInternalServerError, res.StatusCode)
}
