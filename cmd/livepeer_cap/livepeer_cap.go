package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/discovery"
)

func validateURL(u string) (*url.URL, error) {
	if u == "" {
		return nil, nil
	}
	p, err := url.ParseRequestURI(u)
	if err != nil {
		return nil, err
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return nil, errors.New("URL should be HTTP or HTTPS")
	}
	return p, nil
}

func main() {
	flag.Set("logtostderr", "true")
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	orchWebhookURL := flag.String("orchWebhookUrl", "", "Orchestrator discovery callback URL")
	orchAddr := flag.String("orchAddr", "", "Comma delimited list of orchestrator URIs (IP:port or hostname) to use")
	broadcasterAddr := flag.String("broadcasterAddr", "", "Broadcaster ETH address")
	broadcasterSig := flag.String("broadcasterSig", "", "Broadcaster signature over own ETH address (plaintext or file)")
	caps := flag.String("caps", "", "List of capabilities to check for")

	flag.Parse()

	if *broadcasterAddr == "" {
		glog.Fatalf("Missing -broadcasterAddr")
	}
	if *broadcasterSig == "" {
		glog.Fatalf("Missing -broadcasterSig")
	}

	sig, err := common.ReadFileOrString(*broadcasterSig)
	if err != nil {
		glog.Fatalf("Error reading -broadcasterSig: %v", err)
	}

	signer := newStaticSigner(ethcommon.HexToAddress(*broadcasterAddr), ethcommon.FromHex(sig))

	var pool common.OrchestratorPool
	if *orchWebhookURL != "" {
		whurl, err := validateURL(*orchWebhookURL)
		if err != nil {
			glog.Fatalf("Error validating -orchWebhookUrl: %v", err)
		}
		pool = discovery.NewWebhookPool(signer, whurl)
	} else if *orchAddr != "" {
		var uris []*url.URL
		for _, addr := range strings.Split(*orchAddr, ",") {
			uri, err := validateURL(addr)
			if err != nil {
				glog.Errorf("Could not parse orchestrator URI: %v", err)
				continue
			}
			uris = append(uris, uri)
		}
		pool = discovery.NewOrchestratorPool(signer, uris)
	} else {
		glog.Fatalf("Missing -orchWebhookURL or -orchAddr")
	}

	// Initialize the capability list with mandatories
	// Otherwise, orchestrators that define the mandatories will be considered incompatible
	capLst := core.MandatoryCapabilities()
	for _, cap := range strings.Split(*caps, ",") {
		capLst = append(capLst, core.NameToCapability[cap])
	}

	infos, err := pool.GetOrchestrators(pool.Size(), &fakeSuspender{}, core.NewCapabilities(capLst, nil))
	if err != nil {
		glog.Fatalf("Error querying orchestrators: %v", err)
	}

	fmt.Println(len(infos))

	for _, info := range infos {
		fmt.Printf("%v,%v\n", hexutil.Encode(info.GetTicketParams().GetRecipient()), info.Transcoder)
	}
}
