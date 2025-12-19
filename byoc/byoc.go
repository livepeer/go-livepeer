package byoc

import (
	"net/http"
	"sync"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
)

// BYOCServer orchestrates the BYOC handlers and registers routes

type BYOCOrchestratorServer struct {
	node            *core.LivepeerNode
	orch            Orchestrator
	trickleSrv      *trickle.Server
	trickleBasePath string

	httpMux *http.ServeMux

	sharedBalMtx *sync.Mutex
}

type BYOCGatewayServer struct {
	node       *core.LivepeerNode
	httpMux    *http.ServeMux
	whipServer *media.WHIPServer
	whepServer *media.WHEPServer

	statusStore StatusStore

	mu           *sync.RWMutex
	sharedBalMtx *sync.Mutex
}

// NewBYOCServer creates a new BYOC server instance
func NewBYOCGatewayServer(node *core.LivepeerNode, statusStore StatusStore, whipServer *media.WHIPServer, whepServer *media.WHEPServer, mux *http.ServeMux) *BYOCGatewayServer {
	bsg := &BYOCGatewayServer{
		node:         node,
		httpMux:      mux,
		statusStore:  statusStore,
		whipServer:   whipServer,
		whepServer:   whepServer,
		mu:           &sync.RWMutex{},
		sharedBalMtx: &sync.Mutex{},
	}

	bsg.registerRoutes()
	return bsg
}

func NewBYOCOrchestratorServer(node *core.LivepeerNode, orch Orchestrator, trickleSrv *trickle.Server, trickleBasePath string, mux *http.ServeMux) *BYOCOrchestratorServer {
	bso := &BYOCOrchestratorServer{
		node:            node,
		orch:            orch,
		trickleSrv:      trickleSrv,
		trickleBasePath: trickleBasePath,
		httpMux:         mux,
		sharedBalMtx:    &sync.Mutex{},
	}

	bso.registerRoutes()
	return bso
}

func (bsg *BYOCGatewayServer) Node() *core.LivepeerNode {
	return bsg.node
}
func (bsg *BYOCGatewayServer) SharedBalanceLock() *sync.Mutex {
	return bsg.sharedBalMtx
}

// registerRoutes registers all BYOC related routes
func (bsg *BYOCGatewayServer) registerRoutes() {
	//TODO: add /ai/stream routes

	//TODO: add WHEP support

	// Job submission routes
	bsg.httpMux.Handle("/process/request/", bsg.SubmitJob())
}

// withCORS adds CORS headers to responses
func (bs *BYOCGatewayServer) withCORS(statusCode int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, r.Method)
		w.WriteHeader(statusCode)
	})
}

func (bso *BYOCOrchestratorServer) Node() *core.LivepeerNode {
	return bso.node
}
func (bso *BYOCOrchestratorServer) SharedBalanceLock() *sync.Mutex {
	return bso.sharedBalMtx
}

func (bso *BYOCOrchestratorServer) registerRoutes() {
	// Job submission routes
	bso.httpMux.Handle("/process/request/", bso.ProcessJob())
	bso.httpMux.Handle("/process/token", bso.GetJobToken())
	bso.httpMux.Handle("/capability/register", bso.RegisterCapability())
	bso.httpMux.Handle("/capability/unregister", bso.UnregisterCapability())
}
