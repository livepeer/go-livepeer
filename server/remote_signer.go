package server

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/core"
)

// SignOrchestratorInfo handles signing GetOrchestratorInfo requests for multiple orchestrators
func (ls *LivepeerServer) SignOrchestratorInfo(w http.ResponseWriter, r *http.Request) {
	ctx := clog.AddVal(r.Context(), "request_id", string(core.RandomManifestID()))
	remoteAddr := getRemoteAddr(r)
	clog.Info(ctx, "Orch info signature request", "ip", remoteAddr)

	// Get the broadcaster (signer)
	// In remote signer mode, we may not have an OrchestratorPool, so create a broadcaster directly
	gw := core.NewBroadcaster(ls.LivepeerNode)

	// Create empty params for signing
	params := GetOrchestratorInfoParams{}

	// Generate the request (this creates the signature)
	req, err := genOrchestratorReq(gw, params)
	if err != nil {
		clog.Errorf(ctx, "Failed to generate request: err=%q", err)
		respondJsonError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	// Extract signature and format as hex
	var (
		signature = "0x" + hex.EncodeToString(req.Sig)
		address   = gw.Address().String()
	)

	results := map[string]string{
		"address":   address,
		"signature": signature,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(results)
}

// StartRemoteSignerServer starts the HTTP server for remote signer mode
func StartRemoteSignerServer(ls *LivepeerServer, bind string) error {
	// Register the remote signer endpoint
	ls.HTTPMux.Handle("POST /sign-orchestrator-info", http.HandlerFunc(ls.SignOrchestratorInfo))

	// Start the HTTP server
	glog.Info("Starting Remote Signer server on ", bind)
	srv := http.Server{
		Addr:        bind,
		Handler:     ls.HTTPMux,
		IdleTimeout: HTTPIdleTimeout,
	}
	return srv.ListenAndServe()
}
