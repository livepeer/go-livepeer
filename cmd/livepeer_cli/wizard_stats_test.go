package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The node's signing wallet is the reward caller under LIP-118, so the row holding it must
// not be labelled as the orchestrator's account.
func TestAccountLabel(t *testing.T) {
	orch := "0x16a72bdb3017196825BC53809b87F96fbEE31F6C"
	caller := "0x1B0c26FC2E310eFB2649C24eC9AA966efFDA292F"

	tests := []struct {
		name           string
		isOrchestrator bool
		account        string
		want           string
	}{
		{"orchestrator on its own wallet", true, orch, "Orchestrator Account"},
		{"orchestrator delegating to a reward caller", true, caller, "Node Account"},
		{"gateway", false, caller, "Broadcaster Account"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/orchestratorInfo":
					fmt.Fprintf(w, `{"Transcoder":{"Address":%q,"Status":"Registered"},"PriceInfo":null}`, orch)
				case "/ethAddr":
					fmt.Fprint(w, tt.account)
				}
			}))
			defer srv.Close()

			u, err := url.Parse(srv.URL)
			require.NoError(t, err)
			w := &wizard{host: u.Hostname(), httpPort: u.Port()}

			assert.Equal(t, tt.want+"\nYOUR WALLET FOR ETH & LPT", w.accountLabel(tt.isOrchestrator))
		})
	}
}

// A failed lookup must not silently relabel the row.
func TestAccountLabelLookupFailure(t *testing.T) {
	w := &wizard{host: "127.0.0.1", httpPort: "1"}
	assert.Equal(t, "Orchestrator Account\nYOUR WALLET FOR ETH & LPT", w.accountLabel(true))
}
