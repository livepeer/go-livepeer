package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	basicnet "github.com/livepeer/go-livepeer-basicnet"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/transcoder"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
)

var (
	ErrParseBigInt = fmt.Errorf("failed to parse big integer")
)

func parseBigInt(num string) (*big.Int, error) {
	bigNum := new(big.Int)
	bigNum.SetString(num, 10)

	if bigNum == nil {
		return nil, ErrParseBigInt
	} else {
		return bigNum, nil
	}
}

func convertPerc(value int) *big.Int {
	return big.NewInt(int64(value * 10000))
}

func (s *LivepeerServer) StartWebserver() {
	//Temporary endpoint just so we can invoke a transcode job.  IRL this should be invoked by transcoders monitoring the smart contract.
	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		strmID := r.URL.Query().Get("strmID")
		if strmID == "" {
			http.Error(w, "Need to specify strmID", 500)
			return
		}

		ps := []lpmscore.VideoProfile{lpmscore.P240p30fps16x9, lpmscore.P360p30fps16x9}
		tr := transcoder.NewFFMpegSegmentTranscoder(ps, "", s.LivepeerNode.WorkDir)
		config := net.TranscodeConfig{StrmID: strmID, Profiles: ps}
		ids, err := s.LivepeerNode.TranscodeAndBroadcast(config, nil, tr)
		if err != nil {
			glog.Errorf("Error transcoding: %v", err)
			http.Error(w, "Error transcoding.", 500)
		}

		vids := make(map[core.StreamID]lpmscore.VideoProfile)
		for i, vp := range ps {
			vids[ids[i]] = vp
		}

		sid := core.StreamID(strmID)
		s.LivepeerNode.NotifyBroadcaster(sid.GetNodeID(), sid, vids)
	})

	//Set the broadcast config for creating onchain jobs.
	http.HandleFunc("/setBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			glog.Errorf("Parse Form Error: %v", err)
			return
		}

		priceStr := r.FormValue("maxPricePerSegment")
		if priceStr == "" {
			glog.Errorf("Need to provide max price per segment")
			return
		}
		price, err := parseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		transcodingOptions := r.FormValue("transcodingOptions")
		if transcodingOptions == "" {
			glog.Errorf("Need to provide transcoding options")
			return
		}

		profiles := []lpmscore.VideoProfile{}
		for _, pName := range strings.Split(transcodingOptions, ",") {
			p, ok := lpmscore.VideoProfileLookup[pName]
			if ok {
				profiles = append(profiles, p)
			}
		}
		if len(profiles) == 0 {
			glog.Errorf("Invalid transcoding options: %v", transcodingOptions)
			return
		}

		BroadcastPrice = uint64(price.Uint64())
		BroadcastJobVideoProfiles = profiles

		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfiles)
	})

	http.HandleFunc("/getBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		pNames := []string{}
		for _, p := range BroadcastJobVideoProfiles {
			pNames = append(pNames, p.Name)
		}
		config := struct {
			MaxPricePerSegment uint64
			TranscodingOptions string
		}{
			BroadcastPrice,
			strings.Join(pNames, ","),
		}

		data, err := json.Marshal(config)
		if err != nil {
			glog.Errorf("Error marshalling broadcaster config: %v", err)
			return
		}

		w.Write(data)
	})

	http.HandleFunc("/getAvailableTranscodingOptions", func(w http.ResponseWriter, r *http.Request) {
		transcodingOptions := make([]string, 0, len(lpmscore.VideoProfileLookup))
		for opt := range lpmscore.VideoProfileLookup {
			transcodingOptions = append(transcodingOptions, opt)
		}

		data, err := json.Marshal(transcodingOptions)
		if err != nil {
			glog.Errorf("Error marshalling all transcoding options: %v", err)
			return
		}

		w.Write(data)
	})

	http.HandleFunc("/currentRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			currentRound, err := s.LivepeerNode.Eth.CurrentRound()
			if err != nil {
				glog.Error(err)
				return
			}

			w.Write([]byte(currentRound.String()))
		}
	})

	http.HandleFunc("/initializeRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			initialized, err := s.LivepeerNode.Eth.CurrentRoundInitialized()
			if err != nil {
				glog.Error(err)
				return
			}

			if !initialized {
				tx, err := s.LivepeerNode.Eth.InitializeRound()
				if err != nil {
					glog.Error(err)
					return
				}

				err = s.LivepeerNode.Eth.CheckTx(tx)
				if err != nil {
					glog.Error(err)
					return
				}
			}
		}
	})

	//Activate the transcoder on-chain.
	http.HandleFunc("/activateTranscoder", func(w http.ResponseWriter, r *http.Request) {
		t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
		if err != nil {
			glog.Error(err)
			return
		}

		if t.Status == "Registered" {
			glog.Error("Transcoder is already registered")
			return
		}

		if err := r.ParseForm(); err != nil {
			glog.Errorf("Parse Form Error: %v", err)
			return
		}

		blockRewardCutStr := r.FormValue("blockRewardCut")
		if blockRewardCutStr == "" {
			glog.Errorf("Need to provide block reward cut")
			return
		}
		blockRewardCut, err := strconv.Atoi(blockRewardCutStr)
		if err != nil {
			glog.Error(err)
			return
		}

		feeShareStr := r.FormValue("feeShare")
		if feeShareStr == "" {
			glog.Errorf("Need to provide fee share")
			return
		}
		feeShare, err := strconv.Atoi(feeShareStr)
		if err != nil {
			glog.Error(err)
			return
		}

		priceStr := r.FormValue("pricePerSegment")
		if priceStr == "" {
			glog.Errorf("Need to provide price per segment")
			return
		}
		price, err := parseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		amountStr := r.FormValue("amount")
		if amountStr == "" {
			glog.Errorf("Need to provide amount")
			return
		}
		amount, err := parseBigInt(amountStr)
		if err != nil {
			glog.Error(err)
			return
		}

		if amount.Cmp(big.NewInt(0)) == 1 {
			glog.Infof("Bonding %v...", amount)

			tx, err := s.LivepeerNode.Eth.Bond(amount, s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Registering transcoder %v", s.LivepeerNode.Eth.Account().Address.Hex())

			tx, err = s.LivepeerNode.Eth.Transcoder(convertPerc(blockRewardCut), convertPerc(feeShare), price)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	//Set transcoder config on-chain.
	http.HandleFunc("/setTranscoderConfig", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			glog.Errorf("Parse Form Error: %v", err)
			return
		}

		blockRewardCutStr := r.FormValue("blockRewardCut")
		if blockRewardCutStr == "" {
			glog.Errorf("Need to provide block reward cut")
			return
		}
		blockRewardCut, err := strconv.Atoi(blockRewardCutStr)
		if err != nil {
			glog.Errorf("Cannot convert block reward cut: %v", err)
			return
		}

		feeShareStr := r.FormValue("feeShare")
		if feeShareStr == "" {
			glog.Errorf("Need to provide fee share")
			return
		}
		feeShare, err := strconv.Atoi(feeShareStr)
		if err != nil {
			glog.Errorf("Cannot convert fee share: %v", err)
			return
		}

		priceStr := r.FormValue("pricePerSegment")
		if priceStr == "" {
			glog.Errorf("Need to provide price per segment")
			return
		}
		price, err := parseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		tx, err := s.LivepeerNode.Eth.Transcoder(convertPerc(blockRewardCut), convertPerc(feeShare), price)
		if err != nil {
			glog.Error(err)
			return
		}

		err = s.LivepeerNode.Eth.CheckTx(tx)
		if err != nil {
			glog.Error(err)
			return
		}
	})

	//Bond some amount of tokens to a transcoder.
	http.HandleFunc("/bond", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			amountStr := r.FormValue("amount")
			if amountStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			amount, err := parseBigInt(amountStr)
			if err != nil {
				glog.Errorf("Cannot convert amount: %v", err)
				return
			}

			toAddr := r.FormValue("toAddr")
			if toAddr == "" {
				glog.Errorf("Need to provide to addr")
				return
			}

			tx, err := s.LivepeerNode.Eth.Bond(amount, common.HexToAddress(toAddr))
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	http.HandleFunc("/unbond", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			tx, err := s.LivepeerNode.Eth.Unbond()
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	http.HandleFunc("/withdrawStake", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			tx, err := s.LivepeerNode.Eth.WithdrawStake()
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	http.HandleFunc("/withdrawFees", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			tx, err := s.LivepeerNode.Eth.WithdrawFees()
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	http.HandleFunc("/claimTokenPoolsShares", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			endRoundStr := r.FormValue("endRound")
			if endRoundStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			endRound, err := parseBigInt(endRoundStr)
			if err != nil {
				glog.Error(err)
				return
			}

			tx, err := s.LivepeerNode.Eth.ClaimTokenPoolsShares(endRound)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	http.HandleFunc("/delegatorInfo", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			d, err := s.LivepeerNode.Eth.GetDelegator(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}

			data, err := json.Marshal(d)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/transcoderTokenPoolsForRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			roundStr := r.URL.Query().Get("round")
			round, err := parseBigInt(roundStr)
			if err != nil {
				glog.Error(err)
				return
			}

			tp, err := s.LivepeerNode.Eth.GetTranscoderTokenPoolsForRound(s.LivepeerNode.Eth.Account().Address, round)
			if err != nil {
				glog.Error(err)
				return
			}

			data, err := json.Marshal(tp)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			//Parse amount
			amountStr := r.FormValue("amount")
			if amountStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			amount, err := parseBigInt(amountStr)
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Depositing: %v", amount)

			tx, err := s.LivepeerNode.Eth.Deposit(amount)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	//Print the current broadcast HLS streamID
	http.HandleFunc("/streamID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(LastHLSStreamID))
	})

	http.HandleFunc("/manifestID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(LastManifestID))
	})

	http.HandleFunc("/localStreams", func(w http.ResponseWriter, r *http.Request) {
		net := s.LivepeerNode.VideoNetwork.(*basicnet.BasicVideoNetwork)
		ret := make([]map[string]string, 0)
		for _, strmID := range net.GetLocalStreams() {
			ret = append(ret, map[string]string{"format": "hls", "streamID": strmID})
		}
		js, err := json.Marshal(ret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	})

	http.HandleFunc("/peersCount", func(w http.ResponseWriter, r *http.Request) {
		ret := make(map[string]int)
		ret["count"] = lpmon.Instance().GetPeerCount()

		js, err := json.Marshal(ret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	})

	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("\n\nVideoNetwork: %v", s.LivepeerNode.VideoNetwork)))
		w.Write([]byte(fmt.Sprintf("\n\nmediaserver sub timer: %v", s.hlsSubTimer)))
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		nid := r.FormValue("nodeID")

		if nid == "" {
			nid = string(s.LivepeerNode.Identity)
		}

		statusc, err := s.LivepeerNode.VideoNetwork.GetNodeStatus(nid)
		if err == nil {
			status := <-statusc
			mstrs := make(map[string]string, 0)
			for mid, m := range status.Manifests {
				mstrs[mid] = m.String()
			}
			d := struct {
				NodeID    string
				Manifests map[string]string
			}{
				NodeID:    status.NodeID,
				Manifests: mstrs,
			}
			if data, err := json.Marshal(d); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
				return
			}
		}
	})

	http.HandleFunc("/nodeID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(s.LivepeerNode.VideoNetwork.GetNodeID()))
	})

	http.HandleFunc("/nodeAddrs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Join(s.LivepeerNode.Addrs, ", ")))
	})

	http.HandleFunc("/contractAddresses", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			addrMap := s.LivepeerNode.Eth.ContractAddresses()

			data, err := json.Marshal(addrMap)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/ethAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.Eth.Account().Address.Hex()))
		}
	})

	http.HandleFunc("/tokenBalance", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.BalanceOf(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/ethBalance", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.Backend().BalanceAt(context.Background(), s.LivepeerNode.Eth.Account().Address, nil)
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/broadcasterDeposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.BroadcasterDeposit(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/registeredTranscoders", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			transcoders, err := s.LivepeerNode.Eth.RegisteredTranscoders()
			if err != nil {
				glog.Error(err)
				return
			}

			data, err := json.Marshal(transcoders)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/transcoderInfo", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}

			data, err := json.Marshal(t)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/requestTokens", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			glog.Infof("Requesting tokens from faucet")

			tx, err := s.LivepeerNode.Eth.Request()
			if err != nil {
				glog.Errorf("Error requesting tokens from faucet: %v", err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Errorf("Error requesting tokens from faucet: %v", err)
				return
			}
		}
	})
}
