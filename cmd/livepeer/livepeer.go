/*
Livepeer is a peer-to-peer global video live streaming network.  The Golp project is a go implementation of the Livepeer protocol.  For more information, visit the project wiki.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/livepeer/go-livepeer/cmd/livepeer/starter"
	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	"github.com/peterbourgon/ff/v3"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
)

func main() {
	// Override the default flag set since there are dependencies that
	// incorrectly add their own flags (specifically, due to the 'testing'
	// package being linked)
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	//We preserve this flag before resetting all the flags.  Not a scalable approach, but it'll do for now.  More discussions here - https://github.com/livepeer/go-livepeer/pull/617
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.CommandLine.SetOutput(os.Stdout)

	// Help & Log
	mistJSON := flag.Bool("j", false, "Print application info as json")
	version := flag.Bool("version", false, "Print out the version")
	verbosity := flag.String("v", "3", "Log verbosity.  {4|5|6}")

	cfg := parseLivepeerConfig()

	// Config file
	_ = flag.String("config", "", "Config file in the format 'key value', flags and env vars take precedence over the config file")
	err := ff.Parse(flag.CommandLine, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithEnvVarPrefix("LP"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	if err != nil {
		glog.Exit("Error parsing config: ", err)
	}

	vFlag.Value.Set(*verbosity)

	if *mistJSON {
		mistconnector.PrintMistConfigJson(
			"livepeer",
			"Official implementation of the Livepeer video processing protocol. Can play all roles in the network.",
			"Livepeer",
			core.LivepeerVersion,
			flag.CommandLine,
		)
		return
	}

	cfg = updateNilsForUnsetFlags(cfg)

	// compare current settings with default values, and print the difference
	defCfg := starter.DefaultLivepeerConfig()
	vDefCfg := reflect.ValueOf(defCfg)
	vCfg := reflect.ValueOf(cfg)
	cfgType := vCfg.Type()
	paramTable := tablewriter.NewWriter(os.Stdout)
	for i := 0; i < cfgType.NumField(); i++ {
		if !vDefCfg.Field(i).IsNil() && !vCfg.Field(i).IsNil() && vCfg.Field(i).Elem().Interface() != vDefCfg.Field(i).Elem().Interface() {
			paramTable.Append([]string{cfgType.Field(i).Name, fmt.Sprintf("%v", vCfg.Field(i).Elem())})
		}
	}
	paramTable.SetAlignment(tablewriter.ALIGN_LEFT)
	paramTable.SetCenterSeparator("*")
	paramTable.SetColumnSeparator("|")
	paramTable.Render()

	if *version {
		fmt.Println("Livepeer Node Version: " + core.LivepeerVersion)
		fmt.Printf("Golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Operating system: %s\n", runtime.GOOS)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	lc := make(chan struct{})

	go func() {
		starter.StartLivepeer(ctx, cfg)
		lc <- struct{}{}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	select {
	case sig := <-c:
		glog.Infof("Exiting Livepeer: %v", sig)
		cancel()
		time.Sleep(time.Millisecond * 500) //Give time for other processes to shut down completely
	case <-lc:
	}
}

func parseLivepeerConfig() starter.LivepeerConfig {
	cfg := starter.DefaultLivepeerConfig()

	// Network & Addresses:
	cfg.Network = flag.String("network", *cfg.Network, "Network to connect to")
	cfg.RtmpAddr = flag.String("rtmpAddr", *cfg.RtmpAddr, "Address to bind for RTMP commands")
	cfg.CliAddr = flag.String("cliAddr", *cfg.CliAddr, "Address to bind for  CLI commands")
	cfg.HttpAddr = flag.String("httpAddr", *cfg.HttpAddr, "Address to bind for HTTP commands")
	cfg.ServiceAddr = flag.String("serviceAddr", *cfg.ServiceAddr, "Orchestrator only. Overrides the on-chain serviceURI that broadcasters can use to contact this node; may be an IP or hostname.")
	cfg.VerifierURL = flag.String("verifierUrl", *cfg.VerifierURL, "URL of the verifier to use")
	cfg.VerifierPath = flag.String("verifierPath", *cfg.VerifierPath, "Path to verifier shared volume")
	cfg.LocalVerify = flag.Bool("localVerify", *cfg.LocalVerify, "Set to true to enable local verification i.e. pixel count and signature verification.")
	cfg.HttpIngest = flag.Bool("httpIngest", *cfg.HttpIngest, "Set to true to enable HTTP ingest")

	// Broadcaster's Selection Algorithm
	cfg.OrchAddr = flag.String("orchAddr", *cfg.OrchAddr, "Comma-separated list of orchestrators to connect to")
	cfg.OrchWebhookURL = flag.String("orchWebhookUrl", *cfg.OrchWebhookURL, "Orchestrator discovery callback URL")
	cfg.OrchBlacklist = flag.String("orchBlocklist", "", "Comma-separated list of blocklisted orchestrators")
	cfg.OrchMinLivepeerVersion = flag.String("orchMinLivepeerVersion", *cfg.OrchMinLivepeerVersion, "Minimal go-livepeer version orchestrator should have to be selected")
	cfg.SelectRandWeight = flag.Float64("selectRandFreq", *cfg.SelectRandWeight, "Weight of the random factor in the orchestrator selection algorithm")
	cfg.SelectStakeWeight = flag.Float64("selectStakeWeight", *cfg.SelectStakeWeight, "Weight of the stake factor in the orchestrator selection algorithm")
	cfg.SelectPriceWeight = flag.Float64("selectPriceWeight", *cfg.SelectPriceWeight, "Weight of the price factor in the orchestrator selection algorithm")
	cfg.SelectPriceExpFactor = flag.Float64("selectPriceExpFactor", *cfg.SelectPriceExpFactor, "Expresses how significant a small change of price is for the selection algorithm; default 100")
	cfg.OrchPerfStatsURL = flag.String("orchPerfStatsUrl", *cfg.OrchPerfStatsURL, "URL of Orchestrator Performance Stream Tester")
	cfg.Region = flag.String("region", *cfg.Region, "Region in which a broadcaster is deployed; used to select the region while using the orchestrator's performance stats")
	cfg.MaxPricePerUnit = flag.String("maxPricePerUnit", *cfg.MaxPricePerUnit, "The maximum transcoding price per 'pixelsPerUnit' a broadcaster is willing to accept. If not set explicitly, broadcaster is willing to accept ANY price. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	cfg.IgnoreMaxPriceIfNeeded = flag.Bool("ignoreMaxPriceIfNeeded", *cfg.IgnoreMaxPriceIfNeeded, "Set to true to allow exceeding max price condition if there is no O that meets this requirement")
	cfg.MinPerfScore = flag.Float64("minPerfScore", *cfg.MinPerfScore, "The minimum orchestrator's performance score a broadcaster is willing to accept")
	cfg.DiscoveryTimeout = flag.Duration("discoveryTimeout", *cfg.DiscoveryTimeout, "Time to wait for orchestrators to return info to be included in transcoding sessions for manifest (default = 500ms)")

	// Transcoding:
	cfg.Orchestrator = flag.Bool("orchestrator", *cfg.Orchestrator, "Set to true to be an orchestrator")
	cfg.Transcoder = flag.Bool("transcoder", *cfg.Transcoder, "Set to true to be a transcoder")
	cfg.Gateway = flag.Bool("gateway", *cfg.Broadcaster, "Set to true to be a gateway")
	cfg.Broadcaster = flag.Bool("broadcaster", *cfg.Broadcaster, "Set to true to be a broadcaster (**Deprecated**, use -gateway)")
	cfg.OrchSecret = flag.String("orchSecret", *cfg.OrchSecret, "Shared secret with the orchestrator as a standalone transcoder or path to file")
	cfg.TranscodingOptions = flag.String("transcodingOptions", *cfg.TranscodingOptions, "Transcoding options for broadcast job, or path to json config")
	cfg.MaxAttempts = flag.Int("maxAttempts", *cfg.MaxAttempts, "Maximum transcode attempts")
	cfg.MaxSessions = flag.String("maxSessions", *cfg.MaxSessions, "Maximum number of concurrent transcoding sessions for Orchestrator or 'auto' for dynamic limit, maximum number of RTMP streams for Broadcaster, or maximum capacity for transcoder.")
	cfg.CurrentManifest = flag.Bool("currentManifest", *cfg.CurrentManifest, "Expose the currently active ManifestID as \"/stream/current.m3u8\"")
	cfg.Nvidia = flag.String("nvidia", *cfg.Nvidia, "Comma-separated list of Nvidia GPU device IDs (or \"all\" for all available devices)")
	cfg.Netint = flag.String("netint", *cfg.Netint, "Comma-separated list of NetInt device GUIDs (or \"all\" for all available devices)")
	cfg.TestTranscoder = flag.Bool("testTranscoder", *cfg.TestTranscoder, "Test Nvidia GPU transcoding at startup")
	cfg.HevcDecoding = flag.Bool("hevcDecoding", *cfg.HevcDecoding, "Enable or disable HEVC decoding")

	// Onchain:
	cfg.EthAcctAddr = flag.String("ethAcctAddr", *cfg.EthAcctAddr, "Existing Eth account address. For use when multiple ETH accounts exist in the keystore directory")
	cfg.EthPassword = flag.String("ethPassword", *cfg.EthPassword, "Password for existing Eth account address or path to file")
	cfg.EthKeystorePath = flag.String("ethKeystorePath", *cfg.EthKeystorePath, "Path to ETH keystore directory or keyfile. If keyfile, overrides -ethAcctAddr and uses parent directory")
	cfg.EthOrchAddr = flag.String("ethOrchAddr", *cfg.EthOrchAddr, "ETH address of an on-chain registered orchestrator")
	cfg.EthUrl = flag.String("ethUrl", *cfg.EthUrl, "Ethereum node JSON-RPC URL")
	cfg.TxTimeout = flag.Duration("transactionTimeout", *cfg.TxTimeout, "Amount of time to wait for an Ethereum transaction to confirm before timing out")
	cfg.MaxTxReplacements = flag.Int("maxTransactionReplacements", *cfg.MaxTxReplacements, "Number of times to automatically replace pending Ethereum transactions")
	cfg.GasLimit = flag.Int("gasLimit", *cfg.GasLimit, "Gas limit for ETH transactions")
	cfg.MinGasPrice = flag.Int64("minGasPrice", 0, "Minimum gas price (priority fee + base fee) for ETH transactions in wei, 10 Gwei = 10000000000")
	cfg.MaxGasPrice = flag.Int("maxGasPrice", *cfg.MaxGasPrice, "Maximum gas price (priority fee + base fee) for ETH transactions in wei, 40 Gwei = 40000000000")
	cfg.EthController = flag.String("ethController", *cfg.EthController, "Protocol smart contract address")
	cfg.InitializeRound = flag.Bool("initializeRound", *cfg.InitializeRound, "Set to true if running as a transcoder and the node should automatically initialize new rounds")
	cfg.InitializeRoundMaxDelay = flag.Duration("initializeRoundMaxDelay", *cfg.InitializeRoundMaxDelay, "Maximum delay to wait before initializing a round")
	cfg.TicketEV = flag.String("ticketEV", *cfg.TicketEV, "The expected value for PM tickets")
	cfg.MaxFaceValue = flag.String("maxFaceValue", *cfg.MaxFaceValue, "set max ticket face value in WEI")
	// Broadcaster max acceptable ticket EV
	cfg.MaxTicketEV = flag.String("maxTicketEV", *cfg.MaxTicketEV, "The maximum acceptable expected value for one PM ticket")
	// Broadcaster max acceptable total EV for one payment
	cfg.MaxTotalEV = flag.String("maxTotalEV", *cfg.MaxTotalEV, "The maximum acceptable expected value for one PM payment")
	// Broadcaster deposit multiplier to determine max acceptable ticket faceValue
	cfg.DepositMultiplier = flag.Int("depositMultiplier", *cfg.DepositMultiplier, "The deposit multiplier used to determine max acceptable faceValue for PM tickets")
	// Orchestrator base pricing info
	cfg.PricePerUnit = flag.String("pricePerUnit", "0", "The price per 'pixelsPerUnit' amount pixels. Can be specified in wei or a custom currency in the format <price><currency> (e.g. 0.50USD). When using a custom currency, a corresponding price feed must be configured with -priceFeedAddr")
	// Unit of pixels for both O's pricePerUnit and B's maxPricePerUnit
	cfg.PixelsPerUnit = flag.String("pixelsPerUnit", *cfg.PixelsPerUnit, "Amount of pixels per unit. Set to '> 1' to have smaller price granularity than 1 wei / pixel")
	cfg.PriceFeedAddr = flag.String("priceFeedAddr", *cfg.PriceFeedAddr, "ETH address of the Chainlink price feed contract. Used for custom currencies conversion on -pricePerUnit or -maxPricePerUnit")
	cfg.AutoAdjustPrice = flag.Bool("autoAdjustPrice", *cfg.AutoAdjustPrice, "Enable/disable automatic price adjustments based on the overhead for redeeming tickets")
	cfg.PricePerGateway = flag.String("pricePerGateway", *cfg.PricePerGateway, `json list of price per gateway or path to json config file. Example: {"gateways":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	cfg.PricePerBroadcaster = flag.String("pricePerBroadcaster", *cfg.PricePerBroadcaster, `json list of price per broadcaster or path to json config file. Example: {"broadcasters":[{"ethaddress":"address1","priceperunit":0.5,"currency":"USD","pixelsperunit":1000000000000},{"ethaddress":"address2","priceperunit":0.3,"currency":"USD","pixelsperunit":1000000000000}]}`)
	// Interval to poll for blocks
	cfg.BlockPollingInterval = flag.Int("blockPollingInterval", *cfg.BlockPollingInterval, "Interval in seconds at which different blockchain event services poll for blocks")
	// Redemption service
	cfg.Redeemer = flag.Bool("redeemer", *cfg.Redeemer, "Set to true to run a ticket redemption service")
	cfg.RedeemerAddr = flag.String("redeemerAddr", *cfg.RedeemerAddr, "URL of the ticket redemption service to use")
	// Reward service
	cfg.Reward = flag.Bool("reward", false, "Set to true to run a reward service")
	// Metrics & logging:
	cfg.Monitor = flag.Bool("monitor", *cfg.Monitor, "Set to true to send performance metrics")
	cfg.MetricsPerStream = flag.Bool("metricsPerStream", *cfg.MetricsPerStream, "Set to true to group performance metrics per stream")
	cfg.MetricsExposeClientIP = flag.Bool("metricsClientIP", *cfg.MetricsExposeClientIP, "Set to true to expose client's IP in metrics")
	cfg.MetadataQueueUri = flag.String("metadataQueueUri", *cfg.MetadataQueueUri, "URI for message broker to send operation metadata")
	cfg.MetadataAmqpExchange = flag.String("metadataAmqpExchange", *cfg.MetadataAmqpExchange, "Name of AMQP exchange to send operation metadata")
	cfg.MetadataPublishTimeout = flag.Duration("metadataPublishTimeout", *cfg.MetadataPublishTimeout, "Max time to wait in background for publishing operation metadata events")

	// Storage:
	flag.StringVar(cfg.Datadir, "datadir", *cfg.Datadir, "[Deprecated] Directory that data is stored in")
	flag.StringVar(cfg.Datadir, "dataDir", *cfg.Datadir, "Directory that data is stored in")
	cfg.Objectstore = flag.String("objectStore", *cfg.Objectstore, "url of primary object store")
	cfg.Recordstore = flag.String("recordStore", *cfg.Recordstore, "url of object store for recordings")

	// Fast Verification GS bucket:
	cfg.FVfailGsBucket = flag.String("FVfailGsbucket", *cfg.FVfailGsBucket, "Google Cloud Storage bucket for storing segments, which failed fast verification")
	cfg.FVfailGsKey = flag.String("FVfailGskey", *cfg.FVfailGsKey, "Google Cloud Storage private key file name or key in JSON format for accessing FVfailGsBucket")
	// API
	cfg.AuthWebhookURL = flag.String("authWebhookUrl", *cfg.AuthWebhookURL, "RTMP authentication webhook URL")

	// flags
	cfg.TestOrchAvail = flag.Bool("startupAvailabilityCheck", *cfg.TestOrchAvail, "Set to false to disable the startup Orchestrator availability check on the configured serviceAddr")

	return cfg
}

// updateNilsForUnsetFlags changes some cfg fields to nil if they were not explicitly set with flags.
// For some flags, the behavior is different whether the value is default or not set by the user at all.
func updateNilsForUnsetFlags(cfg starter.LivepeerConfig) starter.LivepeerConfig {
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
