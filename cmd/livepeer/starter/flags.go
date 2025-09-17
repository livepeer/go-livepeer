package starter

import (
	"flag"
)

func NewLivepeerConfig(fs *flag.FlagSet) LivepeerConfig {
	cfg := DefaultLivepeerConfig()

	// Network & Addresses:
	cfg.Network = fs.String("network", *cfg.Network, "Network to connect to")
	cfg.RtmpAddr = fs.String("rtmpAddr", *cfg.RtmpAddr, "Address to bind for RTMP commands")
	cfg.CliAddr = fs.String("cliAddr", *cfg.CliAddr, "Address to bind for  CLI commands")
	cfg.HttpAddr = fs.String("httpAddr", *cfg.HttpAddr, "Address to bind for HTTP commands")
	cfg.ServiceAddr = fs.String("serviceAddr", *cfg.ServiceAddr, "Orchestrator only. Overrides the on-chain serviceURI that broadcasters can use to contact this node; may be an IP or hostname.")
	cfg.Nodes = fs.String("nodes", *cfg.Nodes, "Comma-separated list of instance URLs for this orchestrator")
	cfg.VerifierURL = fs.String("verifierUrl", *cfg.VerifierURL, "URL of the verifier to use")
	cfg.VerifierPath = fs.String("verifierPath", *cfg.VerifierPath, "Path to verifier shared volume")
	cfg.LocalVerify = fs.Bool("localVerify", *cfg.LocalVerify, "Set to true to enable local verification i.e. pixel count and signature verification.")
	cfg.HttpIngest = fs.Bool("httpIngest", *cfg.HttpIngest, "Set to true to enable HTTP ingest")

	// Broadcaster's Selection Algorithm
	cfg.OrchAddr = fs.String("orchAddr", *cfg.OrchAddr, "Comma-separated list of orchestrators to connect to")
	cfg.OrchWebhookURL = fs.String("orchWebhookUrl", *cfg.OrchWebhookURL, "Orchestrator discovery callback URL")
	cfg.ExtraNodes = fs.Int("extraNodes", *cfg.ExtraNodes, "Number of extra nodes an orchestrator can advertise within the GetOrchestratorInfo response")
	cfg.OrchBlacklist = fs.String("orchBlocklist", "", "Comma-separated list of blocklisted orchestrators")
	cfg.OrchMinLivepeerVersion = fs.String("orchMinLivepeerVersion", *cfg.OrchMinLivepeerVersion, "Minimal go-livepeer version orchestrator should have to be selected")
	cfg.SelectRandWeight = fs.Float64("selectRandFreq", *cfg.SelectRandWeight, "Weight of the random factor in the orchestrator selection algorithm")
	cfg.SelectStakeWeight = fs.Float64("selectStakeWeight", *cfg.SelectStakeWeight, "Weight of the stake factor in the orchestrator selection algorithm")
	cfg.SelectPriceWeight = fs.Float64("selectPriceWeight", *cfg.SelectPriceWeight, "Weight of the price factor in the orchestrator selection algorithm")
	cfg.SelectPriceExpFactor = fs.Float64("selectPriceExpFactor", *cfg.SelectPriceExpFactor, "Expresses how significant a small change of price is for the selection algorithm; default 100")
	cfg.OrchPerfStatsURL = fs.String("orchPerfStatsUrl", *cfg.OrchPerfStatsURL, "URL of Orchestrator Performance Stream Tester")
	cfg.Region = fs.String("region", *cfg.Region, "Region in which a broadcaster is deployed; used to select the region while using the orchestrator's performance stats")
	cfg.MaxPricePerUnit = fs.String("maxPricePerUnit", *cfg.MaxPricePerUnit, "The maximum transcoding price per 'pixelsPerUnit' a broadcaster is willing to accept. If not set explicitly, broadcaster is willing to accept ANY price. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	cfg.MaxPricePerCapability = fs.String("maxPricePerCapability", *cfg.MaxPricePerCapability, `json list of prices per capability/model or path to json config file. Use "model_id": "default" to price all models in a pipeline the same. Example: {"capabilities_prices": [{"pipeline": "text-to-image", "model_id": "stabilityai/sd-turbo", "price_per_unit": 1000, "pixels_per_unit": 1}, {"pipeline": "upscale", "model_id": "default", price_per_unit": 1200, "pixels_per_unit": 1}]}`)
	cfg.IgnoreMaxPriceIfNeeded = fs.Bool("ignoreMaxPriceIfNeeded", *cfg.IgnoreMaxPriceIfNeeded, "Set to true to allow exceeding max price condition if there is no O that meets this requirement")
	cfg.MinPerfScore = fs.Float64("minPerfScore", *cfg.MinPerfScore, "The minimum orchestrator's performance score a broadcaster is willing to accept")
	cfg.DiscoveryTimeout = fs.Duration("discoveryTimeout", *cfg.DiscoveryTimeout, "Time to wait for orchestrators to return info to be included in transcoding sessions for manifest (default = 500ms)")
	cfg.GatewayHost = fs.String("gatewayHost", *cfg.GatewayHost, "External hostname on which the Gateway node is running. Used when telling external services how to reach the node.")

	// Transcoding:
	cfg.Orchestrator = fs.Bool("orchestrator", *cfg.Orchestrator, "Set to true to be an orchestrator")
	cfg.Transcoder = fs.Bool("transcoder", *cfg.Transcoder, "Set to true to be a transcoder")
	cfg.Gateway = fs.Bool("gateway", *cfg.Broadcaster, "Set to true to be a gateway")
	cfg.Broadcaster = fs.Bool("broadcaster", *cfg.Broadcaster, "Set to true to be a broadcaster (**Deprecated**, use -gateway)")
	cfg.OrchSecret = fs.String("orchSecret", *cfg.OrchSecret, "Shared secret with the orchestrator as a standalone transcoder or path to file")
	cfg.TranscodingOptions = fs.String("transcodingOptions", *cfg.TranscodingOptions, "Transcoding options for broadcast job, or path to json config")
	cfg.MaxAttempts = fs.Int("maxAttempts", *cfg.MaxAttempts, "Maximum transcode attempts")
	cfg.MaxSessions = fs.String("maxSessions", *cfg.MaxSessions, "Maximum number of concurrent transcoding sessions for Orchestrator or 'auto' for dynamic limit, maximum number of RTMP streams for Broadcaster, or maximum capacity for transcoder.")
	cfg.CurrentManifest = fs.Bool("currentManifest", *cfg.CurrentManifest, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	cfg.Nvidia = fs.String("nvidia", *cfg.Nvidia, "Comma-separated list of Nvidia GPU device IDs (or \"all\" for all available devices)")
	cfg.Netint = fs.String("netint", *cfg.Netint, "Comma-separated list of NetInt device GUIDs (or \"all\" for all available devices)")
	cfg.TestTranscoder = fs.Bool("testTranscoder", *cfg.TestTranscoder, "Test Nvidia GPU transcoding at startup")
	cfg.HevcDecoding = fs.Bool("hevcDecoding", *cfg.HevcDecoding, "Enable or disable HEVC decoding")

	// AI:
	cfg.AIServiceRegistry = fs.Bool("aiServiceRegistry", *cfg.AIServiceRegistry, "Set to true to use an AI ServiceRegistry contract address")
	cfg.AIWorker = fs.Bool("aiWorker", *cfg.AIWorker, "Set to true to run an AI worker")
	cfg.AIModels = fs.String("aiModels", *cfg.AIModels, "Set models (pipeline:model_id) for AI worker to load upon initialization")
	cfg.AIModelsDir = fs.String("aiModelsDir", *cfg.AIModelsDir, "Set directory where AI model weights are stored")
	cfg.AIRunnerImage = fs.String("aiRunnerImage", *cfg.AIRunnerImage, "[Deprecated] Specify the base Docker image for the AI runner. Example: livepeer/ai-runner:0.0.1. Use -aiRunnerImageOverrides instead.")
	cfg.AIVerboseLogs = fs.Bool("aiVerboseLogs", *cfg.AIVerboseLogs, "Set to true to enable verbose logs for the AI runner containers created by the worker")
	cfg.AIRunnerImageOverrides = fs.String("aiRunnerImageOverrides", *cfg.AIRunnerImageOverrides, `Specify overrides for the Docker images used by the AI runner. Example: '{"default": "livepeer/ai-runner:v1.0", "batch": {"text-to-speech": "livepeer/ai-runner:text-to-speech-v1.0"}, "live": {"another-pipeline": "livepeer/ai-runner:another-pipeline-v1.0"}}'`)
	cfg.AIProcessingRetryTimeout = fs.Duration("aiProcessingRetryTimeout", *cfg.AIProcessingRetryTimeout, "Timeout for retrying to initiate AI processing request")
	cfg.AIRunnerContainersPerGPU = fs.Int("aiRunnerContainersPerGPU", *cfg.AIRunnerContainersPerGPU, "Number of AI runner containers to run per GPU; default to 1")
	cfg.AIMinRunnerVersion = fs.String("aiMinRunnerVersion", *cfg.AIMinRunnerVersion, `JSON specifying the min runner versions for each pipeline. It works ONLY for warm runner containers, SHOULD NOT be used for cold runners. Example: '[{"model_id": "noop", "pipeline": "live-video-to-video", "minVersion": "0.0.2"}]'; if not set, the runner's min version is used"`)

	// Live AI:
	cfg.MediaMTXApiPassword = fs.String("mediaMTXApiPassword", "", "HTTP basic auth password for MediaMTX API requests")
	cfg.LiveAITrickleHostForRunner = fs.String("liveAITrickleHostForRunner", "", "Trickle Host used by AI Runner; It's used to overwrite the publicly available Trickle Host")
	cfg.LiveAIAuthApiKey = fs.String("liveAIAuthApiKey", "", "API key to use for Live AI authentication requests")
	cfg.LiveAIHeartbeatURL = fs.String("liveAIHeartbeatURL", "", "Base URL for Live AI heartbeat requests")
	cfg.LiveAIHeartbeatHeaders = fs.String("liveAIHeartbeatHeaders", "", "Map of headers to use for Live AI heartbeat requests. e.g. 'header:val,header2:val2'")
	cfg.LiveAIHeartbeatInterval = fs.Duration("liveAIHeartbeatInterval", *cfg.LiveAIHeartbeatInterval, "Interval to send Live AI heartbeat requests")
	cfg.LiveAIAuthWebhookURL = fs.String("liveAIAuthWebhookUrl", "", "Live AI RTMP authentication webhook URL")
	cfg.LivePaymentInterval = fs.Duration("livePaymentInterval", *cfg.LivePaymentInterval, "Interval to pay process Gateway <> Orchestrator Payments for Live AI Video")
	cfg.LiveOutSegmentTimeout = fs.Duration("liveOutSegmentTimeout", *cfg.LiveOutSegmentTimeout, "Timeout duration to wait the output segment to be available in the Live AI pipeline; defaults to no timeout")
	cfg.LiveAICapRefreshModels = fs.String("liveAICapRefreshModels", "", "Comma separated list of models to periodically fetch capacity for. Leave unset to switch off periodic refresh.")
	cfg.LiveAISaveNSegments = fs.Int("liveAISaveNSegments", 10, "Set how many segments to save to disk for debugging (both input and output)")

	// Onchain:
	cfg.EthAcctAddr = fs.String("ethAcctAddr", *cfg.EthAcctAddr, "Existing Eth account address. For use when multiple ETH accounts exist in the keystore directory")
	cfg.EthPassword = fs.String("ethPassword", *cfg.EthPassword, "Password for existing Eth account address or path to file")
	cfg.EthKeystorePath = fs.String("ethKeystorePath", *cfg.EthKeystorePath, "Path to ETH keystore directory or keyfile. If keyfile, overrides -ethAcctAddr and uses parent directory")
	cfg.EthOrchAddr = fs.String("ethOrchAddr", *cfg.EthOrchAddr, "ETH address of an on-chain registered orchestrator")
	cfg.EthUrl = fs.String("ethUrl", *cfg.EthUrl, "Ethereum node JSON-RPC URL")
	cfg.TxTimeout = fs.Duration("transactionTimeout", *cfg.TxTimeout, "Amount of time to wait for an Ethereum transaction to confirm before timing out")
	cfg.MaxTxReplacements = fs.Int("maxTransactionReplacements", *cfg.MaxTxReplacements, "Number of times to automatically replace pending Ethereum transactions")
	cfg.GasLimit = fs.Int("gasLimit", *cfg.GasLimit, "Gas limit for ETH transactions")
	cfg.MinGasPrice = fs.Int64("minGasPrice", 0, "Minimum gas price (priority fee + base fee) for ETH transactions in wei, 10 Gwei = 10000000000")
	cfg.MaxGasPrice = fs.Int("maxGasPrice", *cfg.MaxGasPrice, "Maximum gas price (priority fee + base fee) for ETH transactions in wei, 40 Gwei = 40000000000")
	cfg.EthController = fs.String("ethController", *cfg.EthController, "Protocol smart contract address")
	cfg.InitializeRound = fs.Bool("initializeRound", *cfg.InitializeRound, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	cfg.InitializeRoundMaxDelay = fs.Duration("initializeRoundMaxDelay", *cfg.InitializeRoundMaxDelay, "Maximum delay to wait before initializing a round")
	cfg.TicketEV = fs.String("ticketEV", *cfg.TicketEV, "The expected value for PM tickets")
	cfg.MaxFaceValue = fs.String("maxFaceValue", *cfg.MaxFaceValue, "set max ticket face value in WEI")
	// Broadcaster max acceptable ticket EV
	cfg.MaxTicketEV = fs.String("maxTicketEV", *cfg.MaxTicketEV, "The maximum acceptable expected value for one PM ticket")
	// Broadcaster max acceptable total EV for one payment
	cfg.MaxTotalEV = fs.String("maxTotalEV", *cfg.MaxTotalEV, "The maximum acceptable expected value for one PM payment")
	// Broadcaster deposit multiplier to determine max acceptable ticket faceValue
	cfg.DepositMultiplier = fs.Int("depositMultiplier", *cfg.DepositMultiplier, "The deposit multiplier used to determine max acceptable faceValue for PM tickets")
	// Orchestrator base pricing info
	cfg.PricePerUnit = fs.String("pricePerUnit", "0", "The price per 'pixelsPerUnit' amount pixels. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	// Unit of pixels for both O's pricePerUnit and B's maxPricePerUnit
	cfg.PixelsPerUnit = fs.String("pixelsPerUnit", *cfg.PixelsPerUnit, "Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")
	cfg.PriceFeedAddr = fs.String("priceFeedAddr", *cfg.PriceFeedAddr, "ETH address of the Chainlink price feed contract. Used for custom currencies conversion on -pricePerUnit or -maxPricePerUnit")
	cfg.AutoAdjustPrice = fs.Bool("autoAdjustPrice", *cfg.AutoAdjustPrice, "Enable/disable automatic price adjustments based on the overhead for redeeming tickets")
	cfg.PricePerGateway = fs.String("pricePerGateway", *cfg.PricePerGateway, `json list of price per gateway or path to json config file. Example: {"gateways":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	cfg.PricePerBroadcaster = fs.String("pricePerBroadcaster", *cfg.PricePerBroadcaster, `json list of price per broadcaster or path to json config file. Example: {"broadcasters":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	// Interval to poll for blocks
	cfg.BlockPollingInterval = fs.Int("blockPollingInterval", *cfg.BlockPollingInterval, "Interval in seconds at which different blockchain event services poll for blocks")
	// Redemption service
	cfg.Redeemer = fs.Bool("redeemer", *cfg.Redeemer, "Set to true to run a ticket redemption service")
	cfg.RedeemerAddr = fs.String("redeemerAddr", *cfg.RedeemerAddr, "URL of the ticket redemption service to use")
	// Reward service
	cfg.Reward = fs.Bool("reward", false, "Set to true to run a reward service")
	// Metrics & logging:
	cfg.Monitor = fs.Bool("monitor", *cfg.Monitor, "Set to true to send performance metrics")
	cfg.MetricsPerStream = fs.Bool("metricsPerStream", *cfg.MetricsPerStream, "Set to true to group performance metrics per stream")
	cfg.MetricsExposeClientIP = fs.Bool("metricsClientIP", *cfg.MetricsExposeClientIP, "Set to true to expose client's IP in metrics")
	cfg.MetadataQueueUri = fs.String("metadataQueueUri", *cfg.MetadataQueueUri, "URI for message broker to send operation metadata")
	cfg.MetadataAmqpExchange = fs.String("metadataAmqpExchange", *cfg.MetadataAmqpExchange, "Name of AMQP exchange to send operation metadata")
	cfg.MetadataPublishTimeout = fs.Duration("metadataPublishTimeout", *cfg.MetadataPublishTimeout, "Max time to wait in background for publishing operation metadata events")

	// Storage:
	fs.StringVar(cfg.Datadir, "datadir", *cfg.Datadir, "[Deprecated] Directory that data is stored in")
	fs.StringVar(cfg.Datadir, "dataDir", *cfg.Datadir, "Directory that data is stored in")
	cfg.Objectstore = fs.String("objectStore", *cfg.Objectstore, "url of primary object store")
	cfg.Recordstore = fs.String("recordStore", *cfg.Recordstore, "url of object store for recordings")

	// Fast Verification GS bucket:
	cfg.FVfailGsBucket = fs.String("FVfailGsbucket", *cfg.FVfailGsBucket, "Google Cloud Storage bucket for storing segments, which failed fast verification")
	cfg.FVfailGsKey = fs.String("FVfailGskey", *cfg.FVfailGsKey, "Google Cloud Storage private key file name or key in JSON format for accessing FVfailGsBucket")
	// API
	cfg.AuthWebhookURL = fs.String("authWebhookUrl", *cfg.AuthWebhookURL, "RTMP authentication webhook URL")

	// flags
	cfg.TestOrchAvail = fs.Bool("startupAvailabilityCheck", *cfg.TestOrchAvail, "Set to false to disable the startup Orchestrator availability check on the configured serviceAddr")

	// Gateway metrics
	cfg.KafkaBootstrapServers = fs.String("kafkaBootstrapServers", *cfg.KafkaBootstrapServers, "URL of Kafka Bootstrap Servers")
	cfg.KafkaUsername = fs.String("kafkaUser", *cfg.KafkaUsername, "Kafka Username")
	cfg.KafkaPassword = fs.String("kafkaPassword", *cfg.KafkaPassword, "Kafka Password")
	cfg.KafkaGatewayTopic = fs.String("kafkaGatewayTopic", *cfg.KafkaGatewayTopic, "Kafka Topic used to send gateway logs")

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
