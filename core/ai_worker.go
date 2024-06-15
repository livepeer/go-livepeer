package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
)

var ErrRemoteWorkerTimeout = errors.New("Remote worker took too long")
var ErrNoCompatibleWorkersAvailable = errors.New("no workers can process job requested")
var ErrNoWorkersAvailable = errors.New("no workers available")

type RemoteAIWorkerManager struct {
	remoteAIWorkers []*RemoteAIWorker
	liveAIWorkers   map[net.AIWorker_RegisterAIWorkerServer]*RemoteAIWorker
	RTmutex         sync.Mutex

	// For tracking tasks assigned to remote aiworkers
	taskMutex *sync.RWMutex
	taskChans map[int64]AIWorkerChan
	taskCount int64

	// Map for keeping track of sessions and their respective aiworkers
	requestSessions map[string]*RemoteAIWorker
}

func NewRemoteAIWorker(m *RemoteAIWorkerManager, stream net.AIWorker_RegisterAIWorkerServer, capacity map[uint32]*net.Capabilities_Constraints, caps *Capabilities) *RemoteAIWorker {
	return &RemoteAIWorker{
		manager:      m,
		stream:       stream,
		capacity:     capacity,
		eof:          make(chan struct{}, 1),
		addr:         common.GetConnectionAddr(stream.Context()),
		capabilities: caps,
	}
}

func NewRemoteAIWorkerManager() *RemoteAIWorkerManager {
	return &RemoteAIWorkerManager{
		remoteAIWorkers: []*RemoteAIWorker{},
		liveAIWorkers:   map[net.AIWorker_RegisterAIWorkerServer]*RemoteAIWorker{},
		RTmutex:         sync.Mutex{},

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
	coreCaps := CapabilitiesFromNetCapabilities(capabilities)
	n.Capabilities.AddCapacity(coreCaps)
	n.AddAICapabilities(nil, coreCaps.constraints)
	defer n.Capabilities.RemoveCapacity(coreCaps)
	defer n.RemoveAICapabilities(nil, coreCaps.constraints)
	glog.V(common.DEBUG).Infof("Closing aiworker=%s channel", from)

	// Manage blocks while transcoder is connected
	n.AIWorkerManager.Manage(stream, capabilities)
	glog.V(common.DEBUG).Infof("Closing aiworker=%s channel", from)
}

// Manage adds aiworker to list of live aiworkers. Doesn't return until aiworker disconnects
func (rtm *RemoteAIWorkerManager) Manage(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	aiworker := NewRemoteAIWorker(rtm, stream, capabilities.Constraints, CapabilitiesFromNetCapabilities(capabilities))
	go func() {
		ctx := stream.Context()
		<-ctx.Done()
		err := ctx.Err()
		glog.Errorf("Stream closed for aiworker=%s, err=%q", from, err)
		aiworker.done()
	}()

	rtm.RTmutex.Lock()
	rtm.liveAIWorkers[aiworker.stream] = aiworker
	rtm.remoteAIWorkers = append(rtm.remoteAIWorkers, aiworker)
	// sort.Sort(byLoadFactor(rtm.remoteAIWorkers))
	rtm.RTmutex.Unlock()

	<-aiworker.eof
	glog.Infof("Got aiworker=%s eof, removing from live aiworkers map", from)

	rtm.RTmutex.Lock()
	delete(rtm.liveAIWorkers, aiworker.stream)
	rtm.RTmutex.Unlock()
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
func (rwm *RemoteAIWorkerManager) Process(ctx context.Context, requestID string, pipeline string, modelID string, fname string, req interface{}) (interface{}, error) {
	worker, err := rwm.selectWorker(requestID, pipeline, modelID)
	if err != nil {
		return nil, err
	}
	res, err := worker.Process(ctx, pipeline, modelID, fname, req)
	if err != nil {
		rwm.RTmutex.Lock()
		rwm.completeAIRequest(requestID)
		rwm.RTmutex.Unlock()
	}
	_, fatal := err.(RemoteAIWorkerFatalError)
	if fatal {
		// Don't retry if we've timed out; gateway likely to have moved on
		// XXX problematic for VOD when we *should* retry
		if err.(RemoteAIWorkerFatalError).error == ErrRemoteTranscoderTimeout {
			return res, err
		}
		return rwm.Process(ctx, requestID, pipeline, modelID, fname, req)
	}
	return res, err
}

func (rwm *RemoteAIWorkerManager) selectWorker(requestID string, pipeline string, modelID string) (*RemoteAIWorker, error) {
	rwm.RTmutex.Lock()
	defer rwm.RTmutex.Unlock()

	checkWorkers := func(rtm *RemoteAIWorkerManager) bool {
		return len(rtm.remoteAIWorkers) > 0
	}

	findCompatibleWorker := func(rtm *RemoteAIWorkerManager) int {
		cap := PipelineToCapability(pipeline)
		for idx, worker := range rwm.remoteAIWorkers {
			if worker.capacity[uint32(cap)].Models[modelID].Capacity > 0 {
				worker.capacity[uint32(cap)].Models[modelID].Capacity -= 1
				return idx
			}
		}
		return -1
	}

	for checkWorkers(rwm) {
		worker, sessionExists := rwm.requestSessions[requestID]
		newWorker := findCompatibleWorker(rwm)
		if newWorker == -1 {
			return nil, ErrNoCompatibleTranscodersAvailable
		}
		if !sessionExists {
			worker = rwm.remoteAIWorkers[newWorker]
		}

		if _, ok := rwm.liveAIWorkers[worker.stream]; !ok {
			// Remove the stream session because the worker is no longer live
			if sessionExists {
				rwm.completeAIRequest(requestID)
			}
			// worker does not exist in table; remove and retry
			rwm.remoteAIWorkers = removeFromRemoteWorkers(worker, rwm.remoteAIWorkers)
			continue
		}

		if !sessionExists {
			// Assinging transcoder to session for future use
			rwm.requestSessions[requestID] = worker
			//TODO add sorting by inference time for each pipeline/model
			//sort.Sort(byLoadFactor(rtm.remoteTranscoders))
		}
		return worker, nil
	}

	return nil, ErrNoWorkersAvailable
}

// completeRequestSessions end a AI request session for a remote ai worker
// caller should hold the mutex lock
func (rtm *RemoteAIWorkerManager) completeAIRequest(requestID string) {
	_, ok := rtm.requestSessions[requestID]
	if !ok {
		return
	}

	//sort.Sort(byLoadFactor(rtm.remoteTranscoders))
	delete(rtm.requestSessions, requestID)
}

func removeFromRemoteWorkers(rw *RemoteAIWorker, remoteWorkers []*RemoteAIWorker) []*RemoteAIWorker {
	if len(remoteWorkers) == 0 {
		// No transcoders to remove, return
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

type RemoteWorkerResult struct {
	ResultData interface{}
	Err        error
}

type AIWorkerChan chan *RemoteWorkerResult

func (rwm *RemoteAIWorkerManager) getTaskChan(taskID int64) (AIWorkerChan, error) {
	rwm.taskMutex.RLock()
	defer rwm.taskMutex.RUnlock()
	if tc, ok := rwm.taskChans[taskID]; ok {
		return tc, nil
	}
	return nil, fmt.Errorf("No transcoder channel")
}

func (rwm *RemoteAIWorkerManager) addTaskChan() (int64, AIWorkerChan) {
	rwm.taskMutex.Lock()
	defer rwm.taskMutex.Unlock()
	taskID := rwm.taskCount
	rwm.taskCount++
	if tc, ok := rwm.taskChans[taskID]; ok {
		// should really never happen
		glog.V(common.DEBUG).Info("Transcoder channel already exists for ", taskID)
		return taskID, tc
	}
	rwm.taskChans[taskID] = make(AIWorkerChan, 1)
	return taskID, rwm.taskChans[taskID]
}

func (rwm *RemoteAIWorkerManager) removeTaskChan(taskID int64) {
	rwm.taskMutex.Lock()
	defer rwm.taskMutex.Unlock()
	if _, ok := rwm.taskChans[taskID]; !ok {
		glog.V(common.DEBUG).Info("Transcoder channel nonexistent for job ", taskID)
		return
	}
	delete(rwm.taskChans, taskID)
}

// Process does actual AI processing by sending work to remote ai worker and waiting for the result
func (rw *RemoteAIWorker) Process(logCtx context.Context, pipeline string, modelID string, fname string, req interface{}) (interface{}, error) {
	taskID, taskChan := rw.manager.addTaskChan()
	defer rw.manager.removeTaskChan(taskID)

	signalEOF := func(err error) (interface{}, error) {
		rw.done()
		clog.Errorf(logCtx, "Fatal error with remote worker=%s taskId=%d pipeline=%s model_id=%s err=%q", rw.addr, taskID, pipeline, modelID, err)
		return nil, RemoteTranscoderFatalError{err}
	}

	reqParams, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	msg := &net.NotifyAIJob{
		Url:         fname,
		TaskId:      taskID,
		Pipeline:    pipeline,
		ModelID:     modelID,
		RequestData: reqParams,
	}
	err = rw.stream.Send(msg)

	if err != nil {
		return signalEOF(err)
	}

	// set a minimum timeout to accommodate transport / processing overhead
	//TODO: this should be set for each pipeline, using something long for now
	dur := 15 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	select {
	case <-ctx.Done():
		return signalEOF(ErrRemoteWorkerTimeout)
	case chanData := <-taskChan:
		clog.InfofErr(logCtx, "Successfully received results from remote worker=%s taskId=%d pipeline=%s model_id=%s dur=%v",
			rw.addr, taskID, pipeline, modelID, time.Since(start), chanData.Err)
		return chanData.ResultData, chanData.Err
	}
}

func (n *LivepeerNode) getAIJobChan(ctx context.Context, requestID string) (AIJobChan, error) {
	n.aiJobMutex.Lock()
	defer n.aiJobMutex.Unlock()

	ac := make(AIJobChan, 1)
	n.AIJobChans[ManifestID(requestID)] = ac
	return ac, nil
}

type AIResult struct {
	Err    error
	Result interface{}
	OS     drivers.OSSession
}

type AIChanData struct {
	ctx context.Context
	req interface{}
	res chan *AIResult
}

type AIJobChan chan *AIChanData

// CheckAICapacity verifies if the orchestrator can process a request for a specific pipeline and modelID.
func (orch *orchestrator) CheckAICapacity(pipeline, modelID string) bool {
	return orch.node.AIWorker.HasCapacity(pipeline, modelID)
}

func (orch *orchestrator) TextToImage(ctx context.Context, requestID string, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	//no input file needed for processing
	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "text-to-image", *req.ModelId, "", req)
	if err != nil {
		return nil, err
	}

	imgResp, ok := res.(*worker.ImageResponse)
	if ok {
		return imgResp, nil
	} else {
		return nil, errors.New("response not in expected format")
	}

}

func (orch *orchestrator) ImageToImage(ctx context.Context, requestID string, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	return orch.node.imageToImage(ctx, req)
}

func (orch *orchestrator) ImageToVideo(ctx context.Context, requestID string, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	return orch.node.imageToVideo(ctx, req)
}

func (orch *orchestrator) Upscale(ctx context.Context, requestID string, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	return orch.node.upscale(ctx, req)
}

func (orch *orchestrator) SaveAIRequestInput(ctx context.Context, requestID string, fileData []byte) (string, error) {
	node := orch.node
	if drivers.NodeStorage == nil {
		return "", fmt.Errorf("Missing local storage")
	}

	//TODO: should the gateway provide preferred storage similar to transcoding?
	storage := drivers.NodeStorage.NewSession(requestID)
	storageConfig := &transcodeConfig{OS: storage, LocalOS: storage}
	node.storageMutex.Lock()
	node.StorageConfigs[requestID] = storageConfig
	node.storageMutex.Unlock()

	inName := common.RandName() + ".tempfile"
	if _, err := os.Stat(node.WorkDir); os.IsNotExist(err) {
		err := os.Mkdir(node.WorkDir, 0700)
		if err != nil {
			clog.Errorf(ctx, "Orchestrator cannot create workdir err=%q", err)
			return "", err
		}
	}
	// Create input file, removed after claiming complete or error
	//fname := path.Join(node.WorkDir, inName)
	//if err := os.WriteFile(fname, fileData, 0644); err != nil {
	//	clog.Errorf(ctx, "Orchestrator cannot write file err=%q", err)
	//	return "", err
	//}

	// TODO should we try and serve from disk similar to transcoding if local AI worker?
	// We do not know if it will be local ai worker that does the job at this point in process.
	url, err := storageConfig.LocalOS.SaveData(ctx, inName, bytes.NewReader(fileData), nil, 0)
	if err != nil {
		return "", err
	}

	return url, nil

}
