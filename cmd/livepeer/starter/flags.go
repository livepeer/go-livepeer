package starter

import (
	"flag"
	"strings"

	"github.com/golang/glog"
)

func NewLivepeerConfig(fs *flag.FlagSet) LivepeerConfig {
	cfg := DefaultLivepeerConfig()

	// Network & Addresses:
	// Role annotations: [O]=Orchestrator [B]=Gateway/Broadcaster [T]=Transcoder [W]=AI Worker [ALL]=All roles
	cfg.Network = fs.String("network", *cfg.Network, "[ALL] Network to connect to")
	cfg.RtmpAddr = fs.String("rtmpAddr", *cfg.RtmpAddr, "[B] Address to bind for RTMP commands")
	cfg.CliAddr = fs.String("cliAddr", *cfg.CliAddr, "[ALL] Address to bind for CLI commands")
	cfg.HttpAddr = fs.String("httpAddr", *cfg.HttpAddr, "[ALL] Address to bind for HTTP commands")
	cfg.ServiceAddr = fs.String("serviceAddr", *cfg.ServiceAddr, "[O] Overrides the on-chain serviceURI that gateways can use to contact this node; may be an IP or hostname")
	cfg.Nodes = fs.String("nodes", *cfg.Nodes, "[O] Comma-separated list of instance URLs for this orchestrator")
	cfg.VerifierURL = fs.String("verifierUrl", *cfg.VerifierURL, "[B] URL of the verifier to use")
	cfg.VerifierPath = fs.String("verifierPath", *cfg.VerifierPath, "[B] Path to verifier shared volume")
	cfg.LocalVerify = fs.Bool("localVerify", *cfg.LocalVerify, "[B] Set to true to enable local verification i.e. pixel count and signature verification")
	cfg.HttpIngest = fs.Bool("httpIngest", *cfg.HttpIngest, "[B] Set to true to enable HTTP ingest")

	// Broadcaster/Gateway's Selection Algorithm
	cfg.OrchAddr = fs.String("orchAddr", *cfg.OrchAddr, "[B] Comma-separated list of orchestrators to connect to")
	cfg.OrchWebhookURL = fs.String("orchWebhookUrl", *cfg.OrchWebhookURL, "[B] Orchestrator discovery callback URL")
	cfg.ExtraNodes = fs.Int("extraNodes", *cfg.ExtraNodes, "[O] Number of extra nodes an orchestrator can advertise within the GetOrchestratorInfo response")
	cfg.OrchBlacklist = fs.String("orchBlocklist", "", "[B] Comma-separated list of blocklisted orchestrators")
	cfg.OrchMinLivepeerVersion = fs.String("orchMinLivepeerVersion", *cfg.OrchMinLivepeerVersion, "[B] Minimal go-livepeer version orchestrator should have to be selected")
	cfg.SelectRandWeight = fs.Float64("selectRandFreq", *cfg.SelectRandWeight, "[B] Weight of the random factor in the orchestrator selection algorithm")
	cfg.SelectStakeWeight = fs.Float64("selectStakeWeight", *cfg.SelectStakeWeight, "[B] Weight of the stake factor in the orchestrator selection algorithm")
	cfg.SelectPriceWeight = fs.Float64("selectPriceWeight", *cfg.SelectPriceWeight, "[B] Weight of the price factor in the orchestrator selection algorithm")
	cfg.SelectPriceExpFactor = fs.Float64("selectPriceExpFactor", *cfg.SelectPriceExpFactor, "[B] Expresses how significant a small change of price is for the selection algorithm; default 100")
	cfg.OrchPerfStatsURL = fs.String("orchPerfStatsUrl", *cfg.OrchPerfStatsURL, "[B] URL of Orchestrator Performance Stream Tester")
	cfg.Region = fs.String("region", *cfg.Region, "[B] Region in which a gateway is deployed; used to select the region while using the orchestrator's performance stats")
	cfg.MaxPricePerUnit = fs.String("maxPricePerUnit", *cfg.MaxPricePerUnit, "[B] The maximum transcoding price per 'pixelsPerUnit' a gateway is willing to accept. If not set explicitly, gateway is willing to accept ANY price. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	cfg.MaxPricePerCapability = fs.String("maxPricePerCapability", *cfg.MaxPricePerCapability, `[B] json list of prices per capability/model or path to json config file. Use "model_id": "default" to price all models in a pipeline the same. Example: {"capabilities_prices": [{"pipeline": "text-to-image", "model_id": "stabilityai/sd-turbo", "price_per_unit": 1000, "pixels_per_unit": 1}, {"pipeline": "upscale", "model_id": "default", price_per_unit": 1200, "pixels_per_unit": 1}]}`)
	cfg.IgnoreMaxPriceIfNeeded = fs.Bool("ignoreMaxPriceIfNeeded", *cfg.IgnoreMaxPriceIfNeeded, "[B] Set to true to allow exceeding max price condition if there is no O that meets this requirement")
	cfg.MinPerfScore = fs.Float64("minPerfScore", *cfg.MinPerfScore, "[B] The minimum orchestrator's performance score a gateway is willing to accept")
	cfg.DiscoveryTimeout = fs.Duration("discoveryTimeout", *cfg.DiscoveryTimeout, "[B] Time to wait for orchestrators to return info to be included in transcoding sessions for manifest (default = 500ms)")
	cfg.GatewayHost = fs.String("gatewayHost", *cfg.GatewayHost, "[B] External hostname on which the Gateway node is running. Used when telling external services how to reach the node.")

	// Transcoding:
	cfg.Orchestrator = fs.Bool("orchestrator", *cfg.Orchestrator, "[ALL] Set to true to be an orchestrator")
	cfg.Transcoder = fs.Bool("transcoder", *cfg.Transcoder, "[ALL] Set to true to be a transcoder")
	cfg.Gateway = fs.Bool("gateway", *cfg.Broadcaster, "[ALL] Set to true to be a gateway")
	cfg.Broadcaster = fs.Bool("broadcaster", *cfg.Broadcaster, "[ALL] Set to true to be a broadcaster (Deprecated, use -gateway)")
	cfg.OrchSecret = fs.String("orchSecret", *cfg.OrchSecret, "[T] Shared secret with the orchestrator as a standalone transcoder or path to file")
	cfg.TranscodingOptions = fs.String("transcodingOptions", *cfg.TranscodingOptions, "[B] Transcoding options for broadcast job, or path to json config")
	cfg.MaxAttempts = fs.Int("maxAttempts", *cfg.MaxAttempts, "[B] Maximum transcode attempts")
	cfg.MaxSessions = fs.String("maxSessions", *cfg.MaxSessions, "[O,B,T] Maximum number of concurrent transcoding sessions for Orchestrator or 'auto' for dynamic limit, maximum number of RTMP streams for Broadcaster, or maximum capacity for transcoder")
	cfg.CurrentManifest = fs.Bool("currentManifest", *cfg.CurrentManifest, "[B] Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	cfg.Nvidia = fs.String("nvidia", *cfg.Nvidia, "[T] Comma-separated list of Nvidia GPU device IDs (or \"all\" for all available devices)")
	cfg.Netint = fs.String("netint", *cfg.Netint, "[T] Comma-separated list of NetInt device GUIDs (or \"all\" for all available devices)")
	cfg.TestTranscoder = fs.Bool("testTranscoder", *cfg.TestTranscoder, "[T] Test Nvidia GPU transcoding at startup")
	cfg.HevcDecoding = fs.Bool("hevcDecoding", *cfg.HevcDecoding, "[T] Enable or disable HEVC decoding")

	// AI:
	cfg.AIServiceRegistry = fs.Bool("aiServiceRegistry", *cfg.AIServiceRegistry, "[B] Set to true to use an AI ServiceRegistry contract address")
	cfg.AIWorker = fs.Bool("aiWorker", *cfg.AIWorker, "[ALL] Set to true to run an AI worker")
	cfg.AIModels = fs.String("aiModels", *cfg.AIModels, "[W] Set models (pipeline:model_id) for AI worker to load upon initialization")
	cfg.AIModelsDir = fs.String("aiModelsDir", *cfg.AIModelsDir, "[W] Set directory where AI model weights are stored")
	cfg.AIRunnerImage = fs.String("aiRunnerImage", *cfg.AIRunnerImage, "[W] [Deprecated] Specify the base Docker image for the AI runner. Example: livepeer/ai-runner:0.0.1. Use -aiRunnerImageOverrides instead.")
	cfg.AIVerboseLogs = fs.Bool("aiVerboseLogs", *cfg.AIVerboseLogs, "[W] Set to true to enable verbose logs for the AI runner containers created by the worker")
	cfg.AIRunnerImageOverrides = fs.String("aiRunnerImageOverrides", *cfg.AIRunnerImageOverrides, `[W] Specify overrides for the Docker images used by the AI runner. Example: '{"default": "livepeer/ai-runner:v1.0", "batch": {"text-to-speech": "livepeer/ai-runner:text-to-speech-v1.0"}, "live": {"another-pipeline": "livepeer/ai-runner:another-pipeline-v1.0"}}'`)
	cfg.AIProcessingRetryTimeout = fs.Duration("aiProcessingRetryTimeout", *cfg.AIProcessingRetryTimeout, "[W] Timeout for retrying to initiate AI processing request")
	cfg.AIRunnerContainersPerGPU = fs.Int("aiRunnerContainersPerGPU", *cfg.AIRunnerContainersPerGPU, "[W] Number of AI runner containers to run per GPU; default to 1")
	cfg.AIMinRunnerVersion = fs.String("aiMinRunnerVersion", *cfg.AIMinRunnerVersion, `[O] JSON specifying the min runner versions for each pipeline. It works ONLY for warm runner containers, SHOULD NOT be used for cold runners. Example: '[{"model_id": "noop", "pipeline": "live-video-to-video", "minVersion": "0.0.2"}]'; if not set, the runner's min version is used"`)

	// Live AI:
	cfg.MediaMTXApiPassword = fs.String("mediaMTXApiPassword", "", "[O,W] HTTP basic auth password for MediaMTX API requests")
	cfg.LiveAITrickleHostForRunner = fs.String("liveAITrickleHostForRunner", "", "[O,W] Trickle Host used by AI Runner; It's used to overwrite the publicly available Trickle Host")
	cfg.LiveAIAuthApiKey = fs.String("liveAIAuthApiKey", "", "[O,W] API key to use for Live AI authentication requests")
	cfg.LiveAIHeartbeatURL = fs.String("liveAIHeartbeatURL", "", "[O,W] Base URL for Live AI heartbeat requests")
	cfg.LiveAIHeartbeatHeaders = fs.String("liveAIHeartbeatHeaders", "", "[O,W] Map of headers to use for Live AI heartbeat requests. e.g. 'header:val,header2:val2'")
	cfg.LiveAIHeartbeatInterval = fs.Duration("liveAIHeartbeatInterval", *cfg.LiveAIHeartbeatInterval, "[O,W] Interval to send Live AI heartbeat requests")
	cfg.LiveAIAuthWebhookURL = fs.String("liveAIAuthWebhookUrl", "", "[O,W] Live AI RTMP authentication webhook URL")
	cfg.LivePaymentInterval = fs.Duration("livePaymentInterval", *cfg.LivePaymentInterval, "[B,O] Interval to pay process Gateway <> Orchestrator Payments for Live AI Video")
	cfg.LiveOutSegmentTimeout = fs.Duration("liveOutSegmentTimeout", *cfg.LiveOutSegmentTimeout, "[O,W] Timeout duration to wait the output segment to be available in the Live AI pipeline; defaults to no timeout")
	cfg.LiveAISaveNSegments = fs.Int("liveAISaveNSegments", 10, "[O,W] Set how many segments to save to disk for debugging (both input and output)")
	cfg.LiveAICapRefreshModels = fs.String("liveAICapRefreshModels", "", "[O,W] [Deprecated] Capacity is now available for all models, use -liveAICapReportInterval to set the interval for reporting capacity metrics")
	cfg.LiveAICapReportInterval = fs.Duration("liveAICapReportInterval", *cfg.LiveAICapReportInterval, "[O,W] Interval to report Live AI container capacity metrics, e.g. 10s, 1m, 1h. defaults to 25 minutes")

	// Onchain:
	cfg.EthAcctAddr = fs.String("ethAcctAddr", *cfg.EthAcctAddr, "[ALL] Existing Eth account address. For use when multiple ETH accounts exist in the keystore directory")
	cfg.EthPassword = fs.String("ethPassword", *cfg.EthPassword, "[ALL] Password for existing Eth account address or path to file")
	cfg.EthKeystorePath = fs.String("ethKeystorePath", *cfg.EthKeystorePath, "[ALL] Path to ETH keystore directory or keyfile. If keyfile, overrides -ethAcctAddr and uses parent directory")
	cfg.EthOrchAddr = fs.String("ethOrchAddr", *cfg.EthOrchAddr, "[T] ETH address of an on-chain registered orchestrator")
	cfg.EthUrl = fs.String("ethUrl", *cfg.EthUrl, "[ALL] Ethereum node JSON-RPC URL")
	cfg.TxTimeout = fs.Duration("transactionTimeout", *cfg.TxTimeout, "[ALL] Amount of time to wait for an Ethereum transaction to confirm before timing out")
	cfg.MaxTxReplacements = fs.Int("maxTransactionReplacements", *cfg.MaxTxReplacements, "[ALL] Number of times to automatically replace pending Ethereum transactions")
	cfg.GasLimit = fs.Int("gasLimit", *cfg.GasLimit, "[ALL] Gas limit for ETH transactions")
	cfg.MinGasPrice = fs.Int64("minGasPrice", 0, "[ALL] Minimum gas price (priority fee + base fee) for ETH transactions in wei, 10 Gwei = 10000000000")
	cfg.MaxGasPrice = fs.Int("maxGasPrice", *cfg.MaxGasPrice, "[ALL] Maximum gas price (priority fee + base fee) for ETH transactions in wei, 40 Gwei = 40000000000")
	cfg.EthController = fs.String("ethController", *cfg.EthController, "[ALL] Protocol smart contract address")
	cfg.InitializeRound = fs.Bool("initializeRound", *cfg.InitializeRound, "[O,T] Set to true if running as a transcoder and the node should automatically initialize new rounds")
	cfg.InitializeRoundMaxDelay = fs.Duration("initializeRoundMaxDelay", *cfg.InitializeRoundMaxDelay, "[O,T] Maximum delay to wait before initializing a round")
	cfg.TicketEV = fs.String("ticketEV", *cfg.TicketEV, "[O] The expected value for PM tickets")
	cfg.MaxFaceValue = fs.String("maxFaceValue", *cfg.MaxFaceValue, "[O] Set max ticket face value in WEI")
	cfg.MaxTicketEV = fs.String("maxTicketEV", *cfg.MaxTicketEV, "[B] The maximum acceptable expected value for one PM ticket")
	cfg.MaxTotalEV = fs.String("maxTotalEV", *cfg.MaxTotalEV, "[B] The maximum acceptable expected value for one PM payment")
	cfg.DepositMultiplier = fs.Int("depositMultiplier", *cfg.DepositMultiplier, "[B] The deposit multiplier used to determine max acceptable faceValue for PM tickets")
	cfg.PricePerUnit = fs.String("pricePerUnit", "0", "[O] The price per 'pixelsPerUnit' amount pixels. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	cfg.PixelsPerUnit = fs.String("pixelsPerUnit", *cfg.PixelsPerUnit, "[O,B] Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")
	cfg.PriceFeedAddr = fs.String("priceFeedAddr", *cfg.PriceFeedAddr, "[O,B] ETH address of the Chainlink price feed contract. Used for custom currencies conversion on -pricePerUnit or -maxPricePerUnit")
	cfg.AutoAdjustPrice = fs.Bool("autoAdjustPrice", *cfg.AutoAdjustPrice, "[O] Enable/disable automatic price adjustments based on the overhead for redeeming tickets")
	cfg.PricePerGateway = fs.String("pricePerGateway", *cfg.PricePerGateway, `[O] json list of price per gateway or path to json config file. Example: {"gateways":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	cfg.PricePerBroadcaster = fs.String("pricePerBroadcaster", *cfg.PricePerBroadcaster, `[O] json list of price per broadcaster or path to json config file. Example: {"broadcasters":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	cfg.BlockPollingInterval = fs.Int("blockPollingInterval", *cfg.BlockPollingInterval, "[ALL] Interval in seconds at which different blockchain event services poll for blocks")
	cfg.Redeemer = fs.Bool("redeemer", *cfg.Redeemer, "[ALL] Set to true to run a ticket redemption service")
	cfg.RedeemerAddr = fs.String("redeemerAddr", *cfg.RedeemerAddr, "[O] URL of the ticket redemption service to use")
	cfg.Reward = fs.Bool("reward", false, "[O] Set to true to run a reward service")
	// Metrics & logging:
	cfg.Monitor = fs.Bool("monitor", *cfg.Monitor, "[ALL] Set to true to send performance metrics")
	cfg.MetricsPerStream = fs.Bool("metricsPerStream", *cfg.MetricsPerStream, "[ALL] Set to true to group performance metrics per stream")
	cfg.MetricsExposeClientIP = fs.Bool("metricsClientIP", *cfg.MetricsExposeClientIP, "[ALL] Set to true to expose client's IP in metrics")
	cfg.MetadataQueueUri = fs.String("metadataQueueUri", *cfg.MetadataQueueUri, "[ALL] URI for message broker to send operation metadata")
	cfg.MetadataAmqpExchange = fs.String("metadataAmqpExchange", *cfg.MetadataAmqpExchange, "[ALL] Name of AMQP exchange to send operation metadata")
	cfg.MetadataPublishTimeout = fs.Duration("metadataPublishTimeout", *cfg.MetadataPublishTimeout, "[ALL] Max time to wait in background for publishing operation metadata events")

	// Storage:
	fs.StringVar(cfg.Datadir, "datadir", *cfg.Datadir, "[ALL] [Deprecated] Directory that data is stored in")
	fs.StringVar(cfg.Datadir, "dataDir", *cfg.Datadir, "[ALL] Directory that data is stored in")
	cfg.Objectstore = fs.String("objectStore", *cfg.Objectstore, "[ALL] URL of primary object store")
	cfg.Recordstore = fs.String("recordStore", *cfg.Recordstore, "[B] URL of object store for recordings")

	// Fast Verification GS bucket:
	cfg.FVfailGsBucket = fs.String("FVfailGsbucket", *cfg.FVfailGsBucket, "[B] Google Cloud Storage bucket for storing segments, which failed fast verification")
	cfg.FVfailGsKey = fs.String("FVfailGskey", *cfg.FVfailGsKey, "[B] Google Cloud Storage private key file name or key in JSON format for accessing FVfailGsBucket")
	// API
	cfg.AuthWebhookURL = fs.String("authWebhookUrl", *cfg.AuthWebhookURL, "[B] RTMP authentication webhook URL")

	// Flags
	cfg.TestOrchAvail = fs.Bool("startupAvailabilityCheck", *cfg.TestOrchAvail, "[O] Set to false to disable the startup Orchestrator availability check on the configured serviceAddr")
	cfg.RemoteSigner = fs.Bool("remoteSigner", *cfg.RemoteSigner, "[ALL] Set to true to run remote signer service")
	cfg.RemoteSignerUrl = fs.String("remoteSignerUrl", *cfg.RemoteSignerUrl, "[B] URL of remote signer service to use (e.g., http://localhost:8935). Gateway only.")
	cfg.RemoteDiscovery = fs.Bool("remoteDiscovery", *cfg.RemoteDiscovery, "[B] Enable orchestrator discovery on remote signers")

	// Gateway metrics
	cfg.KafkaBootstrapServers = fs.String("kafkaBootstrapServers", *cfg.KafkaBootstrapServers, "[B] URL of Kafka Bootstrap Servers")
	cfg.KafkaUsername = fs.String("kafkaUser", *cfg.KafkaUsername, "[B] Kafka Username")
	cfg.KafkaPassword = fs.String("kafkaPassword", *cfg.KafkaPassword, "[B] Kafka Password")
	cfg.KafkaGatewayTopic = fs.String("kafkaGatewayTopic", *cfg.KafkaGatewayTopic, "[B] Kafka Topic used to send gateway logs")

	return cfg
}

// UpdateNilsForUnsetFlags changes some cfg fields to nil if they were not explicitly set with flags.
// For some flags, the behavior is different whether the value is default or not set by the user at all.
func UpdateNilsForUnsetFlags(cfg LivepeerConfig) LivepeerConfig {
	res := cfg

	isFlagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { isFlagSet[f.Name] = true })

	if !isFlagSet["minGasPrice"] {
		res.MinGasPrice = nil
	}
	if !isFlagSet["pricePerUnit"] {
		res.PricePerUnit = nil
	}
	if !isFlagSet["reward"] {
		res.Reward = nil
	}
	if !isFlagSet["httpIngest"] {
		res.HttpIngest = nil
	}
	if !isFlagSet["localVerify"] {
		res.LocalVerify = nil
	}
	if !isFlagSet["hevcDecoding"] {
		res.HevcDecoding = nil
	}

	return res
}

// flagRoleMap defines which roles each flag applies to.
// Roles: O=Orchestrator, B=Gateway/Broadcaster, T=Transcoder, W=AI Worker
// Flags not in this map are assumed to apply to all roles.
var flagRoleMap = map[string]string{
	// Network
	"rtmpAddr":     "B",
	"serviceAddr":  "O",
	"nodes":        "O",
	"verifierUrl":  "B",
	"verifierPath": "B",
	"localVerify":  "B",
	"httpIngest":   "B",
	// Gateway selection
	"orchAddr": "B", "orchWebhookUrl": "B", "orchBlocklist": "B",
	"orchMinLivepeerVersion": "B", "selectRandFreq": "B", "selectStakeWeight": "B",
	"selectPriceWeight": "B", "selectPriceExpFactor": "B", "orchPerfStatsUrl": "B",
	"region": "B", "maxPricePerUnit": "B", "maxPricePerCapability": "B",
	"ignoreMaxPriceIfNeeded": "B", "minPerfScore": "B", "discoveryTimeout": "B",
	"gatewayHost": "B",
	// Orchestrator
	"extraNodes": "O", "ticketEV": "O", "maxFaceValue": "O",
	"pricePerUnit": "O", "autoAdjustPrice": "O",
	"pricePerGateway": "O", "pricePerBroadcaster": "O",
	"redeemerAddr": "O", "reward": "O",
	"startupAvailabilityCheck": "O",
	// Transcoder
	"orchSecret": "T", "nvidia": "T", "netint": "T",
	"testTranscoder": "T", "hevcDecoding": "T", "ethOrchAddr": "T",
	// Gateway/Broadcaster
	"transcodingOptions": "B", "maxAttempts": "B", "currentManifest": "B",
	"maxTicketEV": "B", "maxTotalEV": "B", "depositMultiplier": "B",
	"recordStore": "B", "FVfailGsbucket": "B", "FVfailGskey": "B",
	"authWebhookUrl": "B", "remoteSignerUrl": "B", "remoteDiscovery": "B",
	"kafkaBootstrapServers": "B", "kafkaUser": "B", "kafkaPassword": "B", "kafkaGatewayTopic": "B",
	// AI Worker
	"aiModels": "W", "aiModelsDir": "W", "aiRunnerImage": "W",
	"aiVerboseLogs": "W", "aiRunnerImageOverrides": "W",
	"aiProcessingRetryTimeout": "W", "aiRunnerContainersPerGPU": "W",
	// AI + Orchestrator
	"aiServiceRegistry":  "B",
	"aiMinRunnerVersion": "O",
	// Combined roles
	"maxSessions": "O,B,T", "pixelsPerUnit": "O,B", "priceFeedAddr": "O,B",
	"initializeRound": "O,T", "initializeRoundMaxDelay": "O,T",
	"livePaymentInterval": "B,O",
}

// WarnMismatchedFlags logs warnings for flags that don't apply to the current node role.
func WarnMismatchedFlags(cfg LivepeerConfig) {
	role := ""
	if cfg.Orchestrator != nil && *cfg.Orchestrator {
		role = "O"
	} else if cfg.Gateway != nil && *cfg.Gateway {
		role = "B"
	} else if cfg.Broadcaster != nil && *cfg.Broadcaster {
		role = "B"
	} else if cfg.Transcoder != nil && *cfg.Transcoder {
		role = "T"
	} else if cfg.AIWorker != nil && *cfg.AIWorker {
		role = "W"
	}
	if role == "" {
		return
	}

	roleNames := map[string]string{
		"O": "orchestrator", "B": "gateway/broadcaster",
		"T": "transcoder", "W": "AI worker",
	}

	flag.Visit(func(f *flag.Flag) {
		roles, exists := flagRoleMap[f.Name]
		if !exists {
			return // flag applies to all roles
		}
		if !strings.Contains(roles, role) {
			glog.Warningf("Flag -%s is intended for %s roles but this node is running as %s; it may have no effect",
				f.Name, roles, roleNames[role])
		}
	})
}
