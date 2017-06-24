package main

import (
	"context"
	"flag"

	"github.com/golang/glog"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/livepeer/libp2p-livepeer/core"
	"github.com/livepeer/libp2p-livepeer/mediaserver"
)

func main() {
	port := flag.Int("p", 0, "port")
	httpPort := flag.String("http", "", "http port")
	rtmpPort := flag.String("rtmp", "", "rtmp port")
	flag.Parse()

	if *port == 0 {
		glog.Fatalf("Please provide port")
	}
	if *httpPort == "" {
		glog.Fatalf("Please provide http port")
	}
	if *rtmpPort == "" {
		glog.Fatalf("Please provide rtmp port")
	}

	priv, pub, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)

	n, err := core.NewLivepeerNode(*port, priv, pub)
	if err != nil {
		glog.Errorf("Error creating livepeer node: %v", err)
	}
	s := mediaserver.NewLivepeerMediaServer(*rtmpPort, *httpPort, "", n)
	s.StartLPMS(context.Background())

	select {}
}

// import (
// 	"crypto/ecdsa"
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"os/exec"
// 	"runtime"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/codegangsta/cli"
// 	"github.com/ethereum/go-ethereum/accounts"
// 	"github.com/ethereum/go-ethereum/cmd/utils"
// 	"github.com/ethereum/go-ethereum/common"
// 	"github.com/ethereum/go-ethereum/console"
// 	"github.com/ethereum/go-ethereum/crypto"
// 	"github.com/ethereum/go-ethereum/ethclient"
// 	"github.com/ethereum/go-ethereum/metrics"
// 	"github.com/ethereum/go-ethereum/node"
// 	"github.com/ethereum/go-ethereum/p2p"
// 	"github.com/ethereum/go-ethereum/p2p/discover"
// 	"github.com/golang/glog"
// 	"github.com/ossrs/go-oryx-lib/logger"

// 	lp "github.com/livepeer/go-livepeer/livepeer"
// 	bzzapi "github.com/livepeer/go-livepeer/livepeer/api"
// 	"github.com/livepeer/go-livepeer/livepeer/network"

// 	streamingVizClient "github.com/livepeer/streamingviz/client"
// )

// const (
// 	clientIdentifier = "livepeer"
// 	versionString    = "0.1"
// )

// var (
// 	gitCommit       string
// 	app             = utils.NewApp(gitCommit, "Livepeer")
// 	toynetBootNodes = []string{
// 		"enode://fb4c232eb8ed424d513b3a9df229fe003629420f62041381ff5b560c0b69ad4e4c977cc475391295217a7e4b324d8303922989aa699956201f79412ec38247f0@52.14.103.190:30399",
// 		"enode://5639b23de156e605eac5e169a44917767d3776f938a96bbdd0e26ef40235bc23b9c219d55ec3bbe84b4461659142e2f991fd994847ecb5179ca2c53db864ec0e@52.14.16.38:30399",
// 	}
// )

// var (
// 	ChequebookAddrFlag = cli.StringFlag{
// 		Name:  "chequebook",
// 		Usage: "chequebook contract address",
// 	}
// 	SwarmAccountFlag = cli.StringFlag{
// 		Name:  "bzzaccount",
// 		Usage: "Swarm account key file",
// 	}
// 	SwarmPortFlag = cli.StringFlag{
// 		Name:  "bzzport",
// 		Usage: "Swarm local http api port",
// 	}
// 	SwarmNetworkIdFlag = cli.IntFlag{
// 		Name:  "bzznetworkid",
// 		Usage: "Network identifier (integer, default 3=swarm testnet)",
// 		Value: network.NetworkId,
// 	}
// 	SwarmConfigPathFlag = cli.StringFlag{
// 		Name:  "bzzconfig",
// 		Usage: "Swarm config file path (datadir/bzz)",
// 	}
// 	SwarmSwapEnabledFlag = cli.BoolFlag{
// 		Name:  "swap",
// 		Usage: "Swarm SWAP enabled (default false)",
// 	}
// 	SwarmSyncEnabledFlag = cli.BoolTFlag{
// 		Name:  "sync",
// 		Usage: "Swarm Syncing enabled (default true)",
// 	}
// 	EthAPIFlag = cli.StringFlag{
// 		Name:  "ethapi",
// 		Usage: "URL of the Ethereum API provider",
// 		//Value: node.DefaultIPCEndpoint("geth"),
// 		// Set to '' as a default since I don't see why we need geth yet
// 		Value: "",
// 	}
// 	SwarmApiFlag = cli.StringFlag{
// 		Name:  "bzzapi",
// 		Usage: "Swarm HTTP endpoint",
// 		Value: "http://127.0.0.1:8500",
// 	}
// 	SwarmRecursiveUploadFlag = cli.BoolFlag{
// 		Name:  "recursive",
// 		Usage: "Upload directories recursively",
// 	}
// 	SwarmWantManifestFlag = cli.BoolTFlag{
// 		Name:  "manifest",
// 		Usage: "Automatic manifest upload",
// 	}
// 	SwarmUploadDefaultPath = cli.StringFlag{
// 		Name:  "defaultpath",
// 		Usage: "path to file served for empty url path (none)",
// 	}
// 	CorsStringFlag = cli.StringFlag{
// 		Name:  "corsdomain",
// 		Usage: "Domain on which to send Access-Control-Allow-Origin header (multiple domains can be supplied separated by a ',')",
// 	}
// 	RTMPFlag = cli.StringFlag{
// 		Name:  "rtmp",
// 		Usage: "Specify RTMP streaming port",
// 		Value: "1935",
// 	}
// 	FFMpegPathFlag = cli.StringFlag{
// 		Name:  "ffmpegPath",
// 		Usage: "path to ffmpeg",
// 		Value: "",
// 	}
// 	HLSFlag = cli.BoolFlag{
// 		Name:  "hls",
// 		Usage: "True if you'd like to stream the HLS rendition",
// 	}
// 	MetricsEnabledFlag = cli.BoolFlag{
// 		Name:  metrics.MetricsEnabledFlag,
// 		Usage: "Enable metrics collection and reporting",
// 	}
// 	VizEnabledFlag = cli.BoolFlag{
// 		Name:  "viz",
// 		Usage: "true if you want to talk to a metrics visualization server",
// 	}
// 	VizHostFlag = cli.StringFlag{
// 		Name:  "vizhost",
// 		Usage: "The url + port to communicate to the visualization server (ex/default 'http://localhost:8585')",
// 		Value: "http://localhost:8585",
// 	}
// 	LivepeerNetworkIdFlag = cli.IntFlag{
// 		Name:  "lpnetworkid",
// 		Usage: "Network identifier (integer, default 326326=livepeer toy net)",
// 		Value: network.NetworkId,
// 	}
// )

// func init() {
// 	// Override flag defaults so bzzd can run alongside geth.
// 	utils.ListenPortFlag.Value = 30399
// 	utils.IPCPathFlag.Value = utils.DirectoryString{Value: "bzzd.ipc"}
// 	utils.IPCApiFlag.Value = "admin, bzz, chequebook, debug, rpc, web3"
// 	utils.LPNetFlag.Value = "true"

// 	// Set up the cli app.
// 	app.Action = livepeer
// 	app.HideVersion = true
// 	app.Copyright = "Copyright 2013-2017 THe go-ethereum Authors, and Livepeer extensions copyright 2017 Doug and Eric"

// 	app.Commands = []cli.Command{
// 		{
// 			Action:      stream,
// 			Name:        "stream",
// 			Usage:       "Connect to a live stream by id. Pass an optional --rtmp <port> argument (1935 default)",
// 			ArgsUsage:   " <streamID>",
// 			Description: "This command will use ffplay to play the given stream ID from the Livepeer network",
// 		},
// 		{
// 			Action:    version,
// 			Name:      "version",
// 			Usage:     "Print version numbers",
// 			ArgsUsage: " ",
// 			Description: `
// The output of this command is supposed to be machine-readable.
// `,
// 		},
// 	}

// 	app.Flags = []cli.Flag{
// 		utils.IdentityFlag,
// 		utils.DataDirFlag,
// 		utils.BootnodesFlag,
// 		utils.KeyStoreDirFlag,
// 		utils.ListenPortFlag,
// 		utils.NoDiscoverFlag,
// 		utils.DiscoveryV5Flag,
// 		utils.NetrestrictFlag,
// 		utils.NodeKeyFileFlag,
// 		utils.NodeKeyHexFlag,
// 		utils.MaxPeersFlag,
// 		utils.NATFlag,
// 		utils.IPCDisabledFlag,
// 		utils.IPCApiFlag,
// 		utils.IPCPathFlag,
// 		utils.LPNetFlag,
// 		// bzzd-specific flags
// 		CorsStringFlag,
// 		EthAPIFlag,
// 		SwarmConfigPathFlag,
// 		SwarmSwapEnabledFlag,
// 		SwarmSyncEnabledFlag,
// 		SwarmPortFlag,
// 		SwarmAccountFlag,
// 		SwarmNetworkIdFlag,
// 		ChequebookAddrFlag,
// 		// upload flags
// 		SwarmApiFlag,
// 		SwarmRecursiveUploadFlag,
// 		SwarmWantManifestFlag,
// 		SwarmUploadDefaultPath,
// 		// streaming flags
// 		RTMPFlag,
// 		FFMpegPathFlag,
// 		HLSFlag,
// 		MetricsEnabledFlag,
// 		VizEnabledFlag,
// 		VizHostFlag,
// 		LivepeerNetworkIdFlag,
// 	}
// 	app.Flags = append(app.Flags, debug.Flags...)
// 	app.Before = func(ctx *cli.Context) error {
// 		runtime.GOMAXPROCS(runtime.NumCPU())
// 		return debug.Setup(ctx)
// 	}
// 	app.After = func(ctx *cli.Context) error {
// 		debug.Exit()
// 		return nil
// 	}
// }

// func main() {
// 	if err := app.Run(os.Args); err != nil {
// 		fmt.Fprintln(os.Stderr, err)
// 		os.Exit(1)
// 	}
// }

// func version(ctx *cli.Context) error {
// 	fmt.Println(strings.Title(clientIdentifier))
// 	fmt.Println("Version:", versionString)
// 	if gitCommit != "" {
// 		fmt.Println("Git Commit:", gitCommit)
// 	}
// 	fmt.Println("Network Id:", ctx.GlobalInt(utils.NetworkIdFlag.Name))
// 	fmt.Println("Go Version:", runtime.Version())
// 	fmt.Println("OS:", runtime.GOOS)
// 	fmt.Printf("GOPATH=%s\n", os.Getenv("GOPATH"))
// 	fmt.Printf("GOROOT=%s\n", runtime.GOROOT())
// 	return nil
// }

// func livepeer(ctx *cli.Context) error {
// 	vizClient := streamingVizClient.NewClient("", ctx.GlobalBool(VizEnabledFlag.Name), ctx.GlobalString(VizHostFlag.Name)) // Assing nodeID after the node starts

// 	// Start consuming visualization events and reporting your peers every 20 seconds.
// 	// See comments below near the consumeVizEvents() method
// 	vizClient.NodeID = fmt.Sprintf("%s", stack.Server().Self().ID)

// 	donePeers := make(chan bool)
// 	doneEvents := make(chan bool)

// 	vizClient.ConsumeEvents(doneEvents)
// 	go startPeerReporting(stack, donePeers, vizClient)

// 	stack.Wait()

// 	// Close the peer reporting loop
// 	donePeers <- true
// 	doneEvents <- true

// 	return nil
// }

// func stream(ctx *cli.Context) error {
// 	port := ctx.GlobalString(RTMPFlag.Name)
// 	streamID := ctx.Args()[0]
// 	var rtmpURL string

// 	// Determine if you are streaming the HLS or RTMP version. If --hls is passed in, stream HLS
// 	hlsRequest := ctx.GlobalBool(HLSFlag.Name)
// 	if hlsRequest == true {
// 		numericPort, err := strconv.Atoi(port)
// 		if err != nil {
// 			fmt.Println("Need an rtmp port")
// 			os.Exit(1)
// 		}

// 		port = strconv.Itoa(numericPort + 7000) // HLS port is 7000 + RTMP by default

// 		rtmpURL = fmt.Sprintf("http://localhost:%v/stream/%v.m3u8", port, streamID)
// 	} else {
// 		rtmpURL = fmt.Sprintf("rtmp://localhost:%v/stream/%v", port, streamID)
// 	}

// 	cmd := exec.Command("ffplay", rtmpURL)
// 	err := cmd.Start()
// 	if err != nil {
// 		fmt.Println("Couldn't start the stream")
// 		os.Exit(1)
// 	}
// 	fmt.Println("Now streaming")
// 	err = cmd.Wait()
// 	fmt.Println("Finished the stream")
// 	return nil
// }

// // Call peer reporting event at some fixed interval like 20 seconds for the visualization server
// func startPeerReporting(node *node.Node, doneChan chan bool, vizClient *streamingVizClient.Client) {
// 	tickChan := time.NewTicker(20 * time.Second).C
// 	for {
// 		select {
// 		case <-tickChan:
// 			peers := node.Server().PeersInfo()
// 			peerIDs := make([]string, 0, len(peers))
// 			for _, p := range peers {
// 				peerIDs = append(peerIDs, p.ID)
// 			}
// 			vizClient.LogPeers(peerIDs)
// 		case <-doneChan:
// 			return
// 		}
// 	}
// }

// func registerBzzService(ctx *cli.Context, stack *node.Node, viz *streamingVizClient.Client) {
// 	prvkey := getAccount(ctx, stack)

// 	chbookaddr := common.HexToAddress(ctx.GlobalString(ChequebookAddrFlag.Name))
// 	bzzdir := ctx.GlobalString(SwarmConfigPathFlag.Name)
// 	if bzzdir == "" {
// 		bzzdir = stack.InstanceDir()
// 	}

// 	bzzconfig, err := bzzapi.NewConfig(bzzdir, chbookaddr, prvkey, ctx.GlobalUint64(LivepeerNetworkIdFlag.Name), ctx.GlobalString(RTMPFlag.Name), ctx.GlobalString(FFMpegPathFlag.Name))
// 	if err != nil {
// 		utils.Fatalf("unable to configure swarm: %v", err)
// 	}
// 	bzzport := ctx.GlobalString(SwarmPortFlag.Name)
// 	if len(bzzport) > 0 {
// 		bzzconfig.Port = bzzport
// 	}
// 	swapEnabled := ctx.GlobalBool(SwarmSwapEnabledFlag.Name)
// 	syncEnabled := ctx.GlobalBoolT(SwarmSyncEnabledFlag.Name)

// 	ethapi := ctx.GlobalString(EthAPIFlag.Name)
// 	cors := ctx.GlobalString(CorsStringFlag.Name)

// 	boot := func(ctx *node.ServiceContext) (node.Service, error) {
// 		var client *ethclient.Client
// 		if len(ethapi) > 0 {
// 			client, err = ethclient.Dial(ethapi)
// 			if err != nil {
// 				utils.Fatalf("Can't connect: %v", err)
// 			}
// 		}

// 		return lp.NewSwarm(ctx, client, bzzconfig, swapEnabled, syncEnabled, cors, viz)
// 	}
// 	if err := stack.Register(boot); err != nil {
// 		utils.Fatalf("Failed to register the Swarm service: %v", err)
// 	}
// }

// func getAccount(ctx *cli.Context, stack *node.Node) *ecdsa.PrivateKey {
// 	keyid := ctx.GlobalString(SwarmAccountFlag.Name)
// 	if keyid == "" {
// 		//utils.Fatalf("Option %q is required", SwarmAccountFlag.Name)
// 		// No account given, try using the first account
// 		keyid = findOrGenerateFirstAccount(stack.AccountManager(), ctx)
// 		fmt.Println("Found the account", keyid)
// 	}
// 	// Try to load the arg as a hex key file.
// 	if key, err := crypto.LoadECDSA(keyid); err == nil {
// 		glog.V(logger.Info).Infof("swarm account key loaded: %#x", crypto.PubkeyToAddress(key.PublicKey))
// 		return key
// 	}
// 	// Otherwise try getting it from the keystore.
// 	return decryptStoreAccount(stack.AccountManager(), keyid)
// }

// func findOrGenerateFirstAccount(accman *accounts.Manager, ctx *cli.Context) (keyid string) {
// 	acc, err := accman.AccountByIndex(0)
// 	if err != nil {
// 		// Don't have an account, need to generate
// 		password := getPassPhrase("Need to create an account. Please give a password. Do not forget this password.", true, 0, utils.MakePasswordList(ctx))

// 		acc, err = accman.NewAccount(password)
// 		if err != nil {
// 			utils.Fatalf("Failed to create account: %v", err)
// 		}
// 		fmt.Printf("New Address: {%x}\n", acc.Address)
// 	}
// 	return acc.Address.Hex()
// }

// func decryptStoreAccount(accman *accounts.Manager, account string) *ecdsa.PrivateKey {
// 	var a accounts.Account
// 	var err error
// 	if common.IsHexAddress(account) {
// 		a, err = accman.Find(accounts.Account{Address: common.HexToAddress(account)})
// 	} else if ix, ixerr := strconv.Atoi(account); ixerr == nil {
// 		a, err = accman.AccountByIndex(ix)
// 	} else {
// 		utils.Fatalf("Can't find swarm account key %s", account)
// 	}
// 	if err != nil {
// 		utils.Fatalf("Can't find swarm account key: %v", err)
// 	}
// 	keyjson, err := ioutil.ReadFile(a.File)
// 	if err != nil {
// 		utils.Fatalf("Can't load swarm account key: %v", err)
// 	}
// 	for i := 1; i <= 3; i++ {
// 		passphrase := promptPassphrase(fmt.Sprintf("Unlocking swarm account %s [%d/3]", a.Address.Hex(), i))
// 		key, err := accounts.DecryptKey(keyjson, passphrase)
// 		if err == nil {
// 			return key.PrivateKey
// 		}
// 	}
// 	utils.Fatalf("Can't decrypt swarm account key")
// 	return nil
// }

// func promptPassphrase(prompt string) string {
// 	if prompt != "" {
// 		fmt.Println(prompt)
// 	}
// 	password, err := console.Stdin.PromptPassword("Passphrase: ")
// 	if err != nil {
// 		utils.Fatalf("Failed to read passphrase: %v", err)
// 	}
// 	return password
// }

// // getPassPhrase retrieves the passwor associated with an account, either fetched
// // from a list of preloaded passphrases, or requested interactively from the user.
// func getPassPhrase(prompt string, confirmation bool, i int, passwords []string) string {
// 	// If a list of passwords was supplied, retrieve from them
// 	if len(passwords) > 0 {
// 		if i < len(passwords) {
// 			return passwords[i]
// 		}
// 		return passwords[len(passwords)-1]
// 	}
// 	// Otherwise prompt the user for the password
// 	if prompt != "" {
// 		fmt.Println(prompt)
// 	}
// 	password, err := console.Stdin.PromptPassword("Passphrase: ")
// 	if err != nil {
// 		utils.Fatalf("Failed to read passphrase: %v", err)
// 	}
// 	if confirmation {
// 		confirm, err := console.Stdin.PromptPassword("Repeat passphrase: ")
// 		if err != nil {
// 			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
// 		}
// 		if password != confirm {
// 			utils.Fatalf("Passphrases do not match")
// 		}
// 	}
// 	return password
// }

// func injectBootnodes(srv *p2p.Server, nodes []string) {
// 	for _, url := range nodes {
// 		n, err := discover.ParseNode(url)
// 		if err != nil {
// 			glog.Errorf("invalid bootnode %q", err)
// 			continue
// 		}
// 		srv.AddPeer(n)
// 	}
// }
