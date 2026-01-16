package byoc

import (
	"context"
	"crypto/tls"
	"errors"
	"math/big"
	gonet "net"
	"net/http"
	"net/url"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/go-tools/drivers"
)

// Header constants
const (
	jobRequestHdr                   = "Livepeer"
	jobEthAddressHdr                = "Livepeer-Eth-Address"
	jobCapabilityHdr                = "Livepeer-Capability"
	jobPaymentHeaderHdr             = "Livepeer-Payment"
	jobPaymentBalanceHdr            = "Livepeer-Balance"
	jobOrchSearchTimeoutHdr         = "Livepeer-Orch-Search-Timeout"
	jobOrchSearchRespTimeoutHdr     = "Livepeer-Orch-Search-Resp-Timeout"
	jobOrchSearchTimeoutDefault     = 1 * time.Second
	jobOrchSearchRespTimeoutDefault = 500 * time.Millisecond
)

// Error variables
var (
	errNoTimeoutSet         = errors.New("no timeout_seconds set with request, timeout_seconds is required")
	errNoCapabilityCapacity = errors.New("No capacity available for capability")
	errNoJobCreds           = errors.New("Could not verify job creds")
	errPaymentError         = errors.New("Could not parse payment")
	errInsufficientBalance  = errors.New("Insufficient balance for request")
	errZeroCapacity         = errors.New("zero capacity")
	errSegEncoding          = errors.New("ErrorSegEncoding")
	errSegSig               = errors.New("ErrSegSig")
)

// Orchestrator is a subset of the Orchestrator interface for orchestrator operations needed by BYOC
type Orchestrator interface {
	TranscoderSecret() string
	VerifySig(addr ethcommon.Address, msg string, sig []byte) bool
	ReserveExternalCapabilityCapacity(capability string) error
	GetUrlForCapability(capability string) string
	JobPriceInfo(sender ethcommon.Address, capability string) (*net.PriceInfo, error)
	TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error)
	Balance(sender ethcommon.Address, manifestID core.ManifestID) *big.Rat
	CheckExternalCapabilityCapacity(capability string) int64
	RemoveExternalCapability(extCapName string) error
	RegisterExternalCapability(extCapSettings string) (*core.ExternalCapability, error)
	FreeExternalCapabilityCapacity(capability string) error
	ServiceURI() *url.URL
	ProcessPayment(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error
	DebitFees(sender ethcommon.Address, manifestID core.ManifestID, priceInfo *net.PriceInfo, units int64)
}

// interface to interact with streamStatusStore passed to BYOCGatewayServer at initialization
type StatusStore interface {
	Store(streamID string, status map[string]interface{})
	StoreKey(streamID, key string, status interface{})
	Clear(streamID string)
	Get(streamID string) (map[string]interface{}, bool)
	StoreIfNotExists(streamID string, key string, status interface{})
}

type OrchestratorSwapper interface {
	checkSwap(ctx context.Context) error
}

type BYOCStreamPipeline struct {
	RequestID    string
	StreamID     string
	Params       []byte
	Pipeline     string
	ControlPub   *trickle.TricklePublisher
	StopControl  func()
	ReportUpdate func([]byte)
	OutCond      *sync.Cond
	OutWriter    *media.RingBuffer
	Closed       bool

	DataWriter *media.SegmentWriter

	streamCtx     context.Context
	streamCancel  context.CancelCauseFunc
	streamParams  byocAIRequestParams
	streamRequest []byte
}

// Core job request structures
type JobRequest struct {
	ID            string `json:"id"`
	Request       string `json:"request"`
	Parameters    string `json:"parameters"` // additional information for the Gateway to use to select orchestrators or to send to the worker
	Capability    string `json:"capability"`
	CapabilityUrl string `json:"capability_url"` // this is set when verified orch as capability
	Sender        string `json:"sender"`
	Sig           string `json:"sig"`
	Timeout       int    `json:"timeout_seconds"`

	OrchSearchTimeout     time.Duration
	OrchSearchRespTimeout time.Duration
}

type JobRequestDetails struct {
	StreamId string `json:"stream_id"`
}

type JobParameters struct {
	// Gateway
	Orchestrators JobOrchestratorsFilter `json:"orchestrators,omitempty"` // list of orchestrators to use for the job

	// Orchestrator
	EnableVideoIngress bool `json:"enable_video_ingress,omitempty"`
	EnableVideoEgress  bool `json:"enable_video_egress,omitempty"`
	EnableDataOutput   bool `json:"enable_data_output,omitempty"`
}

type JobOrchestratorsFilter struct {
	Exclude []string `json:"exclude,omitempty"`
	Include []string `json:"include,omitempty"`
}

type JobToken struct {
	SenderAddress     *JobSender        `json:"sender_address,omitempty"`
	TicketParams      *net.TicketParams `json:"ticket_params,omitempty"`
	Balance           int64             `json:"balance,omitempty"`
	Price             *net.PriceInfo    `json:"price,omitempty"`
	ServiceAddr       string            `json:"service_addr,omitempty"`
	AvailableCapacity int64             `json:"available_capacity,omitempty"`

	LastNonce uint32
}

func (jt JobToken) Address() string {
	if jt.TicketParams != nil {
		if jt.TicketParams.Recipient != nil {
			return hexutil.Encode(jt.TicketParams.Recipient)
		}
	}
	return ""
}

func (jt JobToken) URL() string {
	return jt.ServiceAddr
}

type JobSender struct {
	Addr string `json:"addr"`
	Sig  string `json:"sig"`
}

// Internal structs used by the job processing pipeline
type orchJob struct {
	Req     *JobRequest
	Details *JobRequestDetails
	Params  *JobParameters

	// Orchestrator fields
	Sender   ethcommon.Address
	JobPrice *net.PriceInfo
}

type StartRequest struct {
	Stream     string `json:"stream_name"`
	RtmpOutput string `json:"rtmp_output"`
	StreamId   string `json:"stream_id"`
	Params     string `json:"params"`
}

type StreamUrls struct {
	StreamId      string `json:"stream_id"`
	WhipUrl       string `json:"whip_url"`
	WhepUrl       string `json:"whep_url"`
	RtmpUrl       string `json:"rtmp_url"`
	RtmpOutputUrl string `json:"rtmp_output_url"`
	UpdateUrl     string `json:"update_url"`
	StatusUrl     string `json:"status_url"`
	DataUrl       string `json:"data_url"`
	StopUrl       string `json:"stop_url"`
}

type orchTrickleUrls struct {
	orchPublishUrl   string
	orchSubscribeUrl string
	orchControlUrl   string
	orchEventsUrl    string
	orchDataUrl      string
}

type gatewayJob struct {
	Job          *orchJob
	Orchs        []JobToken
	SignedJobReq string

	node *core.LivepeerNode
}

var tlsConfig = &tls.Config{InsecureSkipVerify: true}
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: tlsConfig,
		DialTLSContext: func(ctx context.Context, network, addr string) (gonet.Conn, error) {
			cctx, cancel := context.WithTimeout(ctx, common.HTTPDialTimeout)
			defer cancel()

			tlsDialer := &tls.Dialer{Config: tlsConfig}
			return tlsDialer.DialContext(cctx, network, addr)
		},
		// Required for the transport to try to upgrade to HTTP/2 if TLSClientConfig is non-nil or
		// if custom dialers (i.e. via DialTLSContext) are used. This allows us to by default
		// transparently support HTTP/2 while maintaining the flexibility to use HTTP/1 by running
		// with GODEBUG=http2client=0
		ForceAttemptHTTP2: true,

		// Close the underlying connection if unused; otherwise they hang open for a long time
		IdleConnTimeout: 1 * time.Minute,
	},
	// Don't set a timeout here; pass a context to the request
}

type byocAIRequestParams struct {
	node *core.LivepeerNode
	os   drivers.OSSession

	liveParams *byocLiveRequestParams

	inputStreamExists func(streamId string) bool
}

// For live video pipelines
type byocLiveRequestParams struct {
	segmentReader *media.SwitchableSegmentReader
	dataWriter    *media.SegmentWriter
	stream        string
	requestID     string
	streamID      string
	manifestID    string
	pipelineID    string
	pipeline      string
	orchestrator  string

	paymentProcessInterval time.Duration
	outSegmentTimeout      time.Duration

	// list of RTMP output destinations
	rtmpOutputs []string
	// prefix to identify local (MediaMTX) RTMP hosts
	localRTMPPrefix string

	// Stops the pipeline with an error. Also kicks the input
	kickInput func(error)
	// Cancels the execution for the given Orchestrator session
	kickOrch context.CancelCauseFunc

	// Report an error event
	sendErrorEvent func(error)

	// Current orchestrator token (for reference to address, url, price, etc.)
	orchToken JobToken

	// State for the stream processing
	// startTime is the time when the first request is sent to the orchestrator
	startTime time.Time

	// Everything below needs to be protected by `mu` for concurrent modification + access
	mu sync.Mutex

	// when the write for the last segment started
	lastSegmentTime time.Time
}
