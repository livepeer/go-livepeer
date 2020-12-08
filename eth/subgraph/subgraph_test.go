package subgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHttp struct {
	err        error
	statusCode int
	json       string
}

func (s *stubHttp) Do(req *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.statusCode < 200 || s.statusCode >= 300 {
		return &http.Response{
			Body:       ioutil.NopCloser(strings.NewReader(s.err.Error())),
			StatusCode: s.statusCode,
		}, nil
	}
	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(strings.NewReader(s.json)),
	}, nil
}

func TestNewLivepeerSubgraph(t *testing.T) {
	assert := assert.New(t)
	sg, err := NewLivepeerSubgraph("hello.world", 5*time.Second)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid subgraph URL")
	assert.Nil(sg)

	sg, err = NewLivepeerSubgraph("https://api.thegraph.com/subgraphs/name/livepeer/livepeer", 5*time.Second)
	assert.Nil(err)
	assert.NotNil(sg)
}

func TestGetActiveTranscoders(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	sg := &livepeerSubgraph{
		http: &stubHttp{},
		addr: "foo.bar",
	}

	// test http client error
	expErr := errors.New("some error")
	http := &stubHttp{err: expErr}
	sg.http = http
	ts, err := sg.GetActiveTranscoders()
	assert.Nil(ts)
	assert.EqualError(err, expErr.Error())

	// test not-OK statusCode
	expErr = errors.New("bad request")
	http.err = expErr
	http.statusCode = 400
	ts, err = sg.GetActiveTranscoders()
	assert.Nil(ts)
	assert.EqualError(err, expErr.Error())

	// test OK
	tIDs := []string{"foo", "bar", "baz"}
	var transcoders []*transcoder
	for i, id := range tIDs {
		transcoders = append(transcoders, &transcoder{
			ID:        id,
			FeeShare:  bigInt{*big.NewInt(int64(i + 10))},
			RewardCut: bigInt{*big.NewInt(int64(i + 10))},
			LastRewardRound: &round{
				Number: bigInt{*big.NewInt(int64(i))},
			},
			ActivationRound:   bigInt{*big.NewInt(int64(i))},
			DeactivationRound: bigInt{*big.NewInt(int64(i))},
			TotalStake:        bigInt{*big.NewInt(int64(i))},
			ServiceURI:        fmt.Sprintf("%v:%v", id, i),
			Active:            true,
			Status:            "Registered",
		})
	}
	jsonTs, err := json.Marshal(transcoders)
	require.Nil(err)
	data := data{
		Data: json.RawMessage(jsonTs),
	}
	dataJSON, err := json.Marshal(data)
	require.Nil(err)
	sg.http = &stubHttp{
		json:       string(dataJSON),
		statusCode: 200,
	}
	ts, err = sg.GetActiveTranscoders()
	assert.Nil(err)
	for i, t := range ts {
		assert.Equal(t, transcoders[i].parseLivepeerTranscoder())
	}
}

func TestParseLivepeerTranscoder(t *testing.T) {
	assert := assert.New(t)
	id := "foo"
	feeShare := int64(50)
	rewardCut := int64(100)
	lastRewardRound := int64(1500)
	activationRound := int64(1300)
	deactivationRound := int64(50000)
	totalStake := int64(1030403403)
	serviceURI := "foo:bar"
	status := "registered"
	sgT := &transcoder{
		ID:        id,
		FeeShare:  bigInt{*big.NewInt(feeShare)},
		RewardCut: bigInt{*big.NewInt(rewardCut)},
		LastRewardRound: &round{
			Number: bigInt{*big.NewInt(lastRewardRound)},
		},
		ActivationRound:   bigInt{*big.NewInt(activationRound)},
		DeactivationRound: bigInt{*big.NewInt(deactivationRound)},
		TotalStake:        bigInt{*big.NewInt(totalStake)},
		ServiceURI:        serviceURI,
		Active:            true,
		Status:            status,
	}

	lpT := sgT.parseLivepeerTranscoder()
	assert.Equal(lpT.Address.Hex(), ethcommon.HexToAddress(id).Hex())
	assert.Equal(lpT.FeeShare.Int64(), feeShare)
	assert.Equal(lpT.RewardCut.Int64(), rewardCut)
	assert.Equal(lpT.LastRewardRound.Int64(), lastRewardRound)
	assert.Equal(lpT.ActivationRound.Int64(), activationRound)
	assert.Equal(lpT.DeactivationRound.Int64(), deactivationRound)
	assert.Equal(lpT.DelegatedStake.Int64(), totalStake)
	assert.Equal(lpT.ServiceURI, serviceURI)
	assert.Equal(lpT.Active, true)
	assert.Equal(lpT.Status, status)
}
