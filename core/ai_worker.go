package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/glog"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/monitor"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
)

var ErrRemoteWorkerTimeout = errors.New("Remote worker took too long")
var ErrNoCompatibleWorkersAvailable = errors.New("no workers can process job requested")
var ErrNoWorkersAvailable = errors.New("no workers available")

// TODO: consider making this dynamic for each pipeline
var aiWorkerResultsTimeout = 10 * time.Minute
var aiWorkerRequestTimeout = 15 * time.Minute

type RemoteAIWorker struct {
	manager      *RemoteAIWorkerManager
	stream       net.AIWorker_RegisterAIWorkerServer
	capabilities *Capabilities
	eof          chan struct{}
	addr         string
}

func (rw *RemoteAIWorker) done() {
	// select so we don't block indefinitely if there's no listener
	select {
	case rw.eof <- struct{}{}:
	default:
	}
}

type RemoteAIWorkerManager struct {
	remoteAIWorkers []*RemoteAIWorker
	liveAIWorkers   map[net.AIWorker_RegisterAIWorkerServer]*RemoteAIWorker
	RWmutex         sync.Mutex

	// For tracking tasks assigned to remote aiworkers
	taskMutex *sync.RWMutex
	taskChans map[int64]AIWorkerChan
	taskCount int64

	// Map for keeping track of sessions and their respective aiworkers
	requestSessions map[string]*RemoteAIWorker
}

func NewRemoteAIWorker(m *RemoteAIWorkerManager, stream net.AIWorker_RegisterAIWorkerServer, caps *Capabilities) *RemoteAIWorker {
	return &RemoteAIWorker{
		manager:      m,
		stream:       stream,
		eof:          make(chan struct{}, 1),
		addr:         common.GetConnectionAddr(stream.Context()),
		capabilities: caps,
	}
}

func NewRemoteAIWorkerManager() *RemoteAIWorkerManager {
	return &RemoteAIWorkerManager{
		remoteAIWorkers: []*RemoteAIWorker{},
		liveAIWorkers:   map[net.AIWorker_RegisterAIWorkerServer]*RemoteAIWorker{},
		RWmutex:         sync.Mutex{},

		taskMutex: &sync.RWMutex{},
		taskChans: make(map[int64]AIWorkerChan),

		requestSessions: make(map[string]*RemoteAIWorker),
	}
}

func (orch *orchestrator) ServeAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	orch.node.serveAIWorker(stream, capabilities)
}

func (n *LivepeerNode) serveAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	wkrCaps := CapabilitiesFromNetCapabilities(capabilities)
	if n.Capabilities.LivepeerVersionCompatibleWith(capabilities) {
		glog.Infof("Worker compatible, connecting worker_version=%s orchestrator_version=%s worker_addr=%s", capabilities.Version, n.Capabilities.constraints.minVersion, from)
		n.Capabilities.AddCapacity(wkrCaps)
		n.AddAICapabilities(wkrCaps)
		defer n.Capabilities.RemoveCapacity(wkrCaps)
		defer n.RemoveAICapabilities(wkrCaps)

		// Manage blocks while AI worker is connected
		n.AIWorkerManager.Manage(stream, capabilities)
		glog.V(common.DEBUG).Infof("Closing aiworker=%s channel", from)
	} else {
		glog.Errorf("worker %s not connected, version not compatible", from)
	}
}

// Manage adds aiworker to list of live aiworkers. Doesn't return until aiworker disconnects
func (rwm *RemoteAIWorkerManager) Manage(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	aiworker := NewRemoteAIWorker(rwm, stream, CapabilitiesFromNetCapabilities(capabilities))
	go func() {
		ctx := stream.Context()
		<-ctx.Done()
		err := ctx.Err()
		glog.Errorf("Stream closed for aiworker=%s, err=%q", from, err)
		aiworker.done()
	}()

	rwm.RWmutex.Lock()
	rwm.liveAIWorkers[aiworker.stream] = aiworker
	rwm.remoteAIWorkers = append(rwm.remoteAIWorkers, aiworker)
	rwm.RWmutex.Unlock()

	<-aiworker.eof
	glog.Infof("Got aiworker=%s eof, removing from live aiworkers map", from)

	rwm.RWmutex.Lock()
	delete(rwm.liveAIWorkers, aiworker.stream)
	rwm.RWmutex.Unlock()
}

// RemoteAIworkerFatalError wraps error to indicate that error is fatal
type RemoteAIWorkerFatalError struct {
	error
}

// NewRemoteAIWorkerFatalError creates new RemoteAIWorkerFatalError
// Exported here to be used in other packages
func NewRemoteAIWorkerFatalError(err error) error {
	return RemoteAIWorkerFatalError{err}
}

// Process does actual AI job using remote worker from the pool
func (rwm *RemoteAIWorkerManager) Process(ctx context.Context, requestID string, pipeline string, modelID string, fname string, req AIJobRequestData) (*RemoteAIWorkerResult, error) {
	worker, err := rwm.selectWorker(requestID, pipeline, modelID)
	if err != nil {
		return nil, err
	}
	res, err := worker.Process(ctx, pipeline, modelID, fname, req)
	if err != nil {
		rwm.completeAIRequest(requestID, pipeline, modelID)
	}
	_, fatal := err.(RemoteAIWorkerFatalError)
	if fatal {
		// Don't retry if we've timed out; gateway likely to have moved on
		if err.(RemoteAIWorkerFatalError).error == ErrRemoteWorkerTimeout {
			return res, err
		}
		return rwm.Process(ctx, requestID, pipeline, modelID, fname, req)
	}

	rwm.completeAIRequest(requestID, pipeline, modelID)
	return res, err
}

func (rwm *RemoteAIWorkerManager) selectWorker(requestID string, pipeline string, modelID string) (*RemoteAIWorker, error) {
	rwm.RWmutex.Lock()
	defer rwm.RWmutex.Unlock()

	checkWorkers := func(rwm *RemoteAIWorkerManager) bool {
		return len(rwm.remoteAIWorkers) > 0
	}

	findCompatibleWorker := func(rwm *RemoteAIWorkerManager) int {
		cap, _ := PipelineToCapability(pipeline)
		for idx, worker := range rwm.remoteAIWorkers {
			rwCap, hasCap := worker.capabilities.constraints.perCapability[cap]
			if hasCap {
				_, hasModel := rwCap.Models[modelID]
				if hasModel {
					if rwCap.Models[modelID].Capacity > 0 {
						rwm.remoteAIWorkers[idx].capabilities.constraints.perCapability[cap].Models[modelID].Capacity -= 1
						return idx
					}
				}
			}
		}
		return -1
	}

	for checkWorkers(rwm) {
		worker, sessionExists := rwm.requestSessions[requestID]
		newWorker := findCompatibleWorker(rwm)
		if newWorker == -1 {
			return nil, ErrNoCompatibleWorkersAvailable
		}
		if !sessionExists {
			worker = rwm.remoteAIWorkers[newWorker]
		}

		if _, ok := rwm.liveAIWorkers[worker.stream]; !ok {
			// Remove the stream session because the worker is no longer live
			if sessionExists {
				rwm.completeAIRequest(requestID, pipeline, modelID)
			}
			// worker does not exist in table; remove and retry
			rwm.remoteAIWorkers = removeFromRemoteWorkers(worker, rwm.remoteAIWorkers)
			continue
		}

		if !sessionExists {
			// Assigning worker to session for future use
			rwm.requestSessions[requestID] = worker
		}
		return worker, nil
	}

	return nil, ErrNoWorkersAvailable
}

func (rwm *RemoteAIWorkerManager) workerHasCapacity(pipeline, modelID string) bool {
	cap, err := PipelineToCapability(pipeline)
	if err != nil {
		return false
	}
	for _, worker := range rwm.remoteAIWorkers {
		rw, hasCap := worker.capabilities.constraints.perCapability[cap]
		if hasCap {
			_, hasModel := rw.Models[modelID]
			if hasModel {
				if rw.Models[modelID].Capacity > 0 {
					return true
				}
			}
		}
	}
	// no worker has capacity
	return false
}

// completeRequestSessions end a AI request session for a remote ai worker
// caller should hold the mutex lock
func (rwm *RemoteAIWorkerManager) completeAIRequest(requestID, pipeline, modelID string) {
	rwm.RWmutex.Lock()
	defer rwm.RWmutex.Unlock()

	worker, ok := rwm.requestSessions[requestID]
	if !ok {
		return
	}

	for idx, remoteWorker := range rwm.remoteAIWorkers {
		if worker.addr == remoteWorker.addr {
			cap, err := PipelineToCapability(pipeline)
			if err == nil {
				if _, hasCap := rwm.remoteAIWorkers[idx].capabilities.constraints.perCapability[cap]; hasCap {
					if _, hasModel := rwm.remoteAIWorkers[idx].capabilities.constraints.perCapability[cap].Models[modelID]; hasModel {
						rwm.remoteAIWorkers[idx].capabilities.constraints.perCapability[cap].Models[modelID].Capacity += 1
					}
				}

			}
		}
	}
	delete(rwm.requestSessions, requestID)
}

func removeFromRemoteWorkers(rw *RemoteAIWorker, remoteWorkers []*RemoteAIWorker) []*RemoteAIWorker {
	if len(remoteWorkers) == 0 {
		// No workers to remove, return
		return remoteWorkers
	}

	newRemoteWs := make([]*RemoteAIWorker, 0)
	for _, t := range remoteWorkers {
		if t != rw {
			newRemoteWs = append(newRemoteWs, t)
		}
	}
	return newRemoteWs
}

type RemoteAIWorkerResult struct {
	Results      interface{}
	Files        map[string][]byte
	Err          error
	DownloadTime time.Duration
}

type AIWorkerChan chan *RemoteAIWorkerResult

func (rwm *RemoteAIWorkerManager) getTaskChan(taskID int64) (AIWorkerChan, error) {
	rwm.taskMutex.RLock()
	defer rwm.taskMutex.RUnlock()
	if tc, ok := rwm.taskChans[taskID]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No AI Worker channel")
}

func (rwm *RemoteAIWorkerManager) addTaskChan() (int64, AIWorkerChan) {
	rwm.taskMutex.Lock()
	defer rwm.taskMutex.Unlock()
	taskID := rwm.taskCount
	rwm.taskCount++
	if tc, ok := rwm.taskChans[taskID]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("AI Worker channel already exists for ", taskID)
		return taskID, tc
	}
	rwm.taskChans[taskID] = make(AIWorkerChan, 1)
	return taskID, rwm.taskChans[taskID]
}

func (rwm *RemoteAIWorkerManager) removeTaskChan(taskID int64) {
	rwm.taskMutex.Lock()
	defer rwm.taskMutex.Unlock()
	if _, ok := rwm.taskChans[taskID]; !ok {
		glog.V(common.DEBUG).Info("AI Worker channel nonexistent for job ", taskID)
		return
	}
	delete(rwm.taskChans, taskID)
}

// Process does actual AI processing by sending work to remote ai worker and waiting for the result
func (rw *RemoteAIWorker) Process(logCtx context.Context, pipeline string, modelID string, fname string, req AIJobRequestData) (*RemoteAIWorkerResult, error) {
	taskID, taskChan := rw.manager.addTaskChan()
	defer rw.manager.removeTaskChan(taskID)

	signalEOF := func(err error) (*RemoteAIWorkerResult, error) {
		rw.done()
		clog.Errorf(logCtx, "Fatal error with remote AI worker=%s taskId=%d pipeline=%s model_id=%s err=%q", rw.addr, taskID, pipeline, modelID, err)
		return nil, RemoteAIWorkerFatalError{err}
	}

	reqParams, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	jobData := &net.AIJobData{
		Pipeline:    pipeline,
		RequestData: reqParams,
	}
	msg := &net.NotifyAIJob{
		TaskId:    taskID,
		AIJobData: jobData,
	}
	err = rw.stream.Send(msg)

	if err != nil {
		return signalEOF(err)
	}

	clog.V(common.DEBUG).Infof(logCtx, "Job sent to AI worker worker=%s taskId=%d pipeline=%s model_id=%s", rw.addr, taskID, pipeline, modelID)
	// set a minimum timeout to accommodate transport / processing overhead
	// TODO: this should be set for each pipeline, using something long for now
	dur := aiWorkerRequestTimeout

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	select {
	case <-ctx.Done():
		return signalEOF(ErrRemoteWorkerTimeout)
	case chanData := <-taskChan:
		clog.InfofErr(logCtx, "Successfully received results from remote worker=%s taskId=%d pipeline=%s model_id=%s dur=%v",
			rw.addr, taskID, pipeline, modelID, time.Since(start), chanData.Err)

		if monitor.Enabled {
			monitor.AIResultDownloaded(logCtx, pipeline, modelID, chanData.DownloadTime)
		}

		return chanData, chanData.Err
	}
}

type AIResult struct {
	Err    error
	Result *worker.ImageResponse
	Files  map[string]string
}

type AIChanData struct {
	ctx context.Context
	req interface{}
	res chan *AIResult
}

type AIJobRequestData struct {
	InputUrl string      `json:"input_url"`
	Request  interface{} `json:"request"`
}

type AIJobChan chan *AIChanData

// CheckAICapacity verifies if the orchestrator can process a request for a specific pipeline and modelID.
func (orch *orchestrator) CheckAICapacity(pipeline, modelID string) bool {
	if orch.node.AIWorker != nil {
		// confirm local worker has capacity
		return orch.node.AIWorker.HasCapacity(pipeline, modelID)
	} else {
		// remote workers: RemoteAIWorkerManager only selects remote workers if they have capacity for the pipeline/model
		if orch.node.AIWorkerManager != nil {
			return orch.node.AIWorkerManager.workerHasCapacity(pipeline, modelID)
		} else {
			return false
		}
	}
}

func (orch *orchestrator) AIResults(tcID int64, res *RemoteAIWorkerResult) {
	orch.node.AIWorkerManager.aiResults(tcID, res)
}

func (rwm *RemoteAIWorkerManager) aiResults(tcID int64, res *RemoteAIWorkerResult) {
	remoteChan, err := rwm.getTaskChan(tcID)
	if err != nil {
		return // do we need to return anything?
	}

	remoteChan <- res
}

func (n *LivepeerNode) saveLocalAIWorkerResults(ctx context.Context, results interface{}, requestID string, contentType string) (interface{}, error) {
	ext, _ := common.ExtensionByType(contentType)
	fileName := string(RandomManifestID()) + ext

	imgRes, ok := results.(worker.ImageResponse)
	if !ok {
		// worker.TextResponse is JSON, no file save needed
		return results, nil
	}
	storage, exists := n.StorageConfigs[requestID]
	if !exists {
		return nil, errors.New("no storage available for request")
	}
	var buf bytes.Buffer
	for i, image := range imgRes.Images {
		buf.Reset()
		err := worker.ReadImageB64DataUrl(image.Url, &buf)
		if err != nil {
			// try to load local file (image to video returns local file)
			f, err := os.ReadFile(image.Url)
			if err != nil {
				return nil, err
			}
			buf = *bytes.NewBuffer(f)
		}

		osUrl, err := storage.OS.SaveData(ctx, fileName, bytes.NewBuffer(buf.Bytes()), nil, 0)
		if err != nil {
			return nil, err
		}

		imgRes.Images[i].Url = osUrl
	}

	return imgRes, nil
}

func (n *LivepeerNode) saveRemoteAIWorkerResults(ctx context.Context, results *RemoteAIWorkerResult, requestID string) (*RemoteAIWorkerResult, error) {
	if drivers.NodeStorage == nil {
		return nil, fmt.Errorf("Missing local storage")
	}

	// worker.ImageResponse used by ***-to-image and image-to-video require saving binary data for download
	// other pipelines do not require saving data since they are text responses
	imgResp, isImg := results.Results.(worker.ImageResponse)
	if isImg {
		for idx, _ := range imgResp.Images {
			fileName := imgResp.Images[idx].Url
			// save the file data to node and provide url for download
			storage, exists := n.StorageConfigs[requestID]
			if !exists {
				return nil, errors.New("no storage available for request")
			}
			osUrl, err := storage.OS.SaveData(ctx, fileName, bytes.NewReader(results.Files[fileName]), nil, 0)
			if err != nil {
				return nil, err
			}

			imgResp.Images[idx].Url = osUrl
			delete(results.Files, fileName)
		}

		// update results for url updates
		results.Results = imgResp
	}

	return results, nil
}

func (orch *orchestrator) TextToImage(ctx context.Context, requestID string, req worker.GenTextToImageJSONRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		workerResp, err := orch.node.TextToImage(ctx, req)
		if err == nil {
			return orch.node.saveLocalAIWorkerResults(ctx, *workerResp, requestID, "image/png")
		} else {
			clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
			if monitor.Enabled {
				monitor.AIResultSaveError(ctx, "text-to-image", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
			}
			return nil, err
		}
	}

	// remote ai worker proceses job
	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "text-to-image", *req.ModelId, "", AIJobRequestData{Request: req})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "text-to-image", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) ImageToImage(ctx context.Context, requestID string, req worker.GenImageToImageMultipartRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		workerResp, err := orch.node.ImageToImage(ctx, req)
		if err == nil {
			return orch.node.saveLocalAIWorkerResults(ctx, *workerResp, requestID, "image/png")
		} else {
			clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
			if monitor.Enabled {
				monitor.AIResultSaveError(ctx, "image-to-image", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
			}
			return nil, err
		}
	}

	// remote ai worker proceses job
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	inputUrl, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	if err != nil {
		return nil, err
	}
	req.Image.InitFromBytes(nil, "") // remove image data

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "image-to-image", *req.ModelId, inputUrl, AIJobRequestData{Request: req, InputUrl: inputUrl})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "image-to-image", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) ImageToVideo(ctx context.Context, requestID string, req worker.GenImageToVideoMultipartRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		workerResp, err := orch.node.ImageToVideo(ctx, req)
		if err == nil {
			return orch.node.saveLocalAIWorkerResults(ctx, *workerResp, requestID, "video/mp4")
		} else {
			clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
			if monitor.Enabled {
				monitor.AIResultSaveError(ctx, "image-to-video", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
			}
			return nil, err
		}
	}

	// remote ai worker proceses job
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	inputUrl, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	if err != nil {
		return nil, err
	}
	req.Image.InitFromBytes(nil, "") // remove image data

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "image-to-video", *req.ModelId, inputUrl, AIJobRequestData{Request: req, InputUrl: inputUrl})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "image-to-video", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) Upscale(ctx context.Context, requestID string, req worker.GenUpscaleMultipartRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		workerResp, err := orch.node.Upscale(ctx, req)
		if err == nil {
			return orch.node.saveLocalAIWorkerResults(ctx, *workerResp, requestID, "image/png")
		} else {
			clog.Errorf(ctx, "Error processing with local ai worker err=%q", err)
			if monitor.Enabled {
				monitor.AIResultSaveError(ctx, "upscale", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
			}
			return nil, err
		}
	}

	// remote ai worker proceses job
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	inputUrl, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	if err != nil {
		return nil, err
	}
	req.Image.InitFromBytes(nil, "") // remove image data

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "upscale", *req.ModelId, inputUrl, AIJobRequestData{Request: req, InputUrl: inputUrl})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "upscale", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) AudioToText(ctx context.Context, requestID string, req worker.GenAudioToTextMultipartRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		// no file response to save, response is text sent back to gateway
		return orch.node.AudioToText(ctx, req)
	}

	// remote ai worker proceses job
	audioBytes, err := req.Audio.Bytes()
	if err != nil {
		return nil, err
	}

	inputUrl, err := orch.SaveAIRequestInput(ctx, requestID, audioBytes)
	if err != nil {
		return nil, err
	}
	req.Audio.InitFromBytes(nil, "") // remove audio data

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "audio-to-text", *req.ModelId, inputUrl, AIJobRequestData{Request: req, InputUrl: inputUrl})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "audio-to-text", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) SegmentAnything2(ctx context.Context, requestID string, req worker.GenSegmentAnything2MultipartRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		// no file response to save, response is text sent back to gateway
		return orch.node.SegmentAnything2(ctx, req)
	}

	// remote ai worker proceses job
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	inputUrl, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	if err != nil {
		return nil, err
	}
	req.Image.InitFromBytes(nil, "") // remove image data

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "segment-anything-2", *req.ModelId, inputUrl, AIJobRequestData{Request: req, InputUrl: inputUrl})
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
	if err != nil {
		clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
		if monitor.Enabled {
			monitor.AIResultSaveError(ctx, "segment-anything-2", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
		}
		return nil, err
	}

	return res.Results, nil
}

// Return type is LLMResponse, but a stream is available as well as chan(string)
func (orch *orchestrator) LLM(ctx context.Context, requestID string, req worker.GenLLMFormdataRequestBody) (interface{}, error) {
	// local AIWorker processes job if combined orchestrator/ai worker
	if orch.node.AIWorker != nil {
		// no file response to save, response is text sent back to gateway
		return orch.node.AIWorker.LLM(ctx, req)
	}

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "llm", *req.ModelId, "", AIJobRequestData{Request: req})
	if err != nil {
		return nil, err
	}

	// non streaming response
	if _, ok := res.Results.(worker.LLMResponse); ok {
		res, err = orch.node.saveRemoteAIWorkerResults(ctx, res, requestID)
		if err != nil {
			clog.Errorf(ctx, "Error saving remote ai result err=%q", err)
			if monitor.Enabled {
				monitor.AIResultSaveError(ctx, "llm", *req.ModelId, string(monitor.SegmentUploadErrorUnknown))
			}
			return nil, err

		}
	}

	return res.Results, nil
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

//
// Methods called at AI Worker to process AI job
//

// save base64 data to file and returns file path or error
func (n *LivepeerNode) SaveBase64Result(ctx context.Context, data string, requestID string, contentType string) (string, error) {
	resultName := string(RandomManifestID())
	ext, err := common.ExtensionByType(contentType)
	if err != nil {
		return "", err
	}

	resultFile := resultName + ext
	fname := path.Join(n.WorkDir, resultFile)
	err = worker.SaveImageB64DataUrl(data, fname)
	if err != nil {
		return "", err
	}

	return fname, nil
}

func (n *LivepeerNode) TextToImage(ctx context.Context, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.TextToImage(ctx, req)
}

func (n *LivepeerNode) ImageToImage(ctx context.Context, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.ImageToImage(ctx, req)
}

func (n *LivepeerNode) Upscale(ctx context.Context, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.Upscale(ctx, req)
}

func (n *LivepeerNode) AudioToText(ctx context.Context, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	return n.AIWorker.AudioToText(ctx, req)
}
func (n *LivepeerNode) ImageToVideo(ctx context.Context, req worker.GenImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	// We might support generating more than one video in the future (i.e. multiple input images/prompts)
	numVideos := 1

	// Generate frames
	start := time.Now()
	resp, err := n.AIWorker.ImageToVideo(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Frames) != numVideos {
		return nil, fmt.Errorf("unexpected number of image-to-video outputs expected=%v actual=%v", numVideos, len(resp.Frames))
	}

	took := time.Since(start)
	clog.V(common.DEBUG).Infof(ctx, "Generating frames took=%v", took)

	sessionID := string(RandomManifestID())
	framerate := 7
	if req.Fps != nil {
		framerate = *req.Fps
	}
	inProfile := ffmpeg.VideoProfile{
		Framerate:    uint(framerate),
		FramerateDen: 1,
	}
	height := 576
	if req.Height != nil {
		height = *req.Height
	}
	width := 1024
	if req.Width != nil {
		width = *req.Width
	}
	outProfile := ffmpeg.VideoProfile{
		Name:       "image-to-video",
		Resolution: fmt.Sprintf("%vx%v", width, height),
		Bitrate:    "6000k",
		Format:     ffmpeg.FormatMP4,
	}
	// HACK: Re-use worker.ImageResponse to return results
	// Transcode frames into segments.
	videos := make([]worker.Media, len(resp.Frames))
	for i, batch := range resp.Frames {
		// Create slice of frame urls for a batch
		urls := make([]string, len(batch))
		for j, frame := range batch {
			urls[j] = frame.Url
		}

		// Transcode slice of frame urls into a segment
		res := n.transcodeFrames(ctx, sessionID, urls, inProfile, outProfile)
		if res.Err != nil {
			return nil, res.Err
		}

		// Assume only single rendition right now
		seg := res.TranscodeData.Segments[0]
		resultFile := fmt.Sprintf("%v.mp4", RandomManifestID())
		fname := path.Join(n.WorkDir, resultFile)
		if err := os.WriteFile(fname, seg.Data, 0644); err != nil {
			clog.Errorf(ctx, "AI Worker cannot write file err=%q", err)
			return nil, err
		}

		videos[i] = worker.Media{
			Url: fname,
		}

		// NOTE: Seed is consistent for video; NSFW check applies to first frame only.
		if len(batch) > 0 {
			videos[i].Nsfw = batch[0].Nsfw
			videos[i].Seed = batch[0].Seed
		}
	}

	return &worker.ImageResponse{Images: videos}, nil
}

func (n *LivepeerNode) SegmentAnything2(ctx context.Context, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	return n.AIWorker.SegmentAnything2(ctx, req)
}

func (n *LivepeerNode) LLM(ctx context.Context, req worker.GenLLMFormdataRequestBody) (interface{}, error) {
	return n.AIWorker.LLM(ctx, req)
}

func (n *LivepeerNode) transcodeFrames(ctx context.Context, sessionID string, urls []string, inProfile ffmpeg.VideoProfile, outProfile ffmpeg.VideoProfile) *TranscodeResult {
	ctx = clog.AddOrchSessionID(ctx, sessionID)

	var fnamep *string
	terr := func(err error) *TranscodeResult {
		if fnamep != nil {
			if err := os.RemoveAll(*fnamep); err != nil {
				clog.Errorf(ctx, "Transcoder failed to cleanup %v", *fnamep)
			}
		}
		return &TranscodeResult{Err: err}
	}

	// We only support base64 png data urls right now
	// We will want to support HTTP and file urls later on as well
	dirPath := path.Join(n.WorkDir, "input", sessionID+"_"+string(RandomManifestID()))
	fnamep = &dirPath
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		clog.Errorf(ctx, "Transcoder cannot create frames dir err=%q", err)
		return terr(err)
	}
	for i, url := range urls {
		fname := path.Join(dirPath, strconv.Itoa(i)+".png")
		if err := worker.SaveImageB64DataUrl(url, fname); err != nil {
			clog.Errorf(ctx, "Transcoder failed to save image from url err=%q", err)
			return terr(err)
		}
	}

	// Use local software transcoder instead of node's configured transcoder
	// because if the node is using a nvidia transcoder there may be sporadic
	// CUDA operation not permitted errors that are difficult to debug.
	// The majority of the execution time for image-to-video is the frame generation
	// so slower software transcoding should not be a big deal for now.
	transcoder := NewLocalTranscoder(n.WorkDir)

	md := &SegTranscodingMetadata{
		Fname:     path.Join(dirPath, "%d.png"),
		ProfileIn: inProfile,
		Profiles: []ffmpeg.VideoProfile{
			outProfile,
		},
		AuthToken: &net.AuthToken{SessionId: sessionID},
	}

	los := drivers.NodeStorage.NewSession(sessionID)

	// TODO: Figure out a better way to end the OS session after a timeout than creating a new goroutine per request?
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), aiWorkerResultsTimeout)
		defer cancel()
		<-ctx.Done()
		los.EndSession()
		clog.Infof(ctx, "Ended image-to-video session sessionID=%v", sessionID)
	}()

	start := time.Now()
	tData, err := transcoder.Transcode(ctx, md)
	if err != nil {
		if _, ok := err.(UnrecoverableError); ok {
			panic(err)
		}
		clog.Errorf(ctx, "Error transcoding frames dirPath=%s err=%q", dirPath, err)
		return terr(err)
	}

	took := time.Since(start)
	clog.V(common.DEBUG).Infof(ctx, "Transcoding frames took=%v", took)

	transcoder.EndTranscodingSession(md.AuthToken.SessionId)

	tSegments := tData.Segments
	if len(tSegments) != len(md.Profiles) {
		clog.Errorf(ctx, "Did not receive the correct number of transcoded segments; got %v expected %v", len(tSegments),
			len(md.Profiles))
		return terr(fmt.Errorf("MismatchedSegments"))
	}

	// Prepare the result object
	var tr TranscodeResult
	segHashes := make([][]byte, len(tSegments))

	for i := range md.Profiles {
		if tSegments[i].Data == nil || len(tSegments[i].Data) < 25 {
			clog.Errorf(ctx, "Cannot find transcoded segment for bytes=%d", len(tSegments[i].Data))
			return terr(fmt.Errorf("ZeroSegments"))
		}
		clog.V(common.DEBUG).Infof(ctx, "Transcoded segment profile=%s bytes=%d",
			md.Profiles[i].Name, len(tSegments[i].Data))
		hash := crypto.Keccak256(tSegments[i].Data)
		segHashes[i] = hash
	}
	if err := os.RemoveAll(dirPath); err != nil {
		clog.Errorf(ctx, "Transcoder failed to cleanup %v", dirPath)
	}
	tr.OS = los
	tr.TranscodeData = tData

	if n == nil || n.Eth == nil {
		return &tr
	}

	segHash := crypto.Keccak256(segHashes...)
	tr.Sig, tr.Err = n.Eth.Sign(segHash)
	if tr.Err != nil {
		clog.Errorf(ctx, "Unable to sign hash of transcoded segment hashes err=%q", tr.Err)
	}
	return &tr
}
