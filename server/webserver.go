package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/livepeer/lpms/stream"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	eth "github.com/livepeer/go-livepeer/eth"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/types"
)

func (s *LivepeerServer) StartWebserver() {
	//Temporary endpoint just so we can invoke a transcode job.  IRL this should be invoked by transcoders monitoring the smart contract.
	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		strmID := r.URL.Query().Get("strmID")
		if strmID == "" {
			http.Error(w, "Need to specify strmID", 500)
			return
		}

		ps := []types.VideoProfile{types.P240p30fps16x9, types.P360p30fps16x9}
		ids, err := s.LivepeerNode.TranscodeAndBroadcast(net.TranscodeConfig{StrmID: strmID, Profiles: ps}, nil)
		if err != nil {
			glog.Errorf("Error transcoding: %v", err)
			http.Error(w, "Error transcoding.", 500)
		}

		vids := make(map[core.StreamID]types.VideoProfile)
		for i, vp := range ps {
			vids[ids[i]] = vp
		}

		sid := core.StreamID(strmID)
		s.LivepeerNode.NotifyBroadcaster(sid.GetNodeID(), sid, vids)
	})

	//Set the broadcast config for creating onchain jobs.
	http.HandleFunc("/setBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("price")
		if p != "" {
			pi, err := strconv.Atoi(p)
			if err != nil {
				glog.Errorf("Price conversion failed: %v", err)
				return
			}
			BroadcastPrice = big.NewInt(int64(pi))
		}

		j := r.URL.Query().Get("job")
		if j != "" {
			jp := types.VideoProfileLookup[j]
			if jp.Name != "" {
				BroadcastJobVideoProfile = jp
			}
		}

		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfile.Name)
	})

	//Activate the transcoder on-chain.
	http.HandleFunc("/activateTranscoder", func(w http.ResponseWriter, r *http.Request) {
		registered, err := s.LivepeerNode.Eth.IsRegisteredTranscoder()
		if err != nil {
			glog.Errorf("Error checking for registered transcoder: %v", err)
			return
		}

		if registered {
			glog.Error("Transcoder is already registered")
			return
		}

		blockRewardCutStr := r.URL.Query().Get("blockRewardCut")
		if blockRewardCutStr == "" {
			glog.Errorf("Need to provide block reward cut")
			return
		}
		blockRewardCut, err := strconv.Atoi(blockRewardCutStr)
		if err != nil {
			glog.Errorf("Cannot convert block reward cut: %v", err)
			return
		}

		feeShareStr := r.URL.Query().Get("feeShare")
		if feeShareStr == "" {
			glog.Errorf("Need to provide fee share")
			return
		}
		feeShare, err := strconv.Atoi(feeShareStr)
		if err != nil {
			glog.Errorf("Cannot convert fee share: %v", err)
			return
		}

		pricePerSegmentStr := r.URL.Query().Get("pricePerSegment")
		if pricePerSegmentStr == "" {
			glog.Errorf("Need to provide price per segment")
			return
		}
		pricePerSegment, err := strconv.Atoi(pricePerSegmentStr)
		if err != nil {
			glog.Errorf("Cannot convert price per segment: %v", err)
			return
		}

		amountStr := r.URL.Query().Get("amount")
		if amountStr == "" {
			glog.Errorf("Need to provide amount")
			return
		}
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			glog.Errorf("Cannot convert amount: %v", err)
			return
		}

		if err := eth.CheckRoundAndInit(s.LivepeerNode.Eth); err != nil {
			glog.Errorf("Error checking and initializing round: %v", err)
			return
		}

		rc, ec := s.LivepeerNode.Eth.Transcoder(uint8(blockRewardCut), uint8(feeShare), big.NewInt(int64(pricePerSegment)))
		select {
		case <-rc:
			if amount > 0 {
				bondRc, bondEc := s.LivepeerNode.Eth.Bond(big.NewInt(int64(amount)), s.LivepeerNode.Eth.Account().Address)
				select {
				case rec := <-bondRc:
					glog.Infof("%v", rec)
				case err := <-bondEc:
					glog.Errorf("Error bonding: %v", err)
				}
			}
		case err := <-ec:
			glog.Errorf("Error registering as transcoder: %v", err)
		}
	})

	//Set transcoder config on-chain.
	http.HandleFunc("/setTranscoderConfig", func(w http.ResponseWriter, r *http.Request) {
		fc := r.URL.Query().Get("feecut")
		if fc != "" {
			fci, err := strconv.Atoi(fc)
			if err != nil {
				glog.Errorf("Fee cut conversion failed: %v", err)
				return
			}
			TranscoderFeeCut = uint8(fci)
		}

		rc := r.URL.Query().Get("rewardcut")
		if rc != "" {
			rci, err := strconv.Atoi(rc)
			if err != nil {
				glog.Errorf("Reward cut conversion failed: %v", err)
				return
			}
			TranscoderRewardCut = uint8(rci)
		}

		p := r.URL.Query().Get("price")
		if p != "" {
			pi, err := strconv.Atoi(p)
			if err != nil {
				glog.Errorf("Price conversion failed: %v", err)
				return
			}
			TranscoderSegmentPrice = big.NewInt(int64(pi))
		}

		glog.Infof("Transcoder Fee Cut: %v, Transcoder Reward Cut: %v, Transcoder Segment Price: %v", TranscoderFeeCut, TranscoderRewardCut, TranscoderSegmentPrice)
	})

	//Bond some amount of tokens to a transcoder.
	http.HandleFunc("/bond", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			//Parse amount
			amountStr := r.URL.Query().Get("amount")
			if amountStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			amount, err := strconv.Atoi(amountStr)
			if err != nil {
				glog.Errorf("Cannot convert amount: %v", err)
				return
			}

			//Parse transcoder address
			toAddr := r.URL.Query().Get("toAddr")
			if toAddr == "" {
				glog.Errorf("Need to provide transcoder address")
				return
			}

			if err := eth.CheckRoundAndInit(s.LivepeerNode.Eth); err != nil {
				glog.Errorf("Error checking and initializing round: %v", err)
				return
			}

			rc, ec := s.LivepeerNode.Eth.Bond(big.NewInt(int64(amount)), common.HexToAddress(toAddr))
			select {
			case rec := <-rc:
				glog.Infof("%v", rec)
			case err := <-ec:
				glog.Errorf("Error bonding: %v", err)
			}
		}
	})

	//Print the transcoder's stake
	http.HandleFunc("/transcoderStake", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.TranscoderStake()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/delegatorStake", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			s, err := s.LivepeerNode.Eth.DelegatorStake()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(s.String()))
		}
	})

	http.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			//Parse amount
			amountStr := r.URL.Query().Get("amount")
			if amountStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			amount, err := strconv.Atoi(amountStr)
			if err != nil {
				glog.Errorf("Cannot convert amount: %v", err)
				return
			}
			glog.Infof("Depositing: %v", amount)

			rc, ec := s.LivepeerNode.Eth.Deposit(big.NewInt(int64(amount)))
			select {
			case rec := <-rc:
				glog.Infof("%v", rec)
			case err := <-ec:
				glog.Errorf("Error depositing: %v", err)
			}
		}
	})

	//Print the current broadcast HLS streamID
	http.HandleFunc("/streamID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(LastHLSStreamID))
	})

	http.HandleFunc("/localStreams", func(w http.ResponseWriter, r *http.Request) {
		strmIDs := s.LivepeerNode.StreamDB.GetStreamIDs(stream.HLS)
		ret := make([]map[string]string, 0)
		for _, strmID := range strmIDs {
			ret = append(ret, map[string]string{"format": "hls", "streamID": strmID.String()})
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

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("StreamDB: %v", s.LivepeerNode.StreamDB)))
		w.Write([]byte(fmt.Sprintf("\n\nVideoNetwork: %v", s.LivepeerNode.VideoNetwork)))
	})

	http.HandleFunc("/nodeID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(s.LivepeerNode.VideoNetwork.GetNodeID()))
	})

	http.HandleFunc("/nodeAddrs", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Join(s.LivepeerNode.Addrs, ", ")))
	})

	http.HandleFunc("/protocolContractAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.Eth.GetProtocolAddr()))
		}
	})

	http.HandleFunc("/tokenContractAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.Eth.GetTokenAddr()))
		}
	})

	http.HandleFunc("/faucetContractAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.Eth.GetFaucetAddr()))
		}
	})

	http.HandleFunc("/ethAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.EthAccount))
		}
	})

	http.HandleFunc("/tokenBalance", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.TokenBalance()
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
			b, err := s.LivepeerNode.Eth.GetBroadcasterDeposit(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/transcoderBond", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.TranscoderBond()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(b.String()))
		}
	})

	http.HandleFunc("/isActiveTranscoder", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			reg, err := s.LivepeerNode.Eth.IsRegisteredTranscoder()
			if err != nil {
				w.Write([]byte("False"))
				return
			}
			active, err := s.LivepeerNode.Eth.IsActiveTranscoder()
			if err != nil {
				w.Write([]byte("False"))
				return
			}

			if reg && active {
				w.Write([]byte("True"))
			} else {
				w.Write([]byte("False"))
			}
			return
		}

		w.Write([]byte("False"))
	})

	http.HandleFunc("/allTranscoderStats", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			allTranscoderStats, err := s.LivepeerNode.Eth.GetAllTranscoderStats()
			if err != nil {
				w.Write([]byte(""))
			}

			data, err := json.Marshal(allTranscoderStats)
			if err != nil {
				glog.Errorf("Error marshalling all transcoder stats: %v", err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/transcoderPendingBlockRewardCut", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			blockRewardCut, _, _, err := s.LivepeerNode.Eth.TranscoderPendingPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(strconv.Itoa(int(blockRewardCut))))
		}
	})

	http.HandleFunc("/transcoderPendingFeeShare", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			_, feeShare, _, err := s.LivepeerNode.Eth.TranscoderPendingPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(strconv.Itoa(int(feeShare))))
		}
	})

	http.HandleFunc("/transcoderPendingPrice", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			_, _, price, err := s.LivepeerNode.Eth.TranscoderPendingPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(price.String()))
		}
	})

	http.HandleFunc("/transcoderBlockRewardCut", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			blockRewardCut, _, _, err := s.LivepeerNode.Eth.TranscoderPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(strconv.Itoa(int(blockRewardCut))))
		}
	})

	http.HandleFunc("/transcoderFeeShare", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			_, feeShare, _, err := s.LivepeerNode.Eth.TranscoderPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(strconv.Itoa(int(feeShare))))
		}
	})

	http.HandleFunc("/transcoderPrice", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			_, _, price, err := s.LivepeerNode.Eth.TranscoderPricingInfo()
			if err != nil {
				w.Write([]byte(""))
			}
			w.Write([]byte(price.String()))
		}
	})

	http.HandleFunc("/requestTokens", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			glog.Infof("Requesting tokens from faucet")

			rc, ec := s.LivepeerNode.Eth.RequestTokens()
			select {
			case rec := <-rc:
				glog.Infof("%v", rec)
			case err := <-ec:
				glog.Errorf("Error request tokens from faucet: %v", err)
			}
		}
	})
}
