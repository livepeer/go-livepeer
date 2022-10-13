package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
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
	n, _ := core.NewLivepeerNode(&eth.StubClient{}, "./tmp", nil)
	n.NodeType = core.TranscoderNode
	n.TranscoderManager = core.NewRemoteTranscoderManager()
	strm := &common.StubServerStream{}
	go func() { n.TranscoderManager.Manage(strm, 5, nil) }()
	time.Sleep(1 * time.Millisecond)
	n.Transcoder = n.TranscoderManager
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	return srv
}

func TestActivateOrchestrator(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	eth := &eth.StubClient{
		Orch: &lpTypes.Transcoder{},
	}
	n, _ := core.NewLivepeerNode(eth, "./tmp", nil)
	n.NodeType = core.TranscoderNode
	n.TranscoderManager = core.NewRemoteTranscoderManager()
	strm := &common.StubServerStream{}
	go func() { n.TranscoderManager.Manage(strm, 5, nil) }()
	time.Sleep(1 * time.Millisecond)
	n.Transcoder = n.TranscoderManager
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var (
		blockRewardCut int    = 5
		feeShare       int    = 10
		pricePerUnit   int    = 1
		pixelsPerUnit  int    = 1
		serviceURI     string = "http://foo.bar:1337"
	)

	form := url.Values{
		"blockRewardCut": {fmt.Sprintf("%v", blockRewardCut)},
		"feeShare":       {fmt.Sprintf("%v", feeShare)},
		"pricePerUnit":   {fmt.Sprintf("%v", strconv.Itoa(pricePerUnit))},
		"pixelsPerUnit":  {fmt.Sprintf("%v", strconv.Itoa(pixelsPerUnit))},
		"serviceURI":     {fmt.Sprintf("%v", serviceURI)},
	}

	// Test GetTranscoderError
	eth.Err = errors.New("GetTranscoder error")
	req := bytes.NewBufferString(form.Encode())
	res, _ := http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusInternalServerError, res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), eth.Err.Error())
	eth.Err = nil

	// Test Transcoder Registered
	eth.Orch.Status = "Registered"
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "orchestrator already registered")
	eth.Orch.Status = ""

	// Test CurrentRoundLocked error
	eth.RoundLockedErr = errors.New("CurrentRoundLocked error")
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusInternalServerError, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), eth.RoundLockedErr.Error())
	eth.RoundLockedErr = nil

	// Test Round Locked
	eth.RoundLocked = true
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusInternalServerError, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "current round is locked")
	eth.RoundLocked = false

	// Test no block reward cut
	form["blockRewardCut"] = []string{""}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "missing form param: blockRewardCut")

	// Test invalid block reward cut
	form["blockRewardCut"] = []string{"foo"}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "strconv.ParseFloat: parsing \"foo\": invalid syntax")
	form["blockRewardCut"] = []string{"5"}

	// Test no feeshare
	form["feeShare"] = []string{""}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "missing form param: feeShare")

	// Test invalid feeshare
	form["feeShare"] = []string{"foo"}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "strconv.ParseFloat: parsing \"foo\": invalid syntax")
	form["feeShare"] = []string{"10"}

	// setOrchestratorPriceInfo is tested in webserver_test.go separately

	// Test no serviceURI
	form["serviceURI"] = []string{""}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "missing form param: serviceURI")

	// Test invalid ServiceURI
	form["serviceURI"] = []string{"hello world"}
	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusBadRequest, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
	assert.Equal(strings.TrimSpace(string(body)), "parse \"hello world\": invalid URI for request")
	form["serviceURI"] = []string{"http://foo.bar:1337"}

	req = bytes.NewBufferString(form.Encode())
	res, _ = http.Post(fmt.Sprintf("%s/activateOrchestrator", srv.URL), "application/x-www-form-urlencoded", req)
	require.Equal(http.StatusOK, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	res.Body.Close()
	require.Nil(err)
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
	// expected := fmt.Sprintf(`{"Manifests":{},"InternalManifests":{},"StreamInfo":{},"OrchestratorPool":[],"Version":"undefined","GolangRuntimeVersion":"%s","GOArch":"%s","GOOS":"%s","RegisteredTranscodersNumber":1,"RegisteredTranscoders":[{"Address":"TestAddress","Capacity":5}],"LocalTranscoding":false}`,
	// 	runtime.Version(), runtime.GOARCH, runtime.GOOS)
	expected := fmt.Sprintf(`{"Manifests":{},"InternalManifests":{},"StreamInfo":{},"OrchestratorPool":[],"OrchestratorPoolInfos":null,"Version":"undefined","GolangRuntimeVersion":"%s","GOArch":"%s","GOOS":"%s","RegisteredTranscodersNumber":1,"RegisteredTranscoders":[{"Address":"TestAddress","Capacity":5}],"LocalTranscoding":false,"BroadcasterPrices":{}}`,
		runtime.Version(), runtime.GOARCH, runtime.GOOS)
	assert.Equal(expected, string(body))
}

func TestGetEthChainID(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dbh, dbraw, err := common.TempDB(t)
	require.Nil(err)
	defer dbh.Close()
	defer dbraw.Close()
	require.Nil(err)
	err = dbh.SetChainID(big.NewInt(1))
	require.Nil(err)
	n, _ := core.NewLivepeerNode(&eth.StubClient{}, "./tmp", dbh)
	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/EthChainID", srv.URL))
	require.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
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
	assert.Equal("null", string(body))
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

	s, _ := NewLivepeerServer("127.0.0.1:1938", n, true, "")
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
	defer res.Body.Close()

	assert.Nil(err)
	assert.Equal(http.StatusInternalServerError, res.StatusCode)
}
