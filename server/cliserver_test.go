package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
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
	s := NewLivepeerServer("127.0.0.1:1938", n)
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

func TestGetEthNetworkID(t *testing.T) {
	srv := newMockServer()
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/EthNetworkID", srv.URL))
	assert := assert.New(t)
	req := require.New(t)
	req.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	req.Nil(err)
	assert.Equal("offchain", string(body))
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
