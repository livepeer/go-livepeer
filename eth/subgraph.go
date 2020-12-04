package eth

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
)

type LivepeerSubgraph interface {
	GetActiveTranscoders() ([]*lpTypes.Transcoder, error)
}

type livepeerSubgraph struct {
	http *http.Client
	addr string
}

func NewLivepeerSubgraph(addr string, timeout time.Duration) *livepeerSubgraph {
	return &livepeerSubgraph{
		http: &http.Client{
			Timeout: timeout,
		},
		addr: addr,
	}
}

func (s *livepeerSubgraph) GetActiveTranscoders() ([]*lpTypes.Transcoder, error) {
	type queryRes struct {
		Data struct {
			Transcoders []*struct {
				ID              string `json:"id"`
				FeeShare        bigInt `json:"feeShare"`
				RewardCut       bigInt `json:"rewardCut"`
				LastRewardRound struct {
					Number bigInt `json:"id"`
				}
				ActivationRound   bigInt `json:"activationRound"`
				DeactivationRound bigInt `json:"deactivationRound"`
				TotalStake        bigInt `json:"totalStake"`
				ServiceURI        string `json:"serviceURI"`
				Active            bool   `json:"active"`
				Status            string `json:"status"`
			}
		}
	}

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

	data := queryRes{}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	transcoders := []*lpTypes.Transcoder{}

	for _, t := range data.Data.Transcoders {

		transcoders = append(transcoders, &lpTypes.Transcoder{
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
		})
	}

	return transcoders, nil
}

type bigInt struct {
	big.Int
}

func (i *bigInt) UnmarshalJSON(b []byte) error {
	var val string
	err := json.Unmarshal(b, &val)
	if err != nil {
		return err
	}

	i.SetString(val, 10)

	return nil
}
