package byoc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

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

	// WHEP playback support
	if bsg.whepServer != nil {
		bsg.httpMux.Handle("POST /process/stream/{streamId}/whep", bsg.StreamWhep())
		bsg.httpMux.Handle("OPTIONS /process/stream/{streamId}/whep", bsg.withCORS(http.StatusNoContent))
	}

	// Job submission routes for batch processing
	bsg.httpMux.Handle("/process/request/", bsg.SubmitJob())

	// Training routes (proxied to orchestrator)
	bsg.httpMux.Handle("/process/train/", bsg.SubmitTrainingJob())
	bsg.httpMux.Handle("GET /process/job/{jobId}", bsg.GetTrainingJobStatus())
}

// StreamWhep handles WHEP playback for BYOC streams.
// Looks up the stream's OutWriter (RingBuffer) and creates a WebRTC connection.
func (bsg *BYOCGatewayServer) StreamWhep() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamId := r.PathValue("streamId")
		corsHeaders(w, r.Method)

		stream, err := bsg.streamPipeline(streamId)
		if err != nil {
			http.Error(w, "Stream not found", http.StatusNotFound)
			return
		}

		// Wait for OutWriter to be available (blocks until trickle subscribe is set up)
		bsg.mu.RLock()
		outWriter := stream.OutWriter
		bsg.mu.RUnlock()

		if outWriter == nil {
			// Wait up to 10 seconds for OutWriter
			for i := 0; i < 100; i++ {
				stream.OutCond.L.Lock()
				if stream.OutWriter != nil {
					outWriter = stream.OutWriter
					stream.OutCond.L.Unlock()
					break
				}
				stream.OutCond.Wait()
				stream.OutCond.L.Unlock()
			}
		}

		if outWriter == nil {
			http.Error(w, "Stream output not ready", http.StatusServiceUnavailable)
			return
		}

		ctx := r.Context()
		bsg.whepServer.CreateWHEP(ctx, w, r, outWriter.MakeReader(), streamId)
	})
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

	// Training job store for async training jobs
	trainingStore *TrainingJobStore
}

func NewBYOCOrchestratorServer(node *core.LivepeerNode, orch Orchestrator, trickleSrv *trickle.Server, trickleBasePath string, mux *http.ServeMux) *BYOCOrchestratorServer {
	// PR-6: opt-in filesystem-backed training store via TRAINING_CHECKPOINT_DIR.
	// When unset, falls back to in-memory (current behavior, no recovery on
	// orch restart). When set, JSON checkpoints land in that directory and
	// startup-sweep recovers any in-flight jobs as failed_orchestrator_restart.
	checkpointDir := os.Getenv("TRAINING_CHECKPOINT_DIR")
	var store *TrainingJobStore
	if checkpointDir != "" {
		var err error
		store, err = NewTrainingJobStoreWithCheckpoint(24*time.Hour, checkpointDir)
		if err != nil {
			glog.Warningf("training-store: checkpoint init failed (%v), falling back to in-memory", err)
			store = NewTrainingJobStore(24 * time.Hour)
		} else {
			glog.Infof("training-store: persisting checkpoints to %s", checkpointDir)
		}
	} else {
		store = NewTrainingJobStore(24 * time.Hour)
	}

	bso := &BYOCOrchestratorServer{
		node:            node,
		orch:            orch,
		trickleSrv:      trickleSrv,
		trickleBasePath: trickleBasePath,
		httpMux:         mux,
		sharedBalMtx:    &sync.Mutex{},
		trainingStore:   store,
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
	// Training routes (async)
	bso.httpMux.Handle("/process/train/", bso.SubmitTrainingJob())
	bso.httpMux.Handle("GET /process/jobs", bso.ListTrainingJobs())
	bso.httpMux.Handle("GET /process/job/{jobId}", bso.GetTrainingJobStatus())
	bso.httpMux.Handle("POST /process/job/{jobId}/cancel", bso.CancelTrainingJob())
	// PR-5 (byoc-payment-fleet-2026-05): refresh-on-watermark endpoint.
	// SDK calls this when the in-flight job's balance approaches zero,
	// crediting fresh tickets without re-running verifyJobCreds (the job
	// is already authenticated by its submit handshake).
	bso.httpMux.Handle("POST /process/job/{jobId}/refresh-payment", bso.RefreshTrainingPayment())
	// PR-14 (byoc-payment-fleet-2026-05): payment-stats surface for the
	// storyboard /payments page. Returns the in-memory ring buffer of
	// recent BillingEvents as JSON.
	bso.httpMux.Handle("GET /admin/billing-events", bso.BillingEventsHandler())
	// Stream routes
	bso.httpMux.Handle("/ai/stream/start", bso.StartStream())
	bso.httpMux.Handle("/ai/stream/stop", bso.StopStream())
	bso.httpMux.Handle("/ai/stream/update", bso.UpdateStream())
	bso.httpMux.Handle("/ai/stream/payment", bso.ProcessStreamPayment())
}
