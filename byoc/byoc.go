package byoc

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
)

type BYOCGatewayServer struct {
	node       *core.LivepeerNode
	httpMux    *http.ServeMux
	whipServer *media.WHIPServer
	whepServer *media.WHEPServer

	statusStore StatusStore

	StreamPipelines map[string]*BYOCStreamPipeline
	mu              *sync.RWMutex
	sharedBalMtx    *sync.Mutex
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

	bsg.StreamPipelines = make(map[string]*BYOCStreamPipeline)

	bsg.registerRoutes()
	return bsg
}

func (bsg *BYOCGatewayServer) Node() *core.LivepeerNode {
	return bsg.node
}
func (bsg *BYOCGatewayServer) SharedBalanceLock() *sync.Mutex {
	return bsg.sharedBalMtx
}

func (bsg *BYOCGatewayServer) newStreamPipeline(requestID, streamID, pipeline string, streamParams byocAIRequestParams, streamRequest []byte) *BYOCStreamPipeline {
	streamCtx, streamCancel := context.WithCancelCause(context.Background())
	bsg.mu.Lock()
	defer bsg.mu.Unlock()

	//ensure streamRequest is not nil or empty to avoid json unmarshal issues on Orchestrator failover
	//sends the request bytes to next Orchestrator
	if len(streamRequest) == 0 {
		streamRequest = []byte("{}")
	}

	bsg.StreamPipelines[streamID] = &BYOCStreamPipeline{
		RequestID:     requestID,
		StreamID:      streamID,
		Pipeline:      pipeline,
		streamCtx:     streamCtx,
		streamParams:  streamParams,
		streamCancel:  streamCancel,
		streamRequest: streamRequest,
		OutCond:       sync.NewCond(bsg.mu),
	}
	return bsg.StreamPipelines[streamID]
}

func (bsg *BYOCGatewayServer) streamPipeline(streamId string) (*BYOCStreamPipeline, error) {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	p, exists := bsg.StreamPipelines[streamId]
	if !exists {
		return nil, fmt.Errorf("BYOC Stream pipeline %s not found", streamId)
	}
	return p, nil
}

func (bsg *BYOCGatewayServer) streamPipelineExists(streamId string) bool {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	_, exists := bsg.StreamPipelines[streamId]
	return exists
}

func (bsg *BYOCGatewayServer) stopStreamPipeline(streamId string, err error) {
	p, err := bsg.streamPipeline(streamId)
	if err == nil {
func (bsg *BYOCGatewayServer) stopStreamPipeline(streamId string, err error) {
	stream, serr := bsg.streamPipeline(streamId)
	if serr != nil {
		return
	}

	stream.OutCond.L.Lock()
	stream.Closed = true
	stream.OutCond.Broadcast()
	ctrlPub := stream.ControlPub
	stopCtrl := stream.StopControl
	stream.OutCond.L.Unlock()

	if ctrlPub != nil {
		if cerr := ctrlPub.Close(); cerr != nil {
			glog.Errorf("Error closing trickle publisher: %v", cerr)
		}
	}
	if stopCtrl != nil {
		stopCtrl()
	}
	stream.streamCancel(err)
}
		if p.ControlPub != nil {
			if err := p.ControlPub.Close(); err != nil {
				glog.Errorf("Error closing trickle publisher", err)
			}
			if p.StopControl != nil {
				p.StopControl()
			}
		}
		p.streamCancel(err)
		p.Closed = true
	}
}

func (bsg *BYOCGatewayServer) removeStreamPipeline(streamId string) {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	delete(bsg.StreamPipelines, streamId)
}

func (bsg *BYOCGatewayServer) streamPipelineParams(streamId string) (byocAIRequestParams, error) {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	p, exists := bsg.StreamPipelines[streamId]
	if !exists {
		return byocAIRequestParams{}, fmt.Errorf("BYOC Stream pipeline %s not found", streamId)
	}
	return p.streamParams, nil
}

func (bsg *BYOCGatewayServer) updateStreamPipelineParams(streamId string, newParams byocAIRequestParams) {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	p, exists := bsg.StreamPipelines[streamId]
	if exists {
		p.streamParams = newParams
	}
}

func (bsg *BYOCGatewayServer) streamPipelineContext(streamId string) context.Context {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	if p, exists := bsg.StreamPipelines[streamId]; exists {
		return p.streamCtx
	}
	return nil
}

func (bsg *BYOCGatewayServer) streamPipelineRequest(streamId string) []byte {
	bsg.mu.Lock()
	defer bsg.mu.Unlock()
	p, exists := bsg.StreamPipelines[streamId]
	if exists {
		return p.streamRequest
	}

	return nil
}

// registerRoutes registers all BYOC related routes
func (bsg *BYOCGatewayServer) registerRoutes() {
	// CORS preflight
	bsg.httpMux.Handle("OPTIONS /process/stream/", bsg.withCORS(http.StatusNoContent))

	// Stream routes
	bsg.httpMux.Handle("POST /process/stream/start", bsg.StartStream())
	bsg.httpMux.Handle("POST /process/stream/{streamId}/update", bsg.UpdateStream())
	bsg.httpMux.Handle("GET /process/stream/{streamId}/status", bsg.StreamStatus())
	bsg.httpMux.Handle("POST /process/stream/{streamId}/stop", bsg.StopStream())
	bsg.httpMux.Handle("GET /process/stream/{streamId}/data", bsg.StreamData())
	bsg.httpMux.Handle("POST /process/stream/{streamId}/rtmp", bsg.StartStreamRTMPIngest())
	if bsg.whipServer != nil {
		bsg.httpMux.Handle("POST /process/stream/{streamId}/whip", bsg.StartStreamWhipIngest(bsg.whipServer))
	}

	//TODO: add WHEP support

	// Job submission routes for batch processing
	bsg.httpMux.Handle("/process/request/", bsg.SubmitJob())
}

// withCORS adds CORS headers to responses
func (bs *BYOCGatewayServer) withCORS(statusCode int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, r.Method)
		w.WriteHeader(statusCode)
	})
}

type BYOCOrchestratorServer struct {
	node            *core.LivepeerNode
	orch            Orchestrator
	trickleSrv      *trickle.Server
	trickleBasePath string

	httpMux *http.ServeMux

	sharedBalMtx *sync.Mutex
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

func (bso *BYOCOrchestratorServer) Node() *core.LivepeerNode {
	return bso.node
}
func (bso *BYOCOrchestratorServer) SharedBalanceLock() *sync.Mutex {
	return bso.sharedBalMtx
}

func (bso *BYOCOrchestratorServer) registerRoutes() {
	// Job submission routes for batch processing
	bso.httpMux.Handle("/process/request/", bso.ProcessJob())
	bso.httpMux.Handle("/process/token", bso.GetJobToken())
	bso.httpMux.Handle("/capability/register", bso.RegisterCapability())
	bso.httpMux.Handle("/capability/unregister", bso.UnregisterCapability())
	// Stream routes
	bso.httpMux.Handle("/ai/stream/start", bso.StartStream())
	bso.httpMux.Handle("/ai/stream/stop", bso.StopStream())
	bso.httpMux.Handle("/ai/stream/update", bso.UpdateStream())
	bso.httpMux.Handle("/ai/stream/payment", bso.ProcessStreamPayment())
}
