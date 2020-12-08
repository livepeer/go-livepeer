package subgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

type LivepeerSubgraph interface {
	GetActiveTranscoders() ([]*lpTypes.Transcoder, error)
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type livepeerSubgraph struct {
	http httpClient
	addr string
}

type data struct {
	Data json.RawMessage
}

func NewLivepeerSubgraph(addr string, timeout time.Duration) (*livepeerSubgraph, error) {
	url, err := common.ValidateURL(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid subgraph URL: %v", err)
	}
	return &livepeerSubgraph{
		http: &http.Client{
			Timeout: timeout,
		},
		addr: url,
	}, nil
}

func (s *livepeerSubgraph) GetActiveTranscoders() ([]*lpTypes.Transcoder, error) {
	query := map[string]string{
		"query": `
		{
			transcoders(where: {active: true}) {
			  	id
			  	feeShare
			 	rewardCut
			  	lastRewardRound {
					id
			  	}
			  	activationRound
			  	deactivationRound
			  	totalStake
				serviceURI
			  	active
			  	status
			}
		  }
		`,
	}

	input, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", s.addr, bytes.NewBuffer(input))
	if err != nil {
		return nil, err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errors.New(string(body))
	}

	data := data{}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	transcodersJSON := []*transcoder{}
	if err := json.Unmarshal(data.Data, &transcodersJSON); err != nil {
		return nil, err
	}

	transcoders := []*lpTypes.Transcoder{}

	for _, t := range transcodersJSON {
		transcoders = append(transcoders, t.parseLivepeerTranscoder())
	}

	return transcoders, nil
}

type bigInt struct {
	big.Int
}

func (b bigInt) MarshalJSON() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *bigInt) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	var z big.Int
	_, ok := z.SetString(string(p), 10)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", p)
	}
	b.Int = z
	return nil
}

type transcoder struct {
	ID                string `json:"id"`
	FeeShare          bigInt `json:"feeShare"`
	RewardCut         bigInt `json:"rewardCut"`
	LastRewardRound   *round `json:"lastRewardRound"`
	ActivationRound   bigInt `json:"activationRound"`
	DeactivationRound bigInt `json:"deactivationRound"`
	TotalStake        bigInt `json:"totalStake"`
	ServiceURI        string `json:"serviceURI"`
	Active            bool   `json:"active"`
	Status            string `json:"status"`
}

func (t *transcoder) parseLivepeerTranscoder() *lpTypes.Transcoder {
	return &lpTypes.Transcoder{
		Address:           ethcommon.HexToAddress(t.ID),
		ServiceURI:        t.ServiceURI,
		LastRewardRound:   &t.LastRewardRound.Number.Int,
		RewardCut:         &t.RewardCut.Int,
		FeeShare:          &t.FeeShare.Int,
		DelegatedStake:    &t.TotalStake.Int,
		ActivationRound:   &t.ActivationRound.Int,
		DeactivationRound: &t.DeactivationRound.Int,
		Active:            t.Active,
		Status:            t.Status,
	}
}

type round struct {
	Number bigInt `json:"id"`
}
