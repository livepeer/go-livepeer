package byoc

// NOTE: with this exception of the sign() function for gatewayJob, all methods in this file are
// duplicated from server package with minimal changes to be used with byoc package.

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	gonet "net"
	"net/http"
	url2 "net/url"
	"os"
	"strings"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/pkg/errors"
)

var sendJobReqWithTimeout = sendReqWithTimeout

func (g *gatewayJob) sign() error {
	//sign the request
	gateway := g.node.OrchestratorPool.Broadcaster()
	sig, err := gateway.Sign([]byte(g.Job.Req.Request + g.Job.Req.Parameters))
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to sign request err=%v", err))
	}
	g.Job.Req.Sender = gateway.Address().Hex()
	g.Job.Req.Sig = "0x" + hex.EncodeToString(sig)

	//create the job request header with the signature
	jobReqEncoded, err := json.Marshal(g.Job.Req)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to encode job request err=%v", err))
	}
	g.SignedJobReq = base64.StdEncoding.EncodeToString(jobReqEncoded)

	return nil
}

func parseJobRequest(jobReq string) (*JobRequest, error) {
	buf, err := base64.StdEncoding.DecodeString(jobReq)
	if err != nil {
		glog.Error("Unable to base64-decode ", err)
		return nil, errSegEncoding
	}

	var jobData JobRequest
	err = json.Unmarshal(buf, &jobData)
	if err != nil {
		glog.Error("Unable to unmarshal ", err)
		return nil, err
	}

	if jobData.Timeout == 0 {
		return nil, errNoTimeoutSet
	}

	return &jobData, nil
}

func getRemoteAddr(r *http.Request) string {
	addr := r.RemoteAddr
	if proxiedAddr := r.Header.Get("X-Forwarded-For"); proxiedAddr != "" {
		addr = strings.Split(proxiedAddr, ",")[0]
	}

	// addr is typically in the format "ip:port"
	// Need to extract just the IP. Handle IPv6 too.
	host, _, err := gonet.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		// probably not a real IP
		return addr
	}

	return host
}

func parseBalance(line string) *big.Rat {
	balanceStr := strings.Replace(line, "balance:", "", 1)
	balanceStr = strings.Replace(line, "data:", "", 1)
	balanceStr = strings.TrimSpace(balanceStr)

	balance, ok := new(big.Rat).SetString(balanceStr)
	if ok {
		return balance
	} else {
		return big.NewRat(0, 1)
	}
}

func getRemoteHost(remoteAddr string) (string, error) {
	if remoteAddr == "" {
		return "", errors.New("remoteAddr is empty")
	}

	// Handle IPv6 addresses by splitting on last colon
	host, _, err := gonet.SplitHostPort(remoteAddr)
	if err != nil {
		return "", fmt.Errorf("couldn't parse remote host: %w", err)
	}

	// Clean up IPv6 brackets if present
	host = strings.Trim(host, "[]")

	if host == "::1" {
		return "127.0.0.1", nil
	}

	return host, nil
}

func sendReqWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	timeouter := time.AfterFunc(timeout, cancel)

	req = req.WithContext(ctx)
	resp, err := httpClient.Do(req)
	if timeouter.Stop() {
		return resp, err
	}
	// timeout has already fired and cancelled the request
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return nil, context.DeadlineExceeded
}

func getPayment(header string) (net.Payment, error) {
	buf, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		return net.Payment{}, errors.Wrap(err, "base64 decode error")
	}
	var payment net.Payment
	if err := proto.Unmarshal(buf, &payment); err != nil {
		return net.Payment{}, errors.Wrap(err, "protobuf unmarshal error")
	}

	return payment, nil
}

func pmTicketParams(params *net.TicketParams) *pm.TicketParams {
	if params == nil {
		return nil
	}

	return &pm.TicketParams{
		Recipient:         ethcommon.BytesToAddress(params.Recipient),
		FaceValue:         new(big.Int).SetBytes(params.FaceValue),
		WinProb:           new(big.Int).SetBytes(params.WinProb),
		RecipientRandHash: ethcommon.BytesToHash(params.RecipientRandHash),
		Seed:              new(big.Int).SetBytes(params.Seed),
		ExpirationBlock:   new(big.Int).SetBytes(params.ExpirationBlock),
		ExpirationParams: &pm.TicketExpirationParams{
			CreationRound:          params.ExpirationParams.GetCreationRound(),
			CreationRoundBlockHash: ethcommon.BytesToHash(params.ExpirationParams.GetCreationRoundBlockHash()),
		},
	}
}

func corsHeaders(w http.ResponseWriter, reqMethod string) {
	if os.Getenv("LIVE_AI_ALLOW_CORS") == "" { // TODO cli arg
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if reqMethod == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST, GET")
		// Allows us send down a preferred STUN server without ICE restart
		// https://datatracker.ietf.org/doc/html/draft-ietf-wish-whip-16#section-4.6
		w.Header()["Link"] = media.GenICELinkHeaders(media.WebrtcConfig.ICEServers)
	}
}

func generateWhepUrl(streamName, requestID string) string {
	whepURL := os.Getenv("LIVE_AI_WHEP_URL")
	if whepURL == "" {
		whepURL = "http://localhost:8889/" // default mediamtx output
	}
	whepURL = fmt.Sprintf("%s%s-%s-out/whep", whepURL, streamName, requestID)
	return whepURL
}

func respondJsonError(ctx context.Context, w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(err.Error()))
}

func respondWithError(w http.ResponseWriter, errMsg string, code int) {
	glog.Errorf("HTTP Response Error statusCode=%d err=%v", code, errMsg)
	http.Error(w, errMsg, code)
}

func overwriteHost(hostOverwrite, url string) string {
	if hostOverwrite == "" {
		return url
	}
	u, err := url2.ParseRequestURI(url)
	if err != nil {
		slog.Warn("Couldn't parse url to overwrite for worker, using original url", "url", url, "err", err)
		return url
	}
	u.Host = hostOverwrite
	return u.String()
}

// Detect 'slow' orchs by keeping track of in-flight segments
// Count the difference between segments produced and segments completed
type SlowOrchChecker struct {
	mu            sync.Mutex
	segmentCount  int
	completeCount int
}

// Number of in flight segments to allow.
// Should generally not be less than 1, because
// sometimes the beginning of the current segment
// may briefly overlap with the end of the previous segment
const maxInflightSegments = 3

// Returns the number of segments begun so far and
// whether the max number of inflight segments was hit.
// Number of segments is not incremented if inflight max is hit.
// If inflight max is hit, returns true, false otherwise.
func (s *SlowOrchChecker) BeginSegment() (int, bool) {
	// Returns `false` if there are multiple segments in-flight
	// this means the orchestrator is slow reading them
	// If all-OK, returns `true`
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.segmentCount >= s.completeCount+maxInflightSegments {
		// There is > 1 segment in flight ... orchestrator is slow reading
		return s.segmentCount, true
	}
	s.segmentCount += 1
	return s.segmentCount, false
}

func (s *SlowOrchChecker) EndSegment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCount += 1
}

func (s *SlowOrchChecker) GetCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.segmentCount
}
