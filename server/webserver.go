package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

func (s *LivepeerServer) StartWebserver(bindAddr string) {

	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}

	//Set the broadcast config for creating onchain jobs.
	mux.HandleFunc("/setBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/getBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/getAvailableTranscodingOptions", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/currentRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			currentRound, err := s.LivepeerNode.Eth.CurrentRound()
			if err != nil {
				glog.Error(err)
				return
			}

			w.Write([]byte(currentRound.String()))
		}
	})

	mux.HandleFunc("/initializeRound", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
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
	})

	mux.HandleFunc("/roundInitialized", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			initialized, err := s.LivepeerNode.Eth.CurrentRoundInitialized()
			if err != nil {
				glog.Error(err)
				return
			}
			w.Write([]byte(fmt.Sprintf("%v", initialized)))
		}
	})

	//Activate the transcoder on-chain.
	mux.HandleFunc("/activateTranscoder", func(w http.ResponseWriter, r *http.Request) {
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

		serviceURI := r.FormValue("serviceURI")
		if serviceURI == "" {
			glog.Errorf("Need to provide a service URI")
			return
		}
		if _, err := url.ParseRequestURI(serviceURI); err != nil {
			glog.Error(err)
			return
		}

		unbondingLockIDStr := r.FormValue("unbondingLockId")
		if unbondingLockIDStr != "" {
			unbondingLockID, err := lpcommon.ParseBigInt(unbondingLockIDStr)
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Rebonding with unbonding lock %v...", unbondingLockID)

			tx, err := s.LivepeerNode.Eth.RebondFromUnbonded(s.LivepeerNode.Eth.Account().Address, unbondingLockID)
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

		amountStr := r.FormValue("amount")
		if amountStr != "" {
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

		currentServiceURI, err := s.LivepeerNode.Eth.GetServiceURI(s.LivepeerNode.Eth.Account().Address)
		if err != nil {
			glog.Error(err)
			return
		}

		if currentServiceURI != serviceURI {
			glog.Infof("Storing service URI %v in service registry...", serviceURI)

			tx, err = s.LivepeerNode.Eth.SetServiceURI(serviceURI)
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

	mux.HandleFunc("/latestJobs", func(w http.ResponseWriter, r *http.Request) {
		countStr := r.FormValue("count")
		if countStr == "" {
			countStr = "5"
		}
		count, err := strconv.ParseInt(countStr, 10, 8)
		if err != nil {
			glog.Error(err)
			return
		}
		numJobs, err := s.LivepeerNode.Eth.NumJobs()
		if err != nil {
			glog.Error(err)
			return
		}
		ts := ""
		for i := numJobs.Int64() - count; i < numJobs.Int64(); i++ {
			j, err := s.LivepeerNode.Eth.GetJob(big.NewInt(i))
			if err != nil {
				glog.Error(err)
				continue
			}
			t, err := s.LivepeerNode.Eth.AssignedTranscoder(j)
			if err != nil {
				glog.Error(err)
				continue
			}
			ts = fmt.Sprintf("%vJob: %v, Price: %v, Broadcaster: %v, Transcoder: %v\n", ts, i, j.MaxPricePerSegment.String(), j.BroadcasterAddress.String(), t.String())
		}
		w.Write([]byte(ts))
	})

	//Set transcoder config on-chain.
	mux.HandleFunc("/setTranscoderConfig", func(w http.ResponseWriter, r *http.Request) {
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

		serviceURI := r.FormValue("serviceURI")
		if _, err := url.ParseRequestURI(serviceURI); err != nil {
			glog.Error(err)
			return
		}

		t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
		if err != nil {
			glog.Error(err)
			return
		}

		if t.PendingRewardCut.Cmp(eth.FromPerc(blockRewardCut)) != 0 || t.PendingFeeShare.Cmp(eth.FromPerc(feeShare)) != 0 || t.PendingPricePerSegment.Cmp(price) != 0 {
			glog.Infof("Setting transcoder config - Reward Cut: %v Fee Share: %v Price: %v", eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare), price)

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
		}

		if t.ServiceURI != serviceURI {
			glog.Infof("Storing service URI %v in service registry...", serviceURI)

			tx, err := s.LivepeerNode.Eth.SetServiceURI(serviceURI)
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

	//Bond some amount of tokens to a transcoder.
	mux.HandleFunc("/bond", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/rebond", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			unbondingLockIDStr := r.FormValue("unbondingLockId")
			if unbondingLockIDStr == "" {
				glog.Errorf("Need to provide unbondingLockId")
				return
			}
			unbondingLockID, err := lpcommon.ParseBigInt(unbondingLockIDStr)
			if err != nil {
				glog.Errorf("Cannot convert unbondingLockId: %v", err)
				return
			}

			var tx *types.Transaction

			toAddr := r.FormValue("toAddr")
			if toAddr != "" {
				// toAddr provided - invoke rebondFromUnbonded()
				tx, err = s.LivepeerNode.Eth.RebondFromUnbonded(common.HexToAddress(toAddr), unbondingLockID)
			} else {
				// toAddr not provided - invoke rebond()
				tx, err = s.LivepeerNode.Eth.Rebond(unbondingLockID)
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}
	})

	mux.HandleFunc("/unbond", func(w http.ResponseWriter, r *http.Request) {
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

			tx, err := s.LivepeerNode.Eth.Unbond(amount)
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

	mux.HandleFunc("/withdrawStake", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			unbondingLockIDStr := r.FormValue("unbondingLockId")
			if unbondingLockIDStr == "" {
				glog.Errorf("Need to provide unbondingLockID")
				return
			}
			unbondingLockID, err := lpcommon.ParseBigInt(unbondingLockIDStr)
			if err != nil {
				glog.Errorf("Cannot convert unbondingLockId: %v", err)
				return
			}
			tx, err := s.LivepeerNode.Eth.WithdrawStake(unbondingLockID)
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

	mux.HandleFunc("/unbondingLocks", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Database != nil {
			if err := r.ParseForm(); err != nil {
				glog.Errorf("Parse Form Error: %v", err)
				return
			}

			dAddr := s.LivepeerNode.Eth.Account().Address

			d, err := s.LivepeerNode.Eth.GetDelegator(dAddr)
			if err != nil {
				glog.Error(err)
				return
			}

			// Query for local IDs
			unbondingLockIDs, err := s.LivepeerNode.Database.UnbondingLockIDs()
			if err != nil {
				glog.Error(err)
				return
			}

			if big.NewInt(int64(len(unbondingLockIDs))).Cmp(d.NextUnbondingLockId) < 0 {
				// Generate all possible IDs
				missingUnbondingLockIDs := make(map[*big.Int]bool)
				for i := big.NewInt(0); i.Cmp(d.NextUnbondingLockId) < 0; i = new(big.Int).Add(i, big.NewInt(1)) {
					missingUnbondingLockIDs[i] = true
				}

				// Use local IDs to determine which IDs are missing
				for _, id := range unbondingLockIDs {
					delete(missingUnbondingLockIDs, id)
				}

				// Update unbonding locks in local DB if necessary
				for id := range missingUnbondingLockIDs {
					lock, err := s.LivepeerNode.Eth.GetDelegatorUnbondingLock(dAddr, id)
					if err != nil {
						glog.Error(err)
						continue
					}
					// If lock has been used (i.e. withdrawRound == 0) do not insert into DB
					// Note: We do not know what block at which a lock was used when querying the contract directly (as opposed to using events)
					// As a result, instead of having a lock entry in the DB with the usedBlock column set, we do not insert a lock entry at all
					if lock.WithdrawRound.Cmp(big.NewInt(0)) == 1 {
						if err := s.LivepeerNode.Database.InsertUnbondingLock(id, dAddr, lock.Amount, lock.WithdrawRound); err != nil {
							glog.Error(err)
							continue
						}
					}
				}
			}

			var currentRound *big.Int

			withdrawableStr := r.FormValue("withdrawable")
			if withdrawable, err := strconv.ParseBool(withdrawableStr); withdrawable {
				currentRound, err = s.LivepeerNode.Eth.CurrentRound()
				if err != nil {
					glog.Error(err)
					return
				}
			}

			unbondingLocks, err := s.LivepeerNode.Database.UnbondingLocks(currentRound)
			if err != nil {
				glog.Error(err)
				return
			}

			data, err := json.Marshal(unbondingLocks)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	mux.HandleFunc("/withdrawFees", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/claimEarnings", func(w http.ResponseWriter, r *http.Request) {
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

			claim := func() error {
				init, err := s.LivepeerNode.Eth.CurrentRoundInitialized()
				if err != nil {
					glog.Errorf("Trying to claim but round not initalized.")
					return err
				}
				if !init {
					return errors.New("Round not initialized")
				}
				err = s.LivepeerNode.Eth.ClaimEarnings(endRound)
				if err != nil {
					return err
				}
				return nil
			}

			if err := backoff.Retry(claim, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*15), 5)); err != nil {
				glog.Errorf("Error claiming earnings: %v", err)
			}
		}
	})

	mux.HandleFunc("/delegatorInfo", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/transcoderEarningPoolsForRound", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/withdrawDeposit", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			tx, err := s.LivepeerNode.Eth.Withdraw()
			if err != nil {
				glog.Error(err)
				w.Write([]byte(err.Error()))
				return
			}

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				w.Write([]byte(err.Error()))
				return
			}

			mes := "Withdrew deposit"
			glog.Infof(mes)
			w.Write([]byte(mes))
		}
	})

	//Print the current broadcast HLS streamID
	mux.HandleFunc("/streamID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(LastHLSStreamID))
	})

	mux.HandleFunc("/manifestID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(LastManifestID))
	})

	mux.HandleFunc("/localStreams", func(w http.ResponseWriter, r *http.Request) {
		// XXX fetch local streams?
		ret := make([]map[string]string, 0)
		js, err := json.Marshal(ret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	})

	mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("\n\nVideoSource: %v", s.LivepeerNode.VideoSource)))
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := s.LivepeerNode.VideoSource.GetNodeStatus()
		if status != nil {
			mstrs := make(map[string]string, 0)
			for mid, m := range status.Manifests {
				mstrs[string(mid)] = m.String()
			}
			d := struct {
				Manifests map[string]string
			}{
				Manifests: mstrs,
			}
			if data, err := json.Marshal(d); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
				return
			}
		}
	})

	mux.HandleFunc("/contractAddresses", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/transcoderEventSubscriptions", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil && s.LivepeerNode.EthEventMonitor != nil {
			rewardWorking := false
			if s.LivepeerNode.EthServices["RewardService"] != nil && s.LivepeerNode.EthServices["RewardService"].IsWorking() {
				rewardWorking = true
			}
			roundWorking := false
			if s.LivepeerNode.EthServices["RoundService"] != nil && s.LivepeerNode.EthServices["RoundService"].IsWorking() {
				roundWorking = true
			}
			jobWorking := false
			if s.LivepeerNode.EthServices["JobService"] != nil && s.LivepeerNode.EthServices["JobService"].IsWorking() {
				jobWorking = true
			}

			m := map[string]bool{"JobService": jobWorking, "RewardService": rewardWorking, "RoundsService": roundWorking}
			data, err := json.Marshal(m)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	mux.HandleFunc("/protocolParameters", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/ethAddr", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			w.Write([]byte(s.LivepeerNode.Eth.Account().Address.Hex()))
		}
	})

	mux.HandleFunc("/tokenBalance", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/ethBalance", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/broadcasterDeposit", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/registeredTranscoders", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/transcoderInfo", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/transferTokens", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/requestTokens", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/IsTranscoder", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("%v", s.LivepeerNode.NodeType == core.Transcoder)))
	})

	mux.HandleFunc("/EthNetworkID", func(w http.ResponseWriter, r *http.Request) {
		be, err := s.LivepeerNode.Eth.Backend()
		if err != nil {
			glog.Errorf("Error getting eth backend: %v", err)
			return
		}
		networkID, err := be.NetworkID(context.Background())
		if err != nil {
			glog.Errorf("Error getting eth network ID: %v", err)
		}
		w.Write([]byte(networkID.String()))
	})

	mux.HandleFunc("/reward", func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("Calling reward")
		tx, err := s.LivepeerNode.Eth.Reward()
		if err != nil {
			glog.Errorf("Error calling reward: %v", err)
			return
		}
		if err := s.LivepeerNode.Eth.CheckTx(tx); err != nil {
			glog.Errorf("Error calling reward: %v", err)
			return
		}
		glog.Infof("Call to reward successful")
	})

	mux.HandleFunc("/gasPrice", func(w http.ResponseWriter, r *http.Request) {
		_, gprice := s.LivepeerNode.Eth.GetGasInfo()
		if gprice == nil {
			w.Write([]byte("0"))
		} else {
			w.Write([]byte(gprice.String()))
		}
	})

	mux.HandleFunc("/setGasPrice", func(w http.ResponseWriter, r *http.Request) {
		amount := r.FormValue("amount")
		if amount == "" {
			glog.Errorf("Need to set amount")
			return
		}

		gprice, err := lpcommon.ParseBigInt(amount)
		if err != nil {
			glog.Errorf("Parsing failed for price: %v", err)
			return
		}
		if amount == "0" {
			gprice = nil
		}

		glimit, _ := s.LivepeerNode.Eth.GetGasInfo()
		if err := s.LivepeerNode.Eth.SetGasInfo(glimit, gprice); err != nil {
			glog.Errorf("Error setting price info: %v", err)
		}
	})

	glog.Info("CLI server listening on ", bindAddr)
	srv.ListenAndServe()

}
