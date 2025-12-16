package byoc

import (
	"context"
	"crypto/tls"
	"errors"
	"math/big"
	gonet "net"
	"net/http"
	"net/url"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/net"
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

// Orchestrator is an interface for orchestrator operations
type Orchestrator interface {
	TranscoderSecret() string
	VerifySig(addr ethcommon.Address, msg string, sig []byte) bool
	ReserveExternalCapabilityCapacity(capability string) error
	GetUrlForCapability(capability string) string
	JobPriceInfo(sender ethcommon.Address, capability string) (*net.PriceInfo, error)
	TicketParams(sender ethcommon.Address, priceInfo *net.PriceInfo) (*net.TicketParams, error)
	Balance(sender ethcommon.Address, manifestID core.ManifestID) *big.Rat
	CheckExternalCapabilityCapacity(capability string) bool
	RemoveExternalCapability(extCapName string) error
	RegisterExternalCapability(extCapSettings string) (*core.ExternalCapability, error)
	FreeExternalCapabilityCapacity(capability string) error
	ServiceURI() *url.URL
	ProcessPayment(ctx context.Context, payment net.Payment, manifestID core.ManifestID) error
	DebitFees(sender ethcommon.Address, manifestID core.ManifestID, priceInfo *net.PriceInfo, units int64)
}

type StatusStore interface {
	Store(streamID string, status map[string]interface{})
	StoreKey(streamID, key string, status interface{})
	Clear(streamID string)
	Get(streamID string) (map[string]interface{}, bool)
	StoreIfNotExists(streamID string, key string, status interface{})
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
