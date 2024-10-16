package server

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	"github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/livepeer/lpms/ffmpeg"
	"github.com/pkg/errors"
)

const MainnetChainId = 1
const RinkebyChainId = 4

func (s *LivepeerServer) healthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, nil)
	})
}

// Status
func (s *LivepeerServer) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJson(w, s.GetNodeStatus())
	})
}

func (s *LivepeerServer) streamIdHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(s.LastHLSStreamID().String()))
	})
}

func (s *LivepeerServer) manifestIdHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(s.LastManifestID()))
	})
}

func localStreamsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// XXX fetch local streams?
		ret := make([]map[string]string, 0)

		respondJson(w, ret)
	})
}

type ChainIdGetter interface {
	ChainID() (*big.Int, error)
}

func ethChainIdHandler(db ChainIdGetter) http.Handler {
	return mustHaveDb(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID, err := db.ChainID()
		if err != nil {
			respond500(w, fmt.Sprintf("Error getting eth network ID err=%q", err))
			return
		}
		respondOk(w, []byte(chainID.String()))
	}))
}

type BlockGetter interface {
	LastSeenBlock() (*big.Int, error)
}

func currentBlockHandler(db BlockGetter) http.Handler {
	return mustHaveDb(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blk, err := db.LastSeenBlock()
		if err != nil {
			respond500(w, fmt.Sprintf("could not query last seen block: %v", err))
			return
		}
		respondOk(w, blk.Bytes())
	}))
}

type orchInfo struct {
	Transcoder *types.Transcoder
	PriceInfo  *big.Rat
}

func (s *LivepeerServer) orchestratorInfoHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addr := client.Account().Address
		t, err := client.GetTranscoder(addr)
		if err != nil {
			respond500(w, "could not get transcoder")
			return
		}

		info := orchInfo{
			Transcoder: t,
			PriceInfo:  s.LivepeerNode.GetBasePrice("default"),
		}
		respondJson(w, info)
	}))
}

func (s *LivepeerServer) isOrchestratorHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(fmt.Sprintf("%v", s.LivepeerNode.NodeType == core.OrchestratorNode)))
	})
}

func (s *LivepeerServer) isRedeemerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(fmt.Sprintf("%v", s.LivepeerNode.NodeType == core.RedeemerNode)))
	})
}

// Broadcast / Transcoding config
func setBroadcastConfigHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pricePerUnit := r.FormValue("maxPricePerUnit")
		pixelsPerUnit := r.FormValue("pixelsPerUnit")
		currency := r.FormValue("currency")
		transcodingOptions := r.FormValue("transcodingOptions")

		if (pricePerUnit == "" || pixelsPerUnit == "") && transcodingOptions == "" {
			respond400(w, "missing form params (maxPricePerUnit AND pixelsPerUnit) or transcodingOptions")
			return
		}

		// set max price
		if pricePerUnit != "" && pixelsPerUnit != "" {
			pr, ok := new(big.Rat).SetString(pricePerUnit)
			if !ok {
				respond400(w, fmt.Sprintf("Error parsing pricePerUnit value: %s", pricePerUnit))
				return
			}
			px, ok := new(big.Rat).SetString(pixelsPerUnit)
			if !ok {
				respond400(w, fmt.Sprintf("Error parsing pixelsPerUnit value: %s", pixelsPerUnit))
				return
			}
			if px.Sign() <= 0 {
				respond400(w, fmt.Sprintf("pixels per unit must be greater than 0, provided %v", pixelsPerUnit))
				return
			}
			pricePerPixel := new(big.Rat).Quo(pr, px)

			var autoPrice *core.AutoConvertedPrice
			if pricePerPixel.Sign() > 0 {
				var err error
				autoPrice, err = core.NewAutoConvertedPrice(currency, pricePerPixel, func(price *big.Rat) {
					if monitor.Enabled {
						monitor.MaxTranscodingPrice(price)
					}
					glog.Infof("Maximum transcoding price: %v wei per pixel\n", price.FloatString(3))
				})
				if err != nil {
					respond400(w, errors.Wrap(err, "error converting price").Error())
					return
				}
			}

			BroadcastCfg.SetMaxPrice(autoPrice)
		}

		// set broadcast profiles
		if transcodingOptions != "" {
			var profiles []ffmpeg.VideoProfile
			for _, pName := range strings.Split(transcodingOptions, ",") {
				p, ok := ffmpeg.VideoProfileLookup[pName]
				if ok {
					profiles = append(profiles, p)
				}
			}
			if len(profiles) == 0 {
				respond400(w, fmt.Sprintf("invalid transcoding options: %v", transcodingOptions))
				return
			}

			BroadcastJobVideoProfiles = profiles
			glog.Infof("Transcode Job Type: %v", BroadcastJobVideoProfiles)
		}
	})
}

func getBroadcastConfigHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pNames []string
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

		respondJson(w, config)
	})
}

func getAvailableTranscodingOptionsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transcodingOptions := make([]string, 0, len(ffmpeg.VideoProfileLookup))
		for opt := range ffmpeg.VideoProfileLookup {
			transcodingOptions = append(transcodingOptions, opt)
		}

		respondJson(w, transcodingOptions)
	})
}

// Rounds
func currentRoundHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentRound, err := client.CurrentRound()
		if err != nil {
			respond500(w, fmt.Sprintf("could not query current round: %v", err))
			return
		}

		respondOk(w, []byte(currentRound.String()))
	}))
}

func initializeRoundHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.InitializeRound()
		if err != nil {
			glog.Error(err)
			respond500(w, fmt.Sprintf("could not initialize round: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			glog.Error(err)
			respond500(w, fmt.Sprintf("could not initialize round: %v", err))
			return
		}
		respondOk(w, nil)
	}))
}

func roundInitializedHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		initialized, err := client.CurrentRoundInitialized()
		if err != nil {
			glog.Error(err)
			respond500(w, fmt.Sprintf("could not get initialized round: %v", err))
			return
		}
		respondOk(w, []byte(fmt.Sprintf("%v", initialized)))
	}))
}

// Orchestrator registration/activation
func (s *LivepeerServer) activateOrchestratorHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := client.GetTranscoder(client.Account().Address)
		if err != nil {
			glog.Error(err)
			respond500(w, err.Error())
			return
		}

		if t.Status == "Registered" {
			respond400(w, "orchestrator already registered")
			return
		}

		isLocked, err := client.CurrentRoundLocked()
		if err != nil {
			glog.Error(err)
			respond500(w, err.Error())
			return
		}
		if isLocked {
			respond500(w, "current round is locked")
			return
		}

		blockRewardCutStr := r.FormValue("blockRewardCut")
		blockRewardCut, err := strconv.ParseFloat(blockRewardCutStr, 64)
		if err != nil {
			respond400(w, err.Error())
			return
		}

		feeShareStr := r.FormValue("feeShare")
		feeShare, err := strconv.ParseFloat(feeShareStr, 64)
		if err != nil {
			respond400(w, err.Error())
			return
		}

		pricePerUnit, pixelsPerUnit, currency := r.FormValue("pricePerUnit"), r.FormValue("pixelsPerUnit"), r.FormValue("currency")
		if err := s.setOrchestratorPriceInfo("default", pricePerUnit, pixelsPerUnit, currency); err != nil {
			respond400(w, err.Error())
			return
		}

		serviceURI := r.FormValue("serviceURI")
		if _, err := url.ParseRequestURI(serviceURI); err != nil {
			respond400(w, err.Error())
			return
		}

		unbondingLockIDStr := r.FormValue("unbondingLockId")
		if unbondingLockIDStr != "" {
			unbondingLockID, err := common.ParseBigInt(unbondingLockIDStr)
			if err != nil {
				respond400(w, err.Error())
				return
			}

			glog.Infof("Rebonding with unbonding lock %v...", unbondingLockID)

			tx, err := client.RebondFromUnbonded(client.Account().Address, unbondingLockID)
			if err != nil {
				respond500(w, err.Error())
				return
			}

			err = client.CheckTx(tx)
			if err != nil {
				respond500(w, err.Error())
				return
			}
		}

		amountStr := r.FormValue("amount")
		if amountStr != "" {
			amount, err := common.ParseBigInt(amountStr)
			if err != nil {
				respond400(w, err.Error())
				return
			}

			if amount.Cmp(big.NewInt(0)) == 1 {
				glog.Infof("Bonding %v...", amount)

				tx, err := client.Bond(amount, client.Account().Address)
				if err != nil {
					respond500(w, err.Error())
					return
				}

				err = client.CheckTx(tx)
				if err != nil {
					respond500(w, err.Error())
					return
				}
			}
		}

		glog.Infof("Setting orchestrator commission rates %v", client.Account().Address.Hex())

		tx, err := client.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare))
		if err != nil {
			respond500(w, err.Error())
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		currentServiceURI, err := client.GetServiceURI(client.Account().Address)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		if currentServiceURI != serviceURI {
			if err := s.setServiceURI(client, serviceURI); err != nil {
				respond500(w, err.Error())
				return
			}
		}

		respondOk(w, nil)
	}))
}

func (s *LivepeerServer) setOrchestratorConfigHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pixels := r.FormValue("pixelsPerUnit")
		price := r.FormValue("pricePerUnit")
		currency := r.FormValue("currency")
		if pixels != "" && price != "" {
			if err := s.setOrchestratorPriceInfo("default", price, pixels, currency); err != nil {
				respond400(w, err.Error())
				return
			}
		}

		var (
			blockRewardCut float64
			feeShare       float64
			err            error
		)
		blockRewardCutStr := r.FormValue("blockRewardCut")

		if blockRewardCutStr != "" {
			blockRewardCut, err = strconv.ParseFloat(blockRewardCutStr, 64)
			if err != nil {
				err = errors.Wrapf(err, "Cannot convert block reward cut")
				respond400(w, err.Error())
				return
			}
		}

		feeShareStr := r.FormValue("feeShare")
		if feeShareStr != "" {
			feeShare, err = strconv.ParseFloat(feeShareStr, 64)
			if err != nil {
				err = errors.Wrapf(err, "Cannot convert fee share")
				respond400(w, err.Error())
				return
			}
		}

		t, err := client.GetTranscoder(client.Account().Address)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		if feeShareStr != "" && blockRewardCutStr != "" && (t.RewardCut.Cmp(eth.FromPerc(blockRewardCut)) != 0 || t.FeeShare.Cmp(eth.FromPerc(feeShare)) != 0) {
			tx, err := client.Transcoder(eth.FromPerc(blockRewardCut), eth.FromPerc(feeShare))
			if err != nil {
				respond500(w, err.Error())
				return
			}

			glog.Infof("Setting orchestrator commission rates for %v: reward cut=%v feeshare=%v", client.Account().Address.Hex(), blockRewardCut, feeShare)

			err = client.CheckTx(tx)
			if err != nil {
				respond500(w, err.Error())
				return
			}
		}

		serviceURI := r.FormValue("serviceURI")
		if serviceURI != "" {
			if _, err := url.ParseRequestURI(serviceURI); err != nil {
				respond400(w, err.Error())
				return
			}

			if t.ServiceURI != serviceURI {
				if err := s.setServiceURI(client, serviceURI); err != nil {
					respond500(w, err.Error())
					return
				}
			}
		}
		respondOk(w, nil)
	}))
}

func (s *LivepeerServer) setOrchestratorPriceInfo(broadcasterEthAddr, pricePerUnitStr, pixelsPerUnitStr, currency string) error {
	ok, err := regexp.MatchString("^0x[0-9a-fA-F]{40}|default$", broadcasterEthAddr)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("broadcasterEthAddr is not a valid eth address, provided %v", broadcasterEthAddr)
	}

	pricePerUnit, ok := new(big.Rat).SetString(pricePerUnitStr)
	if !ok {
		return fmt.Errorf("error parsing pricePerUnit value: %s", pricePerUnitStr)
	}
	if pricePerUnit.Sign() < 0 {
		return fmt.Errorf("price unit must be greater than or equal to 0, provided %s", pricePerUnitStr)
	}

	pixelsPerUnit, ok := new(big.Rat).SetString(pixelsPerUnitStr)
	if !ok {
		return fmt.Errorf("error parsing pixelsPerUnit value: %v", pixelsPerUnitStr)
	}
	if pixelsPerUnit.Sign() <= 0 {
		return fmt.Errorf("pixels per unit must be greater than 0, provided %s", pixelsPerUnitStr)
	}

	pricePerPixel := new(big.Rat).Quo(pricePerUnit, pixelsPerUnit)
	autoPrice, err := core.NewAutoConvertedPrice(currency, pricePerPixel, func(price *big.Rat) {
		if broadcasterEthAddr == "default" {
			glog.Infof("Price: %v wei per pixel\n ", price.FloatString(3))
		} else {
			glog.Infof("Price: %v wei per pixel for broadcaster %v", price.FloatString(3), broadcasterEthAddr)
		}
	})
	if err != nil {
		return fmt.Errorf("error converting price: %v", err)
	}
	s.LivepeerNode.SetBasePrice(broadcasterEthAddr, autoPrice)

	return nil
}

func (s *LivepeerServer) setServiceURI(client eth.LivepeerEthClient, serviceURI string) error {
	parsedURI, err := url.Parse(serviceURI)
	if err != nil {
		glog.Error(err)
		return err
	}

	if !common.ValidateServiceURI(parsedURI) {
		err = errors.New("service address must be a public IP address or hostname")
		glog.Error(err)
		return err
	}

	glog.Infof("Storing service URI %v in service registry...", serviceURI)

	tx, err := client.SetServiceURI(serviceURI)
	if err != nil {
		glog.Error(err)
		return err
	}

	err = client.CheckTx(tx)
	if err != nil {
		glog.Error(err)
		return err
	}

	// Avoids a restart if only the host has been changed.
	// If the port has been changed, a restart is still needed.
	s.LivepeerNode.SetServiceURI(parsedURI)

	return nil
}

func (s *LivepeerServer) setMaxFaceValueHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.NodeType == core.OrchestratorNode {
			maxfacevalue := r.FormValue("maxfacevalue")
			if maxfacevalue != "" {
				mfv, success := new(big.Int).SetString(maxfacevalue, 10)
				if success {
					s.LivepeerNode.SetMaxFaceValue(mfv)
					respondOk(w, []byte("ticket max face value set"))
					glog.Infof("Max ticket face value set to: %v", maxfacevalue)
				} else {
					respond400(w, "maxfacevalue not set to number")
				}
			} else {
				respond400(w, "need to set 'maxfacevalue'")
			}
		} else {
			respond400(w, "node must be orchestrator node to set maxfacevalue")
		}
	})
}

func (s *LivepeerServer) setPriceForBroadcaster() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.LivepeerNode.NodeType == core.OrchestratorNode {
			pricePerUnitStr := r.FormValue("pricePerUnit")
			pixelsPerUnitStr := r.FormValue("pixelsPerUnit")
			currency := r.FormValue("currency")
			broadcasterEthAddr := r.FormValue("broadcasterEthAddr")

			err := s.setOrchestratorPriceInfo(broadcasterEthAddr, pricePerUnitStr, pixelsPerUnitStr, currency)
			if err == nil {
				respondOk(w, []byte(fmt.Sprintf("Price per pixel set to %s wei for %s pixels for broadcaster %s\n", pricePerUnitStr, pixelsPerUnitStr, broadcasterEthAddr)))
			} else {
				respond400(w, err.Error())
			}
		} else {
			respond400(w, "Node must be orchestrator node to set price for broadcaster")
		}
	})
}

func (s *LivepeerServer) setMaxSessions() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frmMaxSessions := r.FormValue("maxSessions")
		if frmMaxSessions == "auto" {
			s.LivepeerNode.AutoSessionLimit = true
			s.LivepeerNode.SetMaxSessions(s.LivepeerNode.GetCurrentCapacity())
			respondOk(w, []byte("Max sessions set to auto\n"))
		} else if maxSessions, err := strconv.Atoi(frmMaxSessions); err == nil {
			if maxSessions > 0 {
				s.LivepeerNode.AutoSessionLimit = false
				s.LivepeerNode.SetMaxSessions(maxSessions)
				respondOk(w, []byte(fmt.Sprintf("Max sessions set to %d\n", maxSessions)))
			} else {
				respond400(w, "Max Sessions must be 'auto' or greater than zero")
			}
		} else {
			respond400(w, err.Error())
		}
	})
}

// Bond, withdraw, reward
func bondHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amountStr := r.FormValue("amount")
		amount, err := common.ParseBigInt(amountStr)
		if err != nil {
			respond400(w, err.Error())
			return
		}
		toAddr := r.FormValue("toAddr")

		tx, err := client.Bond(amount, ethcommon.HexToAddress(toAddr))
		if err != nil {
			respond400(w, err.Error())
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, nil)
	}))
}

func rebondHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unbondingLockIDStr := r.FormValue("unbondingLockId")
		unbondingLockID, err := common.ParseBigInt(unbondingLockIDStr)
		if err != nil {
			glog.Errorf("Cannot convert unbondingLockId: %v", err)
			return
		}

		var tx *ethtypes.Transaction
		toAddr := r.FormValue("toAddr")
		if toAddr != "" {
			tx, err = client.RebondFromUnbonded(ethcommon.HexToAddress(toAddr), unbondingLockID)
		} else {
			tx, err = client.Rebond(unbondingLockID)
		}
		if err != nil {
			respond400(w, err.Error())
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, nil)
	}))
}

func unbondHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amountStr := r.FormValue("amount")
		amount, err := common.ParseBigInt(amountStr)
		if err != nil {
			respond400(w, err.Error())
			return
		}

		tx, err := client.Unbond(amount)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, nil)
	}))
}

func withdrawStakeHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unbondingLockIDStr := r.FormValue("unbondingLockId")
		unbondingLockID, err := common.ParseBigInt(unbondingLockIDStr)
		if err != nil {
			respond400(w, fmt.Sprintf("Cannot convert unbondingLockId: %v", err))
			return
		}
		tx, err := client.WithdrawStake(unbondingLockID)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, nil)
	}))
}

func unbondingLocksHandler(client eth.LivepeerEthClient, db *common.DB) http.Handler {
	return mustHaveDb(db, mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dAddr := client.Account().Address
		d, err := client.GetDelegator(dAddr)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		// Query for local IDs
		unbondingLockIDs, err := db.UnbondingLockIDs()
		if err != nil {
			respond500(w, err.Error())
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
				lock, err := client.GetDelegatorUnbondingLock(dAddr, id)
				if err != nil {
					glog.Error(err)
					continue
				}
				// If lock has been used (i.e. withdrawRound == 0) do not insert into DB
				// Note: We do not know what block at which a lock was used when querying the contract directly (as opposed to using events)
				// As a result, instead of having a lock entry in the DB with the usedBlock column set, we do not insert a lock entry at all
				if lock.WithdrawRound.Cmp(big.NewInt(0)) == 1 {
					if err := db.InsertUnbondingLock(id, dAddr, lock.Amount, lock.WithdrawRound); err != nil {
						glog.Error(err)
						continue
					}
				}
			}
		}

		var currentRound *big.Int

		withdrawableStr := r.FormValue("withdrawable")
		if withdrawable, err := strconv.ParseBool(withdrawableStr); withdrawable {
			currentRound, err = client.CurrentRound()
			if err != nil {
				glog.Error(err)
				return
			}
		}

		unbondingLocks, err := db.UnbondingLocks(currentRound)
		if err != nil {
			glog.Error(err)
			return
		}

		respondJson(w, unbondingLocks)
	})))
}

func withdrawFeesHandler(client eth.LivepeerEthClient, db ChainIdGetter) http.Handler {
	return mustHaveDb(db, mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// for L1 contracts backwards-compatibility
		var tx *ethtypes.Transaction
		isL1Network, err := isL1Network(db)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		if isL1Network {
			// L1 contracts
			tx, err = client.L1WithdrawFees()
			if err != nil {
				respond500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
				return
			}
		} else {
			// L2 contracts
			amountStr := r.FormValue("amount")
			if amountStr == "" {
				respond400(w, "missing form param: amount")
				return
			}
			amount, err := common.ParseBigInt(amountStr)
			if err != nil {
				respond400(w, fmt.Sprintf("invalid amount: %v", err))
				return
			}

			tx, err = client.WithdrawFees(client.Account().Address, amount)
			if err != nil {
				respond500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
				return
			}
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute WithdrawFees: %v", err))
			return
		}

		respondOk(w, nil)
	})))
}

func claimEarningsHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claim := func() error {
			init, err := client.CurrentRoundInitialized()
			if err != nil {
				glog.Errorf("Trying to claim but round not initialized.")
				return err
			}
			if !init {
				return errors.New("Round not initialized")
			}
			currRound, err := client.CurrentRound()
			if err != nil {
				return err
			}
			tx, err := client.ClaimEarnings(currRound)
			if err != nil {
				return err
			}
			return client.CheckTx(tx)
		}

		if err := backoff.Retry(claim, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second*15), 5)); err != nil {
			respond500(w, fmt.Sprintf("Error claiming earnings: %v", err))
			return
		}
		respondOk(w, nil)
	}))
}

func delegatorInfoHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := client.GetDelegator(client.Account().Address)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		respondJson(w, d)
	}))
}

func orchestratorEarningPoolsForRoundHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roundStr := r.URL.Query().Get("round")
		round, err := common.ParseBigInt(roundStr)
		if err != nil {
			respond400(w, err.Error())
			return
		}

		tp, err := client.GetTranscoderEarningsPoolForRound(client.Account().Address, round)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondJson(w, tp)
	}))
}

func registeredOrchestratorsHandler(client eth.LivepeerEthClient, db *common.DB) http.Handler {
	return mustHaveDb(db, mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orchestrators, err := client.TranscoderPool()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		for _, o := range orchestrators {
			dbO, err := db.SelectOrchs(&common.DBOrchFilter{
				Addresses: []ethcommon.Address{o.Address},
			})
			if err != nil {
				glog.Errorf("unable to get orchestrators from DB err=%q", err)
				continue
			}
			if len(dbO) == 0 {
				o.PricePerPixel = big.NewRat(0, 1)
				continue
			}
			o.PricePerPixel = common.FixedToPrice(dbO[0].PricePerPixel)
		}

		respondJson(w, orchestrators)
	})))
}

func rewardHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		glog.Infof("Calling reward")
		tx, err := client.Reward()
		if err != nil {
			respond500(w, fmt.Sprintf("Error calling reward: %v", err))
			return
		}
		if err := client.CheckTx(tx); err != nil {
			respond500(w, fmt.Sprintf("Error calling reward: %v", err))
			return
		}
		glog.Infof("Call to reward successful")
		respondOk(w, nil)
	}))
}

// Protocol parameters
func protocolParametersHandler(client eth.LivepeerEthClient, db ChainIdGetter) http.Handler {
	return mustHaveDb(db, mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numActiveOrchestrators, err := client.GetTranscoderPoolMaxSize()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		roundLength, err := client.RoundLength()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		roundLockAmount, err := client.RoundLockAmount()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		unbondingPeriod, err := client.UnbondingPeriod()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		inflation, err := client.Inflation()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		inflationChange, err := client.InflationChange()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		targetBondingRate, err := client.TargetBondingRate()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		totalBonded, err := client.GetTotalBonded()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		isL1Network, err := isL1Network(db)
		if err != nil {
			glog.Error(err)
			return
		}
		var totalSupply *big.Int
		if isL1Network {
			totalSupply, err = client.TotalSupply()
		} else {
			totalSupply, err = client.GetGlobalTotalSupply()
		}
		if err != nil {
			respond500(w, err.Error())
			return
		}

		paused, err := client.Paused()
		if err != nil {
			respond500(w, err.Error())
			return
		}

		params := &types.ProtocolParameters{
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

		respondJson(w, params)
	})))
}

// Eth
func contractAddressesHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addrMap := client.ContractAddresses()
		respondJson(w, addrMap)
	}))
}

func ethAddrHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(client.Account().Address.Hex()))
	}))
}

func tokenBalanceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := client.BalanceOf(client.Account().Address)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, []byte(b.String()))
	}))
}

func ethBalanceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := client.Backend()
		balance, err := b.BalanceAt(r.Context(), client.Account().Address, nil)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		respondOk(w, []byte(balance.String()))
	}))
}

func transferTokensHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		to := r.FormValue("to")
		amountStr := r.FormValue("amount")
		amount, err := common.ParseBigInt(amountStr)
		if err != nil {
			respond400(w, err.Error())
			return
		}

		tx, err := client.Transfer(ethcommon.HexToAddress(to), amount)
		if err != nil {
			respond500(w, err.Error())
			return
		}
		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, err.Error())
			return
		}

		glog.Infof("Transferred %v to %v", eth.FormatUnits(amount, "LPT"), to)
		respondOk(w, nil)
	}))
}

func requestTokensHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextValidRequest, err := client.NextValidRequest(client.Account().Address)
		if err != nil {
			respond500(w, fmt.Sprintf("Unable to get the time for the next valid request from faucet: %v", err))
			return
		}

		backend := client.Backend()
		h, err := backend.HeaderByNumber(r.Context(), nil)
		if err != nil {
			respond500(w, fmt.Sprintf("Unable to get latest block: %v", err))
			return
		}

		now := int64(h.Time)
		if nextValidRequest.Int64() != 0 && nextValidRequest.Int64() > now {
			respond500(w, fmt.Sprintf("Error requesting tokens from faucet: can only request tokens once every hour, please wait %v more minutes", (nextValidRequest.Int64()-now)/60))
			return
		}

		glog.Infof("Requesting tokens from faucet")

		tx, err := client.Request()
		if err != nil {
			respond500(w, fmt.Sprintf("Error requesting tokens from faucet: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("Error requesting tokens from faucet: %v", err))
			return
		}
		respondOk(w, nil)
	}))
}

func signMessageHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use EIP-191 (https://github.com/ethereum/EIPs/blob/master/EIPS/eip-191.md) signature versioning
		// The SigFormat terminology is taken from:
		// https://github.com/ethereum/go-ethereum/blob/dddf73abbddb297e61cee6a7e6aebfee87125e49/signer/core/apitypes/types.go#L171
		sigFormat := r.Header.Get("SigFormat")
		if sigFormat == "" {
			sigFormat = "text/plain"
		}

		message := r.FormValue("message")

		var signed []byte
		var err error

		// Only support text/plain and data/typed
		// By default use text/plain
		switch sigFormat {
		case "data/typed":
			var d apitypes.TypedData
			err = json.Unmarshal([]byte(message), &d)
			if err != nil {
				respond400(w, fmt.Sprintf("could not unmarshal typed data - err=%q", err))
				return
			}

			signed, err = client.SignTypedData(d)
			if err != nil {
				respond500(w, fmt.Sprintf("could not sign typed data - err=%q", err))
				return
			}
		default:
			// text/plain
			signed, err = client.Sign([]byte(message))
			if err != nil {
				respond500(w, fmt.Sprintf("could not sign message - err=%q", err))
				return
			}
		}

		respondOk(w, signed)
	}))
}

func voteHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		poll := r.FormValue("poll")
		if !ethcommon.IsHexAddress(poll) {
			respond500(w, "invalid poll contract address")
			return
		}

		choiceStr := r.FormValue("choiceID")
		choiceID, ok := new(big.Int).SetString(choiceStr, 10)
		if !ok {
			respond500(w, "choiceID is not a valid integer value")
			return
		}
		if !types.VoteChoice(int(choiceID.Int64())).IsValid() {
			respond500(w, "invalid choiceID")
			return
		}

		// submit tx
		tx, err := client.Vote(
			ethcommon.HexToAddress(poll),
			choiceID,
		)
		if err != nil {
			respond500(w, fmt.Sprintf("unable to submit vote transaction err=%q", err))
			return
		}

		if err := client.CheckTx(tx); err != nil {
			respond500(w, fmt.Sprintf("unable to mine vote transaction err=%q", err))
			return
		}

		respondOk(w, tx.Hash().Bytes())
	}))
}

// Gas Price
func setMaxGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amount := r.FormValue("amount")
		gprice, err := common.ParseBigInt(amount)
		if err != nil {
			respond400(w, fmt.Sprintf("Parsing failed for price: %v", err))
			return
		}
		if amount == "0" {
			gprice = nil
		}
		client.Backend().GasPriceMonitor().SetMaxGasPrice(gprice)
		if err := client.SetMaxGasPrice(gprice); err != nil {
			glog.Errorf("Error setting max gas price: %v", err)
			respond500(w, err.Error())
		}
		respondOk(w, nil)
	}))
}

func setMinGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		minGasPrice, err := common.ParseBigInt(r.FormValue("minGasPrice"))
		if err != nil {
			respond400(w, fmt.Sprintf("invalid minGasPrice: %v", err))
			return
		}
		client.Backend().GasPriceMonitor().SetMinGasPrice(minGasPrice)

		respondOk(w, nil)
	}))
}

func maxGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := client.Backend()
		gprice := b.GasPriceMonitor().MaxGasPrice()
		if gprice == nil {
			respondOk(w, nil)
		} else {
			respondOk(w, []byte(gprice.String()))
		}
	}))
}

func minGasPriceHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(client.Backend().GasPriceMonitor().MinGasPrice().String()))
	}))
}

// Tickets
func fundDepositAndReserveHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		depositAmount, err := common.ParseBigInt(r.FormValue("depositAmount"))
		if err != nil {
			respond400(w, fmt.Sprintf("invalid depositAmount: %v", err))
			return
		}

		reserveAmount, err := common.ParseBigInt(r.FormValue("reserveAmount"))
		if err != nil {
			respond400(w, fmt.Sprintf("invalid reserveAmount: %v", err))
			return
		}

		tx, err := client.FundDepositAndReserve(depositAmount, reserveAmount)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute fundDepositAndReserve: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute fundDepositAndReserve: %v", err))
			return
		}

		respondOk(w, []byte("fundDepositAndReserve success"))
	}))
}

func fundDepositHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amount, err := common.ParseBigInt(r.FormValue("amount"))
		if err != nil {
			respond400(w, fmt.Sprintf("invalid amount: %v", err))
			return
		}

		tx, err := client.FundDeposit(amount)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute fundDeposit: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute fundDeposit: %v", err))
			return
		}

		respondOk(w, []byte("fundDeposit success"))
	}))
}

func unlockHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.Unlock()
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute unlock: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute unlock: %v", err))
			return
		}

		respondOk(w, []byte("unlock success"))
	}))
}

func cancelUnlockHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.CancelUnlock()
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute cancelUnlock: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute cancelUnlock: %v", err))
			return
		}

		respondOk(w, []byte("cancelUnlock success"))
	}))
}

func withdrawHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := client.Withdraw()
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute withdraw: %v", err))
			return
		}

		err = client.CheckTx(tx)
		if err != nil {
			respond500(w, fmt.Sprintf("could not execute withdraw: %v", err))
			return
		}

		respondOk(w, []byte("withdraw success"))
	}))
}

func senderInfoHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := client.GetSenderInfo(client.Account().Address)
		if err != nil {
			if err.Error() == "ErrNoResult" {
				info = &pm.SenderInfo{
					Deposit:       big.NewInt(0),
					WithdrawRound: big.NewInt(0),
					Reserve: &pm.ReserveInfo{
						FundsRemaining:        big.NewInt(0),
						ClaimedInCurrentRound: big.NewInt(0),
					},
				}
			} else {
				respond500(w, fmt.Sprintf("could not query sender info: %v", err))
				return
			}
		}

		respondJson(w, info)
	}))
}

func ticketBrokerParamsHandler(client eth.LivepeerEthClient) http.Handler {
	return mustHaveClient(client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unlockPeriod, err := client.UnlockPeriod()
		if err != nil {
			respond500(w, fmt.Sprintf("could not query TicketBroker unlockPeriod: %v", err))
			return
		}

		params := struct {
			UnlockPeriod *big.Int
		}{
			unlockPeriod,
		}

		respondJson(w, params)
	}))
}

// Debug, Log Level
func setLogLevelHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vFlag == nil {
			respond500(w, "nil log level")
			return
		}
		err := vFlag.Set(r.FormValue("loglevel"))
		if err != nil {
			respond400(w, "parameter 'logLevel' not defined")
			return
		}
		respondOk(w, nil)
	})
}

func getLogLevelHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vFlag == nil {
			respond500(w, "nil log level")
			return
		}
		respondOk(w, []byte(vFlag.String()))
	})
}

func (s *LivepeerServer) debugHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondOk(w, []byte(fmt.Sprintf("\n\nLatestPlaylist: %v", s.LatestPlaylist())))
	})
}

func isL1Network(db ChainIdGetter) (bool, error) {
	chainId, err := db.ChainID()
	if err != nil {
		return false, err
	}
	return chainId.Int64() == MainnetChainId || chainId.Int64() == RinkebyChainId, err
}

// Helpers
func respondOk(w http.ResponseWriter, msg []byte) {
	w.WriteHeader(http.StatusOK)
	if msg != nil {
		if _, err := w.Write(msg); err != nil {
			glog.Error(err)
		}
	}
}

func respondJson(w http.ResponseWriter, v interface{}) {
	if v == nil {
		respond500(w, "unknown internal server error")
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		respond500(w, err.Error())
	}
	respondJsonOk(w, data)
}

func respondJsonOk(w http.ResponseWriter, msg []byte) {
	w.Header().Set("Content-Type", "application/json")
	respondOk(w, msg)
}

func respond500(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusInternalServerError)
}

func respond400(w http.ResponseWriter, errMsg string) {
	respondWithError(w, errMsg, http.StatusBadRequest)
}

func respondWithError(w http.ResponseWriter, errMsg string, code int) {
	glog.Errorf("HTTP Response Error statusCode=%d err=%v", code, errMsg)
	http.Error(w, errMsg, code)
}

func mustHaveFormParams(h http.Handler, params ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			respond500(w, fmt.Sprintf("parse form error: %v", err))
			return
		}

		for _, param := range params {
			if r.FormValue(param) == "" {
				respond400(w, fmt.Sprintf("missing form param: %s", param))
				return
			}
		}

		h.ServeHTTP(w, r)
	})
}

func mustHaveClient(client eth.LivepeerEthClient, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			respond500(w, "missing ETH client")
			return
		}
		h.ServeHTTP(w, r)
	})
}

func mustHaveDb(db interface{}, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			respond500(w, "missing database")
			return
		}
		h.ServeHTTP(w, r)
	})
}
