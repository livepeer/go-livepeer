package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatus(t *testing.T) {
	n, _ := core.NewLivepeerNode(nil, "./tmp", nil)
	n.NodeType = core.TranscoderNode
	n.TranscoderManager = core.NewRemoteTranscoderManager()
	strm := &common.StubServerStream{}
	transcoder := core.NewRemoteTranscoder(n, strm, 5)
	go func() { n.TranscoderManager.Manage(transcoder) }()
	time.Sleep(1 * time.Millisecond)
	n.Transcoder = n.TranscoderManager
	s := NewLivepeerServer("127.0.0.1:1938", "127.0.0.1:8080", n)
	mux := s.cliWebServerHandlers("addr")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res, err := http.Get(fmt.Sprintf("%s/status", srv.URL))
	assert := assert.New(t)
	req := require.New(t)
	req.Nil(err)
	assert.Equal(http.StatusOK, res.StatusCode)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	req.Nil(err)
	assert.Equal(`{"Manifests":{},"OrchestratorPool":[],"Version":"undefined","RegisteredTranscodersNumber":1,"RegisteredTranscoders":[{"Address":"TestAddress","Capacity":5}],"LocalTranscoding":false}`,
		string(body))
}
