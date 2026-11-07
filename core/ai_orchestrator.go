package core

// ai_orchestrator.go implements logic for managing AI workers and processing AI jobs.

import (
	"context"
	"errors"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-tools/drivers"
)

var ErrNoWorkersAvailable = errors.New("no workers available")

var aiWorkerResultsTimeout = 10 * time.Minute

// CheckAICapacity verifies if the orchestrator can process a request for a specific pipeline and modelID.
func (orch *orchestrator) CheckAICapacity(pipeline, modelID string) (bool, chan<- bool) {
	if orch.node.AIWorker == nil {
		return false, nil
	}

	// confirm local worker has capacity
	if pipeline == "live-video-to-video" {
		return orch.node.AIWorker.HasCapacity(pipeline, modelID), nil
	}

	// other pipelines manage the capacity at the Orchestrator level to manage local ai-worker capacity
	if err := orch.node.ReserveAICapability(pipeline, modelID); err != nil {
		return false, nil
	}

	// reserve AI capacity for the pipeline and modelID
	releaseCapacity := make(chan bool)

	go func() {
		<-releaseCapacity
		orch.node.ReleaseAICapability(pipeline, modelID)
		glog.Infof("Released AI capacity for pipeline=%s model_id=%s", pipeline, modelID)
		close(releaseCapacity)

	}()

	return true, releaseCapacity

}

func (orch *orchestrator) GetLiveAICapacity(pipeline, modelID string) worker.Capacity {
	return orch.node.AIWorker.GetLiveAICapacity(pipeline, modelID)
}

func (orch *orchestrator) WorkerHardware() []worker.HardwareInformation {
	if orch.node.AIWorker == nil {
		return nil
	}
	return orch.node.AIWorker.HardwareInformation()
}

func (n *LivepeerNode) saveLocalAIWorkerResults(ctx context.Context, results interface{}, requestID string, contentType string) (interface{}, error) {
	if _, exists := n.StorageConfigs[requestID]; !exists {
		return nil, errors.New("no storage available for request")
	}

	// live-video-to-video responses carry no binary payload to persist
	return results, nil
}

func (orch *orchestrator) LiveVideoToVideo(ctx context.Context, requestID string, req worker.GenLiveVideoToVideoJSONRequestBody) (interface{}, error) {
	if orch.node.AIWorker == nil {
		return nil, ErrNoWorkersAvailable
	}

	workerResp, err := orch.node.LiveVideoToVideo(ctx, req)
	if err != nil {
		clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "live-video-to-video", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return orch.node.saveLocalAIWorkerResults(ctx, *workerResp, requestID, "application/json")
}

func (o *orchestrator) CreateStorageForRequest(requestID string) error {
	return o.node.createStorageForRequest(requestID)
}

func (n *LivepeerNode) createStorageForRequest(requestID string) error {
	n.storageMutex.Lock()
	defer n.storageMutex.Unlock()
	_, exists := n.StorageConfigs[requestID]
	if !exists {
		os := drivers.NodeStorage.NewSession(requestID)
		n.StorageConfigs[requestID] = &transcodeConfig{OS: os, LocalOS: os}
		// TODO: Figure out a better way to end the OS session after a timeout than creating a new goroutine per request?
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), aiWorkerResultsTimeout)
			defer cancel()
			<-ctx.Done()
			os.EndSession()
			clog.Infof(ctx, "Ended session for requestID=%v", requestID)
		}()
	}

	return nil
}

/*
 * Methods used to process AI job requests on a AI Worker.
 */

func (n *LivepeerNode) LiveVideoToVideo(ctx context.Context, req worker.GenLiveVideoToVideoJSONRequestBody) (*worker.LiveVideoToVideoResponse, error) {
	return n.AIWorker.LiveVideoToVideo(ctx, req)
}
