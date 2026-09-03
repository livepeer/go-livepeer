package server

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"

	// pprof adds handlers to default mux via `init()`
	_ "net/http/pprof"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/runner"
	"github.com/livepeer/go-livepeer/monitor"
)

var vFlag *glog.Level = flag.Lookup("v").Value.(*glog.Level)

// StartCliWebserver starts web server for CLI
// blocks until exit
func (s *LivepeerServer) StartCliWebserver(srv *http.Server) {
	srv.Handler = s.cliWebServerHandlers(srv.Addr)
	glog.Info("CLI server listening on ", srv.Addr)
	err := srv.ListenAndServe()
	glog.Error(err)
}

func (s *LivepeerServer) setOnChainConfig() {}

func (s *LivepeerServer) cliWebServerHandlers(bindAddr string) *http.ServeMux {
	// Override default mux because pprof only uses the default mux
	// We really don't want to accidentally pull pprof into other listeners.
	// Pprof, like the CLI, is a strictly private API!
	mux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	client := s.LivepeerNode.Eth
	db := s.LivepeerNode.Database
	if s.CliTxRoutes {
		s.registerCliTxRoutes(mux)
	}

	// Status
	mux.Handle("/status", s.statusHandler())
	mux.Handle("/streamID", s.streamIdHandler())
	mux.Handle("/manifestID", s.manifestIdHandler())
	mux.Handle("/localStreams", localStreamsHandler())
	mux.Handle("/EthChainID", ethChainIdHandler(db))
	mux.Handle("/currentBlock", currentBlockHandler(db))
	mux.Handle("/orchestratorInfo", s.orchestratorInfoHandler(client))
	mux.Handle("/IsOrchestrator", s.isOrchestratorHandler())
	mux.Handle("/IsRedeemer", s.isRedeemerHandler())

	// Broadcast / Transcoding config
	mux.Handle("POST /setBroadcastConfig", mustHaveFormParams(setBroadcastConfigHandler()))
	mux.Handle("/getBroadcastConfig", getBroadcastConfigHandler())
	mux.Handle("/getAvailableTranscodingOptions", getAvailableTranscodingOptionsHandler())
	mux.Handle("POST /setMaxPriceForCapability", mustHaveFormParams(s.setMaxPriceForCapability(), "maxPricePerUnit", "pixelsPerUnit", "currency", "pipeline", "modelID"))
	mux.Handle("/getAISessionPoolsInfo", s.getAIPoolsInfoHandler())
	mux.Handle("/getNetworkCapabilities", s.getNetworkCapabilitiesHandler())
	mux.Handle("POST /registerLiveRunners", s.registerLiveRunnersHandler())

	// Rounds
	mux.Handle("/currentRound", currentRoundHandler(client))
	mux.Handle("/roundInitialized", roundInitializedHandler(client))

	// Orchestrator registration/activation
	mux.Handle("POST /setMaxFaceValue", mustHaveFormParams(s.setMaxFaceValueHandler(), "maxfacevalue"))
	mux.Handle("POST /setPriceForBroadcaster", mustHaveFormParams(s.setPriceForBroadcaster(), "pricePerUnit", "pixelsPerUnit", "broadcasterEthAddr"))
	mux.Handle("POST /setMaxSessions", mustHaveFormParams(s.setMaxSessions(), "maxSessions"))

	// Bond, withdraw, reward
	mux.Handle("/unbondingLocks", mustHaveFormParams(unbondingLocksHandler(client, db)))
	mux.Handle("/delegatorInfo", delegatorInfoHandler(client))
	mux.Handle("/orchestratorEarningPoolsForRound", orchestratorEarningPoolsForRoundHandler(client))
	mux.Handle("/registeredOrchestrators", registeredOrchestratorsHandler(client, db))
	mux.Handle("/rewardCaller", s.rewardCallerHandler(client))

	// Protocol parameters
	mux.Handle("/protocolParameters", protocolParametersHandler(client, db))

	// Eth
	mux.Handle("/contractAddresses", contractAddressesHandler(client))
	mux.Handle("/ethAddr", ethAddrHandler(client))
	mux.Handle("/tokenBalance", tokenBalanceHandler(client))
	mux.Handle("/ethBalance", ethBalanceHandler(client))

	// Gas Price
	mux.Handle("/maxGasPrice", maxGasPriceHandler(client))
	mux.Handle("/minGasPrice", minGasPriceHandler(client))

	// Tickets
	mux.Handle("/senderInfo", senderInfoHandler(client))
	mux.Handle("/ticketBrokerParams", ticketBrokerParamsHandler(client))

	// Debug, Log Level
	mux.Handle("POST /setLogLevel", mustHaveFormParams(setLogLevelHandler(), "loglevel"))
	mux.Handle("/getLogLevel", getLogLevelHandler())
	mux.Handle("/debug", s.debugHandler())

	// Metrics
	if monitor.Enabled {
		mux.Handle("/metrics", monitor.Exporter)
	}

	return mux
}

func (s *LivepeerServer) registerCliTxRoutes(mux *http.ServeMux) {
	client := s.LivepeerNode.Eth
	db := s.LivepeerNode.Database

	// Rounds and orchestrator registration
	mux.Handle("POST /initializeRound", initializeRoundHandler(client))
	mux.Handle("POST /activateOrchestrator", s.mustBeOrchestratorAccount(client, mustHaveFormParams(s.activateOrchestratorHandler(client), "blockRewardCut", "feeShare", "pricePerUnit", "pixelsPerUnit", "serviceURI")))
	mux.Handle("POST /setOrchestratorConfig", s.mustBeOrchestratorAccount(client, mustHaveFormParams(s.setOrchestratorConfigHandler(client))))

	// Bonding, withdrawals, and rewards
	mux.Handle("POST /bond", mustHaveFormParams(bondHandler(client), "amount", "toAddr"))
	mux.Handle("POST /rebond", mustHaveFormParams(rebondHandler(client), "unbondingLockId"))
	mux.Handle("POST /unbond", mustHaveFormParams(unbondHandler(client), "amount"))
	mux.Handle("POST /withdrawStake", mustHaveFormParams(withdrawStakeHandler(client), "unbondingLockId"))
	mux.Handle("POST /withdrawFees", withdrawFeesHandler(client, db))
	mux.Handle("POST /claimEarnings", claimEarningsHandler(client))
	mux.Handle("POST /reward", s.rewardHandler(client))
	mux.Handle("POST /setRewardCaller", s.mustBeOrchestratorAccount(client, mustHaveFormParams(s.setRewardCallerHandler(client))))

	// Wallet and governance operations
	mux.Handle("POST /transferTokens", mustHaveFormParams(transferTokensHandler(client), "to", "amount"))
	mux.Handle("POST /requestTokens", requestTokensHandler(client))
	mux.Handle("POST /signMessage", mustHaveFormParams(signMessageHandler(client), "message"))
	mux.Handle("POST /vote", mustHaveFormParams(voteHandler(client), "poll", "choiceID"))
	mux.Handle("POST /voteOnProposal", mustHaveFormParams(proposalVoteHandler(client), "proposalID", "support"))

	// Transaction gas controls
	mux.Handle("POST /setMaxGasPrice", mustHaveFormParams(setMaxGasPriceHandler(client), "amount"))
	mux.Handle("POST /setMinGasPrice", mustHaveFormParams(setMinGasPriceHandler(client), "minGasPrice"))

	// Ticket broker transactions
	mux.Handle("POST /fundDepositAndReserve", mustHaveFormParams(fundDepositAndReserveHandler(client), "depositAmount", "reserveAmount"))
	mux.Handle("POST /fundDeposit", mustHaveFormParams(fundDepositHandler(client), "amount"))
	mux.Handle("POST /unlock", unlockHandler(client))
	mux.Handle("POST /cancelUnlock", cancelUnlockHandler(client))
	mux.Handle("POST /withdraw", withdrawHandler(client))
}

func (s *LivepeerServer) registerLiveRunnersHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager, ok := s.LivepeerNode.LiveRunnerManager.(interface {
			RegisterStaticRunnersJSON([]byte) (*runner.StaticLiveRunnerRegistrationResponse, error)
		})
		if !ok {
			http.Error(w, "live runners are not supported", http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := manager.RegisterStaticRunnersJSON(body)
		if err != nil {
			statusCode := http.StatusBadRequest
			if runnerErr, ok := err.(*runner.RunnerError); ok {
				statusCode = runnerErr.StatusCode
			}
			http.Error(w, err.Error(), statusCode)
			return
		}
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
}
