package core

// ai_orchestrator.go implements logic for managing AI workers and processing AI jobs.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
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

// only used for sending work to remote AI worker
func (orch *orchestrator) SaveAIRequestInput(ctx context.Context, requestID string, fileData []byte) (string, error) {
	node := orch.node
	if drivers.NodeStorage == nil {
		return "", fmt.Errorf("Missing local storage")
	}

	storage, exists := node.StorageConfigs[requestID]
	if !exists {
		return "", errors.New("storage does not exist for request")
	}

	url, err := storage.OS.SaveData(ctx, string(RandomManifestID())+".tempfile", bytes.NewReader(fileData), nil, 0)
	if err != nil {
		return "", err
	}

	return url, nil
}

func (o *orchestrator) GetStorageForRequest(requestID string) (drivers.OSSession, bool) {
	session, exists := o.node.getStorageForRequest(requestID)
	if exists {
		return session, true
	} else {
		return nil, false
	}
}

func (n *LivepeerNode) getStorageForRequest(requestID string) (drivers.OSSession, bool) {
	session, exists := n.StorageConfigs[requestID]
	return session.OS, exists
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

func (orch *orchestrator) RegisterExternalCapability(extCapabilitySettings string) (*ExternalCapability, error) {
	cap, err := orch.node.ExternalCapabilities.RegisterCapability(extCapabilitySettings)
	if err != nil {
		return nil, err
	}

	//set the price for the capability
	orch.node.SetPriceForExternalCapability("default", cap.Name, cap.GetPrice())

	return cap, nil
}

func (orch *orchestrator) RemoveExternalCapability(extCapability string) error {
	orch.node.ExternalCapabilities.RemoveCapability(extCapability)
	return nil
}

func (orch *orchestrator) GetUrlForCapability(extCapability string) string {
	for _, capability := range orch.node.ExternalCapabilities.Capabilities {
		if capability.Name == extCapability {
			return capability.Url
		}
	}

	return ""
}

func (orch *orchestrator) CheckExternalCapabilityCapacity(extCapability string) int64 {
	if cap, ok := orch.node.ExternalCapabilities.Capabilities[extCapability]; !ok {
		return 0
	} else {
		if cap.Load < cap.Capacity {
			return int64(cap.Capacity - cap.Load)
		} else {
			return 0
		}
	}
}

func (orch *orchestrator) ReserveExternalCapabilityCapacity(extCapability string) error {
	cap, ok := orch.node.ExternalCapabilities.Capabilities[extCapability]
	if ok {
		cap.Mu.Lock()
		defer cap.Mu.Unlock()

		cap.Load++
		return nil
	} else {
		return errors.New("external capability not found")
	}
}

func (orch *orchestrator) FreeExternalCapabilityCapacity(extCapability string) error {
	cap, ok := orch.node.ExternalCapabilities.Capabilities[extCapability]
	if ok {
		cap.Mu.Lock()
		defer cap.Mu.Unlock()

		cap.Load--
		return nil
	} else {
		return errors.New("external capability not found")
	}
}

func (orch *orchestrator) JobPriceInfo(sender ethcommon.Address, jobCapability string) (*net.PriceInfo, error) {
	if orch.node == nil || orch.node.Recipient == nil {
		//return a price of zero for offhain mode
		return &net.PriceInfo{
			PricePerUnit:  0,
			PixelsPerUnit: 1,
		}, nil
	}

	jobPrice, err := orch.jobPriceInfo(sender, jobCapability)
	if err != nil {
		return nil, err
	}

	//ensure price numerator and denominator can be int64
	jobPrice, err = common.PriceToInt64(jobPrice)
	if err != nil {
		return nil, fmt.Errorf("invalid job price: %w", err)
	}

	return &net.PriceInfo{
		PricePerUnit:  jobPrice.Num().Int64(),
		PixelsPerUnit: jobPrice.Denom().Int64(),
	}, nil
}

func (orch *orchestrator) jobPriceInfo(sender ethcommon.Address, jobCapability string) (*big.Rat, error) {
	basePrice := orch.node.GetPriceForJob(sender.Hex(), jobCapability)

	if basePrice == nil {
		basePrice = orch.node.GetPriceForJob("default", jobCapability)
	}

	if !orch.node.AutoAdjustPrice {
		return basePrice, nil
	}

	// If price = 0, overhead is 1
	// If price > 0, overhead = 1 + (1 / txCostMultiplier)
	overhead := big.NewRat(1, 1)
	if basePrice.Num().Cmp(big.NewInt(0)) > 0 {
		txCostMultiplier, err := orch.node.Recipient.TxCostMultiplier(sender)
		if err != nil {
			glog.Errorf("failed to get tx cost multiplier for sender %s: %v  (txCost=%v)", sender.Hex(), err)
			return nil, err
		}

		if txCostMultiplier.Cmp(big.NewRat(0, 1)) > 0 {
			overhead = overhead.Add(overhead, new(big.Rat).Inv(txCostMultiplier))
		}
	}

	// pricePerPixel = basePrice * overhead
	fixedPrice, err := common.PriceToFixed(new(big.Rat).Mul(basePrice, overhead))
	if err != nil {
		return nil, err
	}
	return common.FixedToPrice(fixedPrice), nil

}
