package byoc

import (
	"context"
	"encoding/base64"
	"math/big"
	gonet "net"
	"net/http"
	"os"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/pkg/errors"
)

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
