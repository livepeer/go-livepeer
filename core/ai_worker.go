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
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"
	"github.com/livepeer/lpms/ffmpeg"
)

var ErrRemoteWorkerTimeout = errors.New("Remote worker took too long")
var ErrNoCompatibleWorkersAvailable = errors.New("no workers can process job requested")
var ErrNoWorkersAvailable = errors.New("no workers available")

type RemoteAIWorker struct {
	manager      *RemoteAIWorkerManager
	stream       net.AIWorker_RegisterAIWorkerServer
	capabilities *Capabilities
	eof          chan struct{}
	addr         string
	capacity     map[uint32]*net.Capabilities_Constraints
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

func (orch *orchestrator) ServeAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capacity int, capabilities *net.Capabilities) {
	orch.node.serveAIWorker(stream, capabilities)
}

func (n *LivepeerNode) serveAIWorker(stream net.AIWorker_RegisterAIWorkerServer, capabilities *net.Capabilities) {
	from := common.GetConnectionAddr(stream.Context())
	coreCaps := CapabilitiesFromNetCapabilities(capabilities)
	n.Capabilities.AddCapacity(coreCaps)
	n.AddAICapabilities(nil, coreCaps.constraints)
	defer n.Capabilities.RemoveCapacity(coreCaps)
	defer n.RemoveAICapabilities(nil, coreCaps.constraints)

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
func (rwm *RemoteAIWorkerManager) Process(ctx context.Context, requestID string, pipeline string, modelID string, fname string, req interface{}) (*RemoteAIWorkerResult, error) {
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
func (rwm *RemoteAIWorkerManager) completeAIRequest(requestID, pipeline, modelID string) {
	worker, ok := rwm.requestSessions[requestID]
	if !ok {
		return
	}
	for idx, remoteWorker := range rwm.remoteAIWorkers {
		if worker.addr == remoteWorker.addr {
			cap := PipelineToCapability(pipeline)
			if cap > Capability_Unused {
				rwm.remoteAIWorkers[idx].capacity[uint32(cap)].Models[modelID].Capacity += 1
			}
		}
	}
	//sort.Sort(byLoadFactor(rtm.remoteTranscoders))
	delete(rwm.requestSessions, requestID)
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

type RemoteAIWorkerResult struct {
	Results *worker.ImageResponse
	Files   map[string][]byte
	Err     error
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
func (rw *RemoteAIWorker) Process(logCtx context.Context, pipeline string, modelID string, fname string, req interface{}) (*RemoteAIWorkerResult, error) {
	taskID, taskChan := rw.manager.addTaskChan()
	defer rw.manager.removeTaskChan(taskID)

	signalEOF := func(err error) (*RemoteAIWorkerResult, error) {
		rw.done()
		clog.Errorf(logCtx, "Fatal error with remote AI worker=%s taskId=%d pipeline=%s model_id=%s err=%q", rw.addr, taskID, pipeline, modelID, err)
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
		return chanData, chanData.Err
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
	Result *worker.ImageResponse
	Files  map[string]string
}

type AIChanData struct {
	ctx context.Context
	req interface{}
	res chan *AIResult
}

type AIJobChan chan *AIChanData

// CheckAICapacity verifies if the orchestrator can process a request for a specific pipeline and modelID.
func (orch *orchestrator) CheckAICapacity(pipeline, modelID string) bool {
	if orch.node.AIWorker != nil {
		//confirm local worker has capacity
		return orch.node.AIWorker.HasCapacity(pipeline, modelID)
	} else {
		//remote workers: RemoteAIWorkerManager only selects remote workers if they have capacity for the pipeline/model
		if orch.node.AIWorkerManager != nil {
			return true
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

func (n *LivepeerNode) saveAIResults(ctx context.Context, results *RemoteAIWorkerResult, requestID string) (*RemoteAIWorkerResult, error) {
	for idx, _ := range results.Results.Images {
		fileName := results.Results.Images[idx].Url

		if drivers.NodeStorage == nil {
			return nil, fmt.Errorf("Missing local storage")
		}

		los := drivers.NodeStorage.NewSession(requestID)

		// determine appropriate OS to use
		//os := drivers.NewSession(FromNetOsInfo(md.OS))
		//if os == nil {
		//	// no preference (or unknown pref), so use our own
		//	os = los
		//}
		storage := transcodeConfig{
			OS:      los,
			LocalOS: los,
		}
		n.storageMutex.Lock()
		n.StorageConfigs[requestID] = &storage
		n.storageMutex.Unlock()

		//save the file data to node and provide url for download
		url, err := storage.LocalOS.SaveData(ctx, fileName, bytes.NewReader(results.Files[fileName]), nil, 0)
		if err != nil {
			return nil, err
		}

		results.Results.Images[idx].Url = url
		delete(results.Files, fileName)
	}

	// TODO: Figure out a better way to end the OS session after a timeout than creating a new goroutine per request?
	go func() {
		ctx, cancel := transcodeLoopContext()
		defer cancel()
		<-ctx.Done()
		n.storageMutex.Lock()
		n.StorageConfigs[requestID].LocalOS.EndSession()
		delete(n.StorageConfigs, requestID)
		n.storageMutex.Unlock()
		clog.Infof(ctx, "Ended session requestID=%v", requestID)
	}()

	return results, nil
}

func (orch *orchestrator) TextToImage(ctx context.Context, requestID string, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	//no input file needed for processing
	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "text-to-image", *req.ModelId, "", req)
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveAIResults(ctx, res, requestID)
	if err != nil {
		return nil, err
	}

	return res.Results, nil

}

func (orch *orchestrator) ImageToImage(ctx context.Context, requestID string, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "image-to-image", *req.ModelId, "", req)
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveAIResults(ctx, res, requestID)
	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) ImageToVideo(ctx context.Context, requestID string, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "image-to-video", *req.ModelId, "", req)
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveAIResults(ctx, res, requestID)
	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func (orch *orchestrator) Upscale(ctx context.Context, requestID string, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	imgBytes, err := req.Image.Bytes()
	if err != nil {
		return nil, err
	}

	url, err := orch.SaveAIRequestInput(ctx, requestID, imgBytes)
	req.Image.InitFromBytes(nil, url)

	res, err := orch.node.AIWorkerManager.Process(ctx, requestID, "upscale", *req.ModelId, "", req)
	if err != nil {
		return nil, err
	}

	res, err = orch.node.saveAIResults(ctx, res, requestID)
	if err != nil {
		return nil, err
	}

	return res.Results, nil
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
	fname := path.Join(node.WorkDir, inName)
	if err := os.WriteFile(fname, fileData, 0644); err != nil {
		clog.Errorf(ctx, "Orchestrator cannot write file err=%q", err)
		return "", err
	}

	// TODO should we try and serve from disk similar to transcoding if local AI worker?
	// We do not know if it will be local ai worker that does the job at this point in process.
	url, err := storageConfig.LocalOS.SaveData(ctx, inName, bytes.NewReader(fileData), nil, 0)
	if err != nil {
		return "", err
	}

	return url, nil

}

//
// Methods called at Transcoder to process AI job
//

func (n *LivepeerNode) TextToImage(ctx context.Context, req worker.TextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.TextToImage(ctx, req)
}

func (n *LivepeerNode) ImageToImage(ctx context.Context, req worker.ImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.ImageToImage(ctx, req)
}

func (n *LivepeerNode) Upscale(ctx context.Context, req worker.UpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	return n.AIWorker.Upscale(ctx, req)
}

func (n *LivepeerNode) ImageToVideo(ctx context.Context, req worker.ImageToVideoMultipartRequestBody) (*worker.ImageResponse, error) {
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
		name := fmt.Sprintf("%v.mp4", RandomManifestID())
		segData := bytes.NewReader(seg.Data)
		uri, err := res.OS.SaveData(ctx, name, segData, nil, 0) //this is localOS, uri is file path
		if err != nil {
			return nil, err
		}

		videos[i] = worker.Media{
			Url: uri,
		}

		// NOTE: Seed is consistent for video; NSFW check applies to first frame only.
		if len(batch) > 0 {
			videos[i].Nsfw = batch[0].Nsfw
			videos[i].Seed = batch[0].Seed
		}
	}

	return &worker.ImageResponse{Images: videos}, nil
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
		ctx, cancel := transcodeLoopContext()
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
