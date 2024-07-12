package server

import (
	"flag"
	"net/http"

	// pprof adds handlers to default mux via `init()`
	_ "net/http/pprof"

	"github.com/golang/glog"
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
	mux.Handle("/setBroadcastConfig", mustHaveFormParams(setBroadcastConfigHandler()))
	mux.Handle("/getBroadcastConfig", getBroadcastConfigHandler())
	mux.Handle("/getAvailableTranscodingOptions", getAvailableTranscodingOptionsHandler())

	// Rounds
	mux.Handle("/currentRound", currentRoundHandler(client))
	mux.Handle("/initializeRound", initializeRoundHandler(client))
	mux.Handle("/roundInitialized", roundInitializedHandler(client))

	// Orchestrator registration/activation
	mux.Handle("/activateOrchestrator", mustHaveFormParams(s.activateOrchestratorHandler(client), "blockRewardCut", "feeShare", "pricePerUnit", "pixelsPerUnit", "serviceURI"))
	mux.Handle("/setOrchestratorConfig", mustHaveFormParams(s.setOrchestratorConfigHandler(client)))
	mux.Handle("/setMaxFaceValue", mustHaveFormParams(s.setMaxFaceValueHandler(), "maxfacevalue"))
	mux.Handle("/setPriceForBroadcaster", mustHaveFormParams(s.setPriceForBroadcaster(), "pricePerUnit", "pixelsPerUnit", "broadcasterEthAddr"))
	mux.Handle("/setMaxSessions", mustHaveFormParams(s.setMaxSessions(), "maxSessions"))

	// Bond, withdraw, reward
	mux.Handle("/bond", mustHaveFormParams(bondHandler(client), "amount", "toAddr"))
	mux.Handle("/rebond", mustHaveFormParams(rebondHandler(client), "unbondingLockId"))
	mux.Handle("/unbond", mustHaveFormParams(unbondHandler(client), "amount"))
	mux.Handle("/withdrawStake", mustHaveFormParams(withdrawStakeHandler(client), "unbondingLockId"))
	mux.Handle("/unbondingLocks", mustHaveFormParams(unbondingLocksHandler(client, db)))
	mux.Handle("/withdrawFees", withdrawFeesHandler(client, db))
	mux.Handle("/claimEarnings", claimEarningsHandler(client))
	mux.Handle("/delegatorInfo", delegatorInfoHandler(client))
	mux.Handle("/orchestratorEarningPoolsForRound", orchestratorEarningPoolsForRoundHandler(client))
	mux.Handle("/registeredOrchestrators", registeredOrchestratorsHandler(client, db))
	mux.Handle("/reward", rewardHandler(client))

	// Protocol parameters
	mux.Handle("/protocolParameters", protocolParametersHandler(client, db))

	// Eth
	mux.Handle("/contractAddresses", contractAddressesHandler(client))
	mux.Handle("/ethAddr", ethAddrHandler(client))
	mux.Handle("/tokenBalance", tokenBalanceHandler(client))
	mux.Handle("/ethBalance", ethBalanceHandler(client))
	mux.Handle("/transferTokens", mustHaveFormParams(transferTokensHandler(client), "to", "amount"))
	mux.Handle("/requestTokens", requestTokensHandler(client))
	mux.Handle("/signMessage", mustHaveFormParams(signMessageHandler(client), "message"))
	mux.Handle("/vote", mustHaveFormParams(voteHandler(client), "poll", "choiceID"))

	// Gas Price
	mux.Handle("/setMaxGasPrice", mustHaveFormParams(setMaxGasPriceHandler(client), "amount"))
	mux.Handle("/setMinGasPrice", mustHaveFormParams(setMinGasPriceHandler(client), "minGasPrice"))
	mux.Handle("/maxGasPrice", maxGasPriceHandler(client))
	mux.Handle("/minGasPrice", minGasPriceHandler(client))

	// Tickets
	mux.Handle("/fundDepositAndReserve", mustHaveFormParams(fundDepositAndReserveHandler(client), "depositAmount", "reserveAmount"))
	mux.Handle("/fundDeposit", mustHaveFormParams(fundDepositHandler(client), "amount"))
	mux.Handle("/unlock", unlockHandler(client))
	mux.Handle("/cancelUnlock", cancelUnlockHandler(client))
	mux.Handle("/withdraw", withdrawHandler(client))
	mux.Handle("/senderInfo", senderInfoHandler(client))
	mux.Handle("/ticketBrokerParams", ticketBrokerParamsHandler(client))

	// Debug, Log Level
	mux.Handle("/setLogLevel", mustHaveFormParams(setLogLevelHandler(), "loglevel"))
	mux.Handle("/getLogLevel", getLogLevelHandler())
	mux.Handle("/debug", s.debugHandler())

	// Metrics
	if monitor.Enabled {
		mux.Handle("/metrics", monitor.Exporter)
	}

	return mux
}
