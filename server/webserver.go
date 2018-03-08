package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ericxtang/m3u8"
	basicnet "github.com/livepeer/go-livepeer-basicnet"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
	"github.com/livepeer/lpms/transcoder"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	lpmon "github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
)

func (s *LivepeerServer) StartWebserver() {
	//Temporary endpoint just so we can invoke a transcode job.  IRL this should be invoked by transcoders monitoring the smart contract.
	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		strmID := r.URL.Query().Get("strmID")
		if strmID == "" {
			http.Error(w, "Need to specify strmID", 500)
			return
		}

		//Do transcoding
		ps := []ffmpeg.VideoProfile{ffmpeg.P240p30fps16x9, ffmpeg.P360p30fps16x9}
		tr := transcoder.NewFFMpegSegmentTranscoder(ps, "", s.LivepeerNode.WorkDir)
		config := net.TranscodeConfig{StrmID: strmID, Profiles: ps}
		ids, err := s.LivepeerNode.TranscodeAndBroadcast(config, nil, tr)
		if err != nil {
			glog.Errorf("Error transcoding: %v", err)
			http.Error(w, "Error transcoding.", 500)
		}

		//Get the manifest that contains the stream
		sid := core.StreamID(strmID)
		manifestID, _ := core.MakeManifestID(sid.GetNodeID(), sid.GetVideoID())
		mch, err := s.LivepeerNode.VideoNetwork.GetMasterPlaylist(string(sid.GetNodeID()), manifestID.String())
		if err != nil {
			glog.Errorf("Error getting manifest: %v", err)
			return
		}
		var manifest *m3u8.MasterPlaylist
		select {
		case manifest = <-mch:
		case <-time.After(time.Second):
			glog.Errorf("Get Master Playlist timed out.")
			return
		}

		//Update the manifest
		vids := make(map[core.StreamID]ffmpeg.VideoProfile)
		for i, vp := range ps {
			vids[ids[i]] = vp
			vParams := ffmpeg.VideoProfileToVariantParams(vp)
			pl, err := m3u8.NewMediaPlaylist(stream.DefaultHLSStreamWin, stream.DefaultHLSStreamCap)
			if err != nil {
				glog.Errorf("Error creating new media playlist: %v", err)
			}
			variant := &m3u8.Variant{URI: fmt.Sprintf("%v.m3u8", ids[i]), Chunklist: pl, VariantParams: vParams}
			manifest.Append(variant.URI, variant.Chunklist, variant.VariantParams)
		}
		s.LivepeerNode.VideoNetwork.UpdateMasterPlaylist(manifestID.String(), manifest)

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
		price, err := lpcommon.ParseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		transcodingOptions := r.FormValue("transcodingOptions")
		if transcodingOptions == "" {
			glog.Errorf("Need to provide transcoding options")
			return
		}

		profiles := []ffmpeg.VideoProfile{}
		for _, pName := range strings.Split(transcodingOptions, ",") {
			p, ok := ffmpeg.VideoProfileLookup[pName]
			if ok {
				profiles = append(profiles, p)
			}
		}
		if len(profiles) == 0 {
			glog.Errorf("Invalid transcoding options: %v", transcodingOptions)
			return
		}

		BroadcastPrice = price
		BroadcastJobVideoProfiles = profiles

		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfiles)
	})

	http.HandleFunc("/getBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		pNames := []string{}
		for _, p := range BroadcastJobVideoProfiles {
			pNames = append(pNames, p.Name)
		}
		config := struct {
			MaxPricePerSegment *big.Int
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
		transcodingOptions := make([]string, 0, len(ffmpeg.VideoProfileLookup))
		for opt := range ffmpeg.VideoProfileLookup {
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
		blockRewardCut, err := strconv.ParseFloat(blockRewardCutStr, 64)
		if err != nil {
			glog.Error(err)
			return
		}

		feeShareStr := r.FormValue("feeShare")
		if feeShareStr == "" {
			glog.Errorf("Need to provide fee share")
			return
		}
		feeShare, err := strconv.ParseFloat(feeShareStr, 64)
		if err != nil {
			glog.Error(err)
			return
		}

		priceStr := r.FormValue("pricePerSegment")
		if priceStr == "" {
			glog.Errorf("Need to provide price per segment")
			return
		}
		price, err := lpcommon.ParseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		amountStr := r.FormValue("amount")
		if amountStr == "" {
			glog.Errorf("Need to provide amount")
			return
		}
		amount, err := lpcommon.ParseBigInt(amountStr)
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

		}

		glog.Infof("Registering transcoder %v", s.LivepeerNode.Eth.Account().Address.Hex())

		tx, err := s.LivepeerNode.Eth.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare), price)
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
		blockRewardCut, err := strconv.ParseFloat(blockRewardCutStr, 64)
		if err != nil {
			glog.Errorf("Cannot convert block reward cut: %v", err)
			return
		}

		feeShareStr := r.FormValue("feeShare")
		if feeShareStr == "" {
			glog.Errorf("Need to provide fee share")
			return
		}
		feeShare, err := strconv.ParseFloat(feeShareStr, 64)
		if err != nil {
			glog.Errorf("Cannot convert fee share: %v", err)
			return
		}

		priceStr := r.FormValue("pricePerSegment")
		if priceStr == "" {
			glog.Errorf("Need to provide price per segment")
			return
		}
		price, err := lpcommon.ParseBigInt(priceStr)
		if err != nil {
			glog.Error(err)
			return
		}

		tx, err := s.LivepeerNode.Eth.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare), price)
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
			amount, err := lpcommon.ParseBigInt(amountStr)
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

	http.HandleFunc("/claimEarnings", func(w http.ResponseWriter, r *http.Request) {
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
			endRound, err := lpcommon.ParseBigInt(endRoundStr)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.ClaimEarnings(endRound)
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

	http.HandleFunc("/transcoderEarningPoolsForRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			roundStr := r.URL.Query().Get("round")
			round, err := lpcommon.ParseBigInt(roundStr)
			if err != nil {
				glog.Error(err)
				return
			}

			tp, err := s.LivepeerNode.Eth.GetTranscoderEarningsPoolForRound(s.LivepeerNode.Eth.Account().Address, round)
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
			amount, err := lpcommon.ParseBigInt(amountStr)
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

	http.HandleFunc("/withdrawDeposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			tx, err := s.LivepeerNode.Eth.Withdraw()
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Withdrew deposit")
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

	http.HandleFunc("/transcoderEventSubscriptions", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			activeEventSubMap := s.LivepeerNode.EthEventMonitor.EventSubscriptions()

			data, err := json.Marshal(activeEventSubMap)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	http.HandleFunc("/protocolParameters", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			lp := s.LivepeerNode.Eth

			numActiveTranscoders, err := lp.NumActiveTranscoders()
			if err != nil {
				glog.Error(err)
				return
			}

			roundLength, err := lp.RoundLength()
			if err != nil {
				glog.Error(err)
				return
			}

			roundLockAmount, err := lp.RoundLockAmount()
			if err != nil {
				glog.Error(err)
				return
			}

			unbondingPeriod, err := lp.UnbondingPeriod()
			if err != nil {
				glog.Error(err)
				return
			}

			verificationRate, err := lp.VerificationRate()
			if err != nil {
				glog.Error(err)
				return
			}

			verificationPeriod, err := lp.VerificationPeriod()
			if err != nil {
				glog.Error(err)
				return
			}

			slashingPeriod, err := lp.VerificationSlashingPeriod()
			if err != nil {
				glog.Error(err)
				return
			}

			failedVerificationSlashAmount, err := lp.FailedVerificationSlashAmount()
			if err != nil {
				glog.Error(err)
				return
			}

			missedVerificationSlashAmount, err := lp.MissedVerificationSlashAmount()
			if err != nil {
				glog.Error(err)
				return
			}

			doubleClaimSegmentSlashAmount, err := lp.DoubleClaimSegmentSlashAmount()
			if err != nil {
				glog.Error(err)
				return
			}

			finderFee, err := lp.FinderFee()
			if err != nil {
				glog.Error(err)
				return
			}

			inflation, err := lp.Inflation()
			if err != nil {
				glog.Error(err)
				return
			}

			inflationChange, err := lp.InflationChange()
			if err != nil {
				glog.Error(err)
				return
			}

			targetBondingRate, err := lp.TargetBondingRate()
			if err != nil {
				glog.Error(err)
				return
			}

			verificationCodeHash, err := lp.VerificationCodeHash()
			if err != nil {
				glog.Error(err)
				return
			}

			totalBonded, err := lp.GetTotalBonded()
			if err != nil {
				glog.Error(err)
				return
			}

			totalSupply, err := lp.TotalSupply()
			if err != nil {
				glog.Error(err)
				return
			}

			paused, err := lp.Paused()
			if err != nil {
				glog.Error(err)
				return
			}

			params := &lpTypes.ProtocolParameters{
				NumActiveTranscoders:          numActiveTranscoders,
				RoundLength:                   roundLength,
				RoundLockAmount:               roundLockAmount,
				UnbondingPeriod:               unbondingPeriod,
				VerificationRate:              verificationRate,
				VerificationPeriod:            verificationPeriod,
				SlashingPeriod:                slashingPeriod,
				FailedVerificationSlashAmount: failedVerificationSlashAmount,
				MissedVerificationSlashAmount: missedVerificationSlashAmount,
				DoubleClaimSegmentSlashAmount: doubleClaimSegmentSlashAmount,
				FinderFee:                     finderFee,
				Inflation:                     inflation,
				InflationChange:               inflationChange,
				TargetBondingRate:             targetBondingRate,
				VerificationCodeHash:          verificationCodeHash,
				TotalBonded:                   totalBonded,
				TotalSupply:                   totalSupply,
				Paused:                        paused,
			}

			data, err := json.Marshal(params)
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
				glog.Error(err)
				w.Write([]byte(""))
			} else {
				w.Write([]byte(b.String()))
			}
		}
	})

	http.HandleFunc("/ethBalance", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.Backend()
			if err != nil {
				glog.Error(err)
				return
			}

			balance, err := b.BalanceAt(context.Background(), s.LivepeerNode.Eth.Account().Address, nil)
			if err != nil {
				glog.Error(err)
				w.Write([]byte(""))
			} else {
				w.Write([]byte(balance.String()))
			}
		}
	})

	http.HandleFunc("/broadcasterDeposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			b, err := s.LivepeerNode.Eth.BroadcasterDeposit(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				w.Write([]byte(""))
			} else {
				w.Write([]byte(b.String()))
			}
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

	http.HandleFunc("/transferTokens", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			to := r.FormValue("to")
			if to == "" {
				glog.Errorf("Need to provide to address")
				return
			}

			amountStr := r.FormValue("amount")
			if amountStr == "" {
				glog.Errorf("Need to provide amount")
				return
			}
			amount, err := lpcommon.ParseBigInt(amountStr)
			if err != nil {
				glog.Error(err)
				return
			}

			tx, err := s.LivepeerNode.Eth.Transfer(common.HexToAddress(to), amount)
			if err != nil {
				glog.Error(err)
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Transferred %v to %v", eth.FormatUnits(amount, "LPT"), to)
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
