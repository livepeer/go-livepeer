package main

import (
	"bufio"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpGet returns the response body whatever the status, so a handler that reports an
// error with a 500 and a plain-text body looks to the caller like a successful read. The
// reward caller wizard offers to unset the value it reads back, so it must be able to tell
// an error body from an address.
func TestHttpGetWithSuccess(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantBody string
		wantOk   bool
	}{
		{
			name:     "ok with a value",
			status:   http.StatusOK,
			body:     "0x2222222222222222222222222222222222222222",
			wantBody: "0x2222222222222222222222222222222222222222",
			wantOk:   true,
		},
		{
			name:   "ok with an empty body",
			status: http.StatusOK,
			wantOk: true,
		},
		{
			name:     "server error carrying an error string",
			status:   http.StatusInternalServerError,
			body:     "Error getting reward caller: execution reverted",
			wantBody: "Error getting reward caller: execution reverted",
			wantOk:   false,
		},
		{
			name:   "bad request",
			status: http.StatusBadRequest,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			body, ok := httpGetWithSuccess(srv.URL)
			assert.Equal(t, tt.wantBody, body)
			assert.Equal(t, tt.wantOk, ok)
		})
	}
}

// httpGet keeps its existing shape for the callers that do not check the status.
func TestHttpGetUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	body, ok := httpGetWithSuccess(url)
	assert.Equal(t, "", body)
	assert.False(t, ok)
	assert.Equal(t, "", httpGet(url))
}

func newTestWizard(input string) *wizard {
	return &wizard{in: bufio.NewReader(strings.NewReader(input))}
}

// newTestNode starts a stub of the node's CLI HTTP API and returns a wizard
// pointing at it along with the form values it received.
func newTestNode(t *testing.T, input string, routes map[string]string) (*wizard, map[string]url.Values) {
	posted := make(map[string]url.Values)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// t.Errorf, not require: this runs on the server goroutine, where
			// FailNow is not allowed
			if err := r.ParseForm(); err != nil {
				t.Errorf("parsing form for %v: %v", r.URL.Path, err)
				return
			}
			posted[r.URL.Path] = r.PostForm
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	host, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	require.NoError(t, err)

	w := newTestWizard(input)
	w.host = host
	w.httpPort = port

	return w, posted
}

func TestReadPositiveBaseAmount(t *testing.T) {
	assert := assert.New(t)

	// a whole number of tokens
	assert.Equal("5000000000000000000", newTestWizard("5\n").readPositiveBaseAmount().String())

	// a fractional amount
	assert.Equal("1500000000000000000", newTestWizard("1.5\n").readPositiveBaseAmount().String())

	// an explicit sign
	assert.Equal("1500000000000000000", newTestWizard("+1.5\n").readPositiveBaseAmount().String())

	// the smallest unit
	assert.Equal("1", newTestWizard("0.000000000000000001\n").readPositiveBaseAmount().String())

	// zero, which the bond flow uses to skip bonding
	assert.Equal("0", newTestWizard("0\n").readPositiveBaseAmount().String())

	// a negative amount is rejected and the wizard asks again
	assert.Equal("2000000000000000000", newTestWizard("-1\n2\n").readPositiveBaseAmount().String())

	// so is an amount with more than 18 decimals
	assert.Equal("500000000000000000", newTestWizard("0.1111111111111111111\n0.5\n").readPositiveBaseAmount().String())

	// and so is anything that is not a number
	assert.Equal("3000000000000000000", newTestWizard("abc\n3\n").readPositiveBaseAmount().String())
}

func TestBondAmountIsReadAsLPT(t *testing.T) {
	toAddr := "0x0000000000000000000000000000000000000001"
	// no orchestrator list, so the wizard asks for an address, then 1.5 LPT out
	// of a 10 LPT balance
	w, posted := newTestNode(t, toAddr+"\n1.5\n", map[string]string{
		"/tokenBalance": "10000000000000000000",
	})

	w.bond()

	require.Contains(t, posted, "/bond")
	assert.Equal(t, "1500000000000000000", posted["/bond"].Get("amount"))
	assert.Equal(t, toAddr, posted["/bond"].Get("toAddr"))
}

func TestUnbondAmountIsReadAsLPT(t *testing.T) {
	dInfo, err := json.Marshal(lpTypes.Delegator{
		Address:         ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
		DelegateAddress: ethcommon.HexToAddress("0x0000000000000000000000000000000000000001"),
		BondedAmount:    new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)),
	})
	require.NoError(t, err)

	// not a full unbond, then 0.5 LPT out of the 10 LPT bonded
	w, posted := newTestNode(t, "n\n0.5\n", map[string]string{
		"/delegatorInfo": string(dInfo),
	})

	w.unbond()

	require.Contains(t, posted, "/unbond")
	assert.Equal(t, "500000000000000000", posted["/unbond"].Get("amount"))
}

func TestUnbondRefusesWhenNotBonded(t *testing.T) {
	dInfo, err := json.Marshal(lpTypes.Delegator{
		Address:      ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
		BondedAmount: big.NewInt(0),
	})
	require.NoError(t, err)

	// stdin is empty: nothing bonded means unbond has to bail out before it asks
	// anything, or it would loop on a condition it can never satisfy
	w, posted := newTestNode(t, "", map[string]string{
		"/delegatorInfo": string(dInfo),
	})

	w.unbond()

	assert.NotContains(t, posted, "/unbond")
}

func TestTransferTokensAmountIsReadAsLPT(t *testing.T) {
	toAddr := "0x0000000000000000000000000000000000000001"
	w, posted := newTestNode(t, toAddr+"\n2.5\ny\n", map[string]string{
		"/tokenBalance": "10000000000000000000",
	})

	w.transferTokens()

	require.Contains(t, posted, "/transferTokens")
	assert.Equal(t, "2500000000000000000", posted["/transferTokens"].Get("amount"))
	assert.Equal(t, toAddr, posted["/transferTokens"].Get("to"))
}

// orchestratorInfo has to carry a service URI and non-nil cuts: promptOrchestratorConfig
// dereferences the orchestrator either way, and falls back to an external IP lookup when
// the service URI is empty.
func orchestratorInfoJSON(t *testing.T) string {
	body, err := json.Marshal(struct {
		Transcoder *lpTypes.Transcoder
		PriceInfo  *big.Rat
	}{
		Transcoder: &lpTypes.Transcoder{
			Address:    ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
			ServiceURI: "https://127.0.0.1:8935",
			RewardCut:  big.NewInt(0),
			FeeShare:   big.NewInt(0),
		},
	})
	require.NoError(t, err)

	return string(body)
}

// the six empty lines take the default for every orchestrator config prompt: reward cut,
// fee cut, pixels per unit, currency, price per unit, and the service URI
const orchestratorConfigDefaults = "\n\n\n\n\n\n"

func TestActivateOrchestratorBondAmountIsReadAsLPT(t *testing.T) {
	dInfo, err := json.Marshal(lpTypes.Delegator{
		Address:      ethcommon.HexToAddress("0x0000000000000000000000000000000000000002"),
		BondedAmount: big.NewInt(0),
	})
	require.NoError(t, err)

	// nothing bonded, so the wizard asks for a self-bond after the config prompts
	w, posted := newTestNode(t, orchestratorConfigDefaults+"1.5\n", map[string]string{
		"/delegatorInfo":    string(dInfo),
		"/orchestratorInfo": orchestratorInfoJSON(t),
		"/unbondingLocks":   "[]",
		"/tokenBalance":     "10000000000000000000",
	})

	w.activateOrchestrator()

	require.Contains(t, posted, "/activateOrchestrator")
	assert.Equal(t, "1500000000000000000", posted["/activateOrchestrator"].Get("amount"))
	assert.Equal(t, "https://127.0.0.1:8935", posted["/activateOrchestrator"].Get("serviceURI"))
}

func TestSetOrchestratorConfigReportsBalance(t *testing.T) {
	w, posted := newTestNode(t, orchestratorConfigDefaults, map[string]string{
		"/orchestratorInfo": orchestratorInfoJSON(t),
		"/tokenBalance":     "10000000000000000000",
	})

	w.setOrchestratorConfig()

	require.Contains(t, posted, "/setOrchestratorConfig")
	assert.Equal(t, "https://127.0.0.1:8935", posted["/setOrchestratorConfig"].Get("serviceURI"))
}

func TestFormattedTokenBalanceKeepsWhatTheNodeSaid(t *testing.T) {
	// the node reports errors as a plain body, so a balance that is not a number has to
	// come through as it is rather than as a 0 LPT the operator would believe
	w, _ := newTestNode(t, "", map[string]string{
		"/tokenBalance": "Error getting token balance: execution reverted",
	})
	assert.Equal(t, "Error getting token balance: execution reverted", w.getFormattedTokenBalance())

	w, _ = newTestNode(t, "", map[string]string{"/tokenBalance": "10000000000000000000"})
	assert.Equal(t, "10 LPT", w.getFormattedTokenBalance())
}
