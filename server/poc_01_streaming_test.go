//go:build nvidia
// +build nvidia

package server

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Represents one mining farm
type OCluster struct {
	port        int
	oIngest     *OrchestratorIngest
	transcoders []*StreamingTranscoder
	Url         url.URL
	trusted     bool
}

// Specify GPU index for new transcoder. Tests run on same machine.
type TestTranscoderSpec struct {
	GpuIndex        int
	ConnectionLimit int
}

func TestStreaming_DataFlow(t *testing.T) {
	rand.Seed(time.Now().Unix())

	// Using 5 workers for GPU
	gpusToUse := []TestTranscoderSpec{{0, 5}} // single GPU scenario

	trusted_O := newOCluster(t, 2020, gpusToUse, true)
	untrusted_O := newOCluster(t, 2022, gpusToUse, false)
	// TODO: create corrupted O, sneaky, returning input as outputs
	bPort := 2024
	B, err := newBroadcaster(bPort, []*OrchestratorContact{trusted_O.Contact(), untrusted_O.Contact()})
	require.NoError(t, err)
	// Here we tell to Os what is B's offchain address. In production this is read from chain.
	trusted_O.MeetBroadcaster(B.verifyer)
	untrusted_O.MeetBroadcaster(B.verifyer)

	time.Sleep(300 * time.Millisecond)

	// Create Mist
	mist := newMist(t, bPort)
	mist.Run(t)
}

// Prepare input. Use several input segments & real segment boundary
func newMist(t *testing.T, broadcasterPort int) *MistMockup {
	input := readInput(t, []string{"input0_15730.ts", "input1_14650.ts", "input2_16100.ts", "input3_15000.ts", "input4_14860.ts"})
	return &MistMockup{
		// Live Mist would receive stream, here we read data from file
		inputData: input,
		// Live Mist would extract metadata, here we hardcoded it
		inputFormat: MediaFormatInfo{
			VideoCodec: "h264",
			AudioCodec: "aac",
			FpKs:       24000,
			Width:      2048,
			Height:     858,
		},
		// Pass Url to B node
		url: url.URL{Scheme: "ws", Host: fmt.Sprintf("127.0.0.1:%d", broadcasterPort), Path: "/streaming"},
	}
}

func (c *OCluster) Contact() *OrchestratorContact {
	return &OrchestratorContact{
		Trusted:   c.trusted,
		verifyer:  c.oIngest.verifyer,
		WsAddress: c.Url,
	}
}

// Used from test to introduce B crypto-address to O, offchain
func (c *OCluster) MeetBroadcaster(verifyer SignatureChecker) {
	c.oIngest.SingleKnownBaddress = verifyer
}

func newOCluster(t *testing.T, port int, transcoders []TestTranscoderSpec, trusted bool) *OCluster {
	// O and connected Ts share same passphrase
	transcoderPassword := "testing"
	cluster := &OCluster{
		port:        port,
		oIngest:     &OrchestratorIngest{port: port},
		transcoders: make([]*StreamingTranscoder, 0),
		trusted:     trusted,
	}
	mux := http.NewServeMux()
	router := &TestOrchRouter{}
	pool := &OrchestratorTranscoderPool{
		loginPassword:  transcoderPassword,
		maxConnections: 20,
	}
	pool.Init()
	// New wallet for O to sign data
	wallet := TestWallet{}
	wallet.Init()

	cluster.oIngest.Init(mux, router, pool, wallet)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	require.NoError(t, err)
	// Run server in separate goro
	go http.Serve(listener, mux)
	// Spawn transcoders, point them to our O
	cluster.Url = url.URL{Scheme: "ws", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/streaming/"}
	transcoderUrl := url.URL{Scheme: "ws", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: "/transcoder/"}
	for _, spec := range transcoders {
		// Usually one if single GPU installed
		transcoder := StreamingTranscoder{
			connectionLimit: spec.ConnectionLimit,
			hwDeviceIndex:   spec.GpuIndex,
			orchestratorUrl: transcoderUrl,
			loginPassword:   transcoderPassword,
		}
		transcoder.Start()
		cluster.transcoders = append(cluster.transcoders, &transcoder)
	}
	return cluster
}

func newBroadcaster(port int, orchestrators []*OrchestratorContact) (*BroadcasterIngest, error) {
	router := &TestBroadasterRouter{
		availableOrchestrators: orchestrators,
		verificationFrequency:  3,
	}
	mux := http.NewServeMux()
	b := &BroadcasterIngest{}
	wallet := TestWallet{}
	wallet.Init()
	b.Init(mux, router, wallet)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	go http.Serve(listener, mux)
	fmt.Printf("B listening on port %d \n", port)
	return b, nil
}

func readInput(t *testing.T, names []string) []*InputSpec {
	wd, err := os.Getwd()
	require.NoError(t, err)
	segments := make([]*InputSpec, 0, len(names))
	for _, name := range names {
		inputFileName := path.Join(wd, "..", "samples", name)
		data, err := ioutil.ReadFile(inputFileName)
		require.NoError(t, err)
		ms, err := strconv.Atoi(strings.Split(name, "_")[1])
		segments = append(segments, &InputSpec{name, data, ms})
	}
	return segments
}
