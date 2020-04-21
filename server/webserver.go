package server

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	// pprof adds handlers to default mux via `init()`
	_ "net/http/pprof"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/monitor"
	ffmpeg "github.com/livepeer/lpms/ffmpeg"
)

var vFlag *glog.Level = flag.Lookup("v").Value.(*glog.Level)

func (s *LivepeerServer) setServiceURI(serviceURI string) error {

	parsedURI, err := url.Parse(serviceURI)
	if err != nil {
		glog.Error(err)
		return err
	}

	glog.Infof("Storing service URI %v in service registry...", serviceURI)

	tx, err := s.LivepeerNode.Eth.SetServiceURI(serviceURI)
	if err != nil {
		glog.Error(err)
		return err
	}

	err = s.LivepeerNode.Eth.CheckTx(tx)
	if err != nil {
		glog.Error(err)
		return err
	}

	// Avoids a restart if only the host has been changed.
	// If the port has been changed, a restart is still needed.
	s.LivepeerNode.SetServiceURI(parsedURI)

	return nil
}

// StartCliWebserver starts web server for CLI
// blocks until exit
func (s *LivepeerServer) StartCliWebserver(bindAddr string) {
	mux := s.cliWebServerHandlers(bindAddr)
	srv := &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}

	glog.Info("CLI server listening on ", bindAddr)
	srv.ListenAndServe()
}

func (s *LivepeerServer) cliWebServerHandlers(bindAddr string) *http.ServeMux {
	// Override default mux because pprof only uses the default mux
	// We really don't want to accidentally pull pprof into other listeners.
	// Pprof, like the CLI, is a strictly private API!
	mux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	mux.Handle("/signMessage", mustHaveFormParams(signMessageHandler(s.LivepeerNode.Eth), "message"))

	mux.Handle("/vote", mustHaveFormParams(voteHandler(s.LivepeerNode.Eth), "poll", "choiceID"))

	//Set the broadcast config for creating onchain jobs.
	mux.HandleFunc("/setBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			glog.Errorf("Parse Form Error: %v", err)
			return
		}

		pricePerUnit := r.FormValue("maxPricePerUnit")
		pr, err := strconv.ParseInt(pricePerUnit, 10, 64)
		if err != nil {
			glog.Errorf("Error converting string to int64: %v\n", err)
			return
		}

		pixelsPerUnit := r.FormValue("pixelsPerUnit")
		px, err := strconv.ParseInt(pixelsPerUnit, 10, 64)
		if err != nil {
			glog.Errorf("Error converting string to int64: %v\n", err)
			return
		}
		if px <= 0 {
			glog.Errorf("pixels per unit must be greater than 0, provided %d\n", px)
			return
		}

		var price *big.Rat
		if pr > 0 {
			price = big.NewRat(pr, px)
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
		BroadcastCfg.SetMaxPrice(price)
		BroadcastJobVideoProfiles = profiles
		if price != nil {
			glog.Infof("Maximum transcoding price: %d per %q pixels\n", pr, px)
		} else {
			glog.Info("Maximum transcoding price per pixel not set, broadcaster is currently set to accept ANY price.\n")
		}
		glog.Infof("Transcode Job Type: %v", BroadcastJobVideoProfiles)
	})

	mux.HandleFunc("/getBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		pNames := []string{}
		for _, p := range BroadcastJobVideoProfiles {
			pNames = append(pNames, p.Name)
		}
		config := struct {
			MaxPrice           *big.Rat
			TranscodingOptions string
		}{
			BroadcastCfg.MaxPrice(),
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

	mux.Handle("/currentRound", currentRoundHandler(s.LivepeerNode.Eth))

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

	//Activate the orchestrator on-chain.
	mux.HandleFunc("/activateOrchestrator", func(w http.ResponseWriter, r *http.Request) {
		t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
		if err != nil {
			glog.Error(err)
			return
		}

		if t.Status == "Registered" {
			glog.Error("Orchestrator is already registered")
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

		if err := s.setOrchestratorPriceInfo(r.FormValue("pricePerUnit"), r.FormValue("pixelsPerUnit")); err != nil {
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

		glog.Infof("Setting orchestrator commission rates %v", s.LivepeerNode.Eth.Account().Address.Hex())

		tx, err := s.LivepeerNode.Eth.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare))
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
			if err := s.setServiceURI(serviceURI); err != nil {
				return
			}
		}
	})

	//Set transcoder config on-chain.
	mux.HandleFunc("/setOrchestratorConfig", func(w http.ResponseWriter, r *http.Request) {
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

		if err := s.setOrchestratorPriceInfo(r.FormValue("pricePerUnit"), r.FormValue("pixelsPerUnit")); err != nil {
			glog.Error(err)
			return
		}

		t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
		if err != nil {
			glog.Error(err)
			return
		}

		if t.RewardCut.Cmp(eth.FromPerc(blockRewardCut)) != 0 || t.FeeShare.Cmp(eth.FromPerc(feeShare)) != 0 {
			tx, err := s.LivepeerNode.Eth.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare))
			if err != nil {
				glog.Error(err)
				return
			}

			glog.Infof("Setting orchestrator commission rates for %v: reward cut=%v feeshare=%v", s.LivepeerNode.Eth.Account().Address.Hex(), blockRewardCut, feeShare)

			err = s.LivepeerNode.Eth.CheckTx(tx)
			if err != nil {
				glog.Error(err)
				return
			}
		}

		serviceURI := r.FormValue("serviceURI")
		if _, err := url.ParseRequestURI(serviceURI); err != nil {
			glog.Error(err)
			return
		}

		if t.ServiceURI != serviceURI {
			if err := s.setServiceURI(serviceURI); err != nil {
				return
			}
		}
	})

	//Bond some amount of tokens to an orchestrator.
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
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
		}
	})

	mux.HandleFunc("/orchestratorEarningPoolsForRound", func(w http.ResponseWriter, r *http.Request) {
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

	//Print the current broadcast HLS streamID
	mux.HandleFunc("/streamID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(s.LastHLSStreamID().String()))
	})

	mux.HandleFunc("/manifestID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(s.LastManifestID()))
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
		w.Write([]byte(fmt.Sprintf("\n\nLatestPlaylist: %v", s.LatestPlaylist())))
	})

	mux.HandleFunc("/getLogLevel", func(w http.ResponseWriter, r *http.Request) {
		if vFlag == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(vFlag.String()))
	})

	mux.HandleFunc("/setLogLevel", func(w http.ResponseWriter, r *http.Request) {
		if vFlag == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err := vFlag.Set(r.FormValue("loglevel"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := s.GetNodeStatus()
		if status != nil {
			if data, err := json.Marshal(status); err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
				return
			}
		}
		http.Error(w, "Error getting status", http.StatusInternalServerError)
	})

	mux.HandleFunc("/contractAddresses", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			addrMap := s.LivepeerNode.Eth.ContractAddresses()

			data, err := json.Marshal(addrMap)
			if err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
		}
	})

	mux.HandleFunc("/protocolParameters", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			lp := s.LivepeerNode.Eth

			numActiveOrchestrators, err := lp.GetTranscoderPoolMaxSize()
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
				NumActiveTranscoders: numActiveOrchestrators,
				RoundLength:          roundLength,
				RoundLockAmount:      roundLockAmount,
				UnbondingPeriod:      unbondingPeriod,
				Inflation:            inflation,
				InflationChange:      inflationChange,
				TargetBondingRate:    targetBondingRate,
				TotalBonded:          totalBonded,
				TotalSupply:          totalSupply,
				Paused:               paused,
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

			balance, err := b.BalanceAt(r.Context(), s.LivepeerNode.Eth.Account().Address, nil)
			if err != nil {
				glog.Error(err)
				w.Write([]byte(""))
			} else {
				w.Write([]byte(balance.String()))
			}
		}
	})

	mux.HandleFunc("/registeredOrchestrators", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			orchestrators, err := s.LivepeerNode.Eth.TranscoderPool()
			if err != nil {
				glog.Error(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			for _, o := range orchestrators {
				dbO, err := s.LivepeerNode.Database.SelectOrchs(&lpcommon.DBOrchFilter{
					Addresses: []common.Address{o.Address},
				})
				if err != nil {
					glog.Errorf("unable to get orchestrators from DB err=%v", err)
					continue
				}
				if len(dbO) == 0 {
					o.PricePerPixel = big.NewRat(0, 1)
					continue
				}
				o.PricePerPixel = lpcommon.FixedToPrice(dbO[0].PricePerPixel)
			}

			data, err := json.Marshal(orchestrators)
			if err != nil {
				glog.Error(err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})

	mux.HandleFunc("/orchestratorInfo", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth != nil {
			t, err := s.LivepeerNode.Eth.GetTranscoder(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Error(err)
				return
			}

			config := struct {
				Transcoder *lpTypes.Transcoder
				PriceInfo  *big.Rat
			}{
				Transcoder: t,
				PriceInfo:  s.LivepeerNode.GetBasePrice(),
			}

			data, err := json.Marshal(config)
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

			nextValidRequest, err := s.LivepeerNode.Eth.NextValidRequest(s.LivepeerNode.Eth.Account().Address)
			if err != nil {
				glog.Errorf("Unable to get the time for the next valid request from faucet: %v", err)
				return
			}
			backend, err := s.LivepeerNode.Eth.Backend()
			if err != nil {
				glog.Errorf("Unable to get LivepeerEthClient backend: %v", err)
				return
			}

			blk, err := backend.BlockByNumber(r.Context(), nil)
			if err != nil {
				glog.Errorf("Unable to get latest block")
				return
			}

			now := int64(blk.Time())
			if nextValidRequest.Int64() != 0 && nextValidRequest.Int64() > now {
				glog.Errorf("Error requesting tokens from faucet: can only request tokens once every hour, please wait %v more minutes", (nextValidRequest.Int64()-now)/60)
				return
			}

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

	mux.HandleFunc("/IsOrchestrator", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("%v", s.LivepeerNode.NodeType == core.OrchestratorNode)))
	})

	mux.HandleFunc("/EthChainID", func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.Eth == nil {
			w.Write([]byte("0"))
			return
		}
		chainID, err := s.LivepeerNode.Database.ChainID()
		if err != nil {
			glog.Errorf("Error getting eth network ID err=%v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(chainID.String()))
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

	mux.Handle("/currentBlock", currentBlockHandler(s.LivepeerNode.Database))

	// TicketBroker

	mux.Handle("/fundDepositAndReserve", mustHaveFormParams(fundDepositAndReserveHandler(s.LivepeerNode.Eth), "depositAmount", "reserveAmount"))
	mux.Handle("/fundDeposit", mustHaveFormParams(fundDepositHandler(s.LivepeerNode.Eth), "amount"))
	mux.Handle("/unlock", unlockHandler(s.LivepeerNode.Eth))
	mux.Handle("/cancelUnlock", cancelUnlockHandler(s.LivepeerNode.Eth))
	mux.Handle("/withdraw", withdrawHandler(s.LivepeerNode.Eth))
	mux.Handle("/senderInfo", senderInfoHandler(s.LivepeerNode.Eth))
	mux.Handle("/ticketBrokerParams", ticketBrokerParamsHandler(s.LivepeerNode.Eth))

	// Metrics
	if monitor.Enabled {
		mux.Handle("/metrics", monitor.Exporter)

	}
	return mux
}

func (s *LivepeerServer) setOrchestratorPriceInfo(pricePerUnitStr, pixelsPerUnitStr string) error {

	pricePerUnit, err := strconv.ParseInt(pricePerUnitStr, 10, 64)
	if err != nil {
		return fmt.Errorf("Error converting pricePerUnit string to int64: %v\n", err)
	}
	if pricePerUnit <= 0 {
		return fmt.Errorf("price unit must be greater than 0, provided %d\n", pricePerUnit)
	}

	pixelsPerUnit, err := strconv.ParseInt(pixelsPerUnitStr, 10, 64)
	if err != nil {
		return fmt.Errorf("Error converting pixelsPerUnit string to int64: %v\n", err)
	}
	if pixelsPerUnit <= 0 {
		return fmt.Errorf("pixels per unit must be greater than 0, provided %d\n", pixelsPerUnit)
	}
	s.LivepeerNode.SetBasePrice(big.NewRat(pricePerUnit, pixelsPerUnit))
	glog.Infof("Price per pixel set to %d wei for %d pixels\n", pricePerUnit, pixelsPerUnit)
	return nil
}
