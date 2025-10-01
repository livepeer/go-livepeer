package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/go-livepeer/ai/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"
	"github.com/livepeer/go-tools/drivers"

	"github.com/stretchr/testify/assert"
)

func TestPipelineToCapability(t *testing.T) {
	good := "audio-to-text"
	bad := "i-love-tests"
	noSpaces := "llm"

	cap, err := PipelineToCapability(good)
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_AudioToText)

	cap, err = PipelineToCapability(bad)
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)

	cap, err = PipelineToCapability(noSpaces)
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_LLM)
}

func TestServeAIWorker(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities = NewCapabilities(DefaultCapabilities(), nil)
	n.Capabilities.SetPerCapabilityConstraints(make(PerCapabilityConstraints))
	n.Capabilities.SetMinVersionConstraint("1.0")
	n.AIWorkerManager = NewRemoteAIWorkerManager()
	strm := &StubAIWorkerServer{}

	// test that an ai worker was created
	caps := createAIWorkerCapabilities()
	netCaps := caps.ToNetCapabilities()
	go n.serveAIWorker(strm, netCaps, nil)
	time.Sleep(1 * time.Second)

	wkr, ok := n.AIWorkerManager.liveAIWorkers[strm]
	if !ok {
		t.Error("Unexpected transcoder type")
	}
	//confirm worker info
	assert.Equal(t, wkr.capabilities, caps)
	assert.Nil(t, wkr.hardware)

	// test shutdown
	wkr.eof <- struct{}{}
	time.Sleep(1 * time.Second)

	// stream should be removed
	_, ok = n.AIWorkerManager.liveAIWorkers[strm]
	if ok {
		t.Error("Unexpected ai worker presence")
	}

	//confirm no workers connected
	assert.Len(t, n.AIWorkerManager.liveAIWorkers, 0)

	//connect worker with hardware information
	strm1 := &StubAIWorkerServer{}
	hdwDetail := net.GPUComputeInfo{Id: "gpu-1", Name: "gpu name", Major: 8, Minor: 9, MemoryFree: 1, MemoryTotal: 10}
	hdwInfo := make(map[string]*net.GPUComputeInfo)
	hdwInfo["0"] = &hdwDetail
	hdw := net.HardwareInformation{Pipeline: "livepeer-pipeline", ModelId: "livepeer/model1", GpuInfo: hdwInfo}
	var netHdwList []*net.HardwareInformation
	netHdwList = append(netHdwList, &hdw)
	go n.serveAIWorker(strm1, netCaps, netHdwList)
	time.Sleep(1 * time.Second)

	wkr, ok = n.AIWorkerManager.liveAIWorkers[strm1]
	if !ok {
		t.Error("Unexpected transcoder type")
	}

	//confirm worker attached and has hardware information
	assert.Len(t, n.AIWorkerManager.liveAIWorkers, 1)
	wkrHdw := hardwareInformationFromNetHardware(netHdwList)
	assert.Equal(t, wkrHdw, n.AIWorkerManager.liveAIWorkers[strm1].hardware)

	// test shutdown
	wkr.eof <- struct{}{}
	time.Sleep(1 * time.Second)

	// stream should be removed
	_, ok = n.AIWorkerManager.liveAIWorkers[strm]
	if ok {
		t.Error("Unexpected ai worker presence")
	}
}

func TestServeAIWorker_IncompatibleVersion(t *testing.T) {
	assert := assert.New(t)
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities.SetPerCapabilityConstraints(make(PerCapabilityConstraints))
	n.Capabilities.SetMinVersionConstraint("1.1")
	n.AIWorkerManager = NewRemoteAIWorkerManager()
	strm := &StubAIWorkerServer{}

	// test that an ai worker was created
	caps := createAIWorkerCapabilities()
	netCaps := caps.ToNetCapabilities()
	go n.serveAIWorker(strm, netCaps, nil)
	time.Sleep(5 * time.Second)
	assert.Zero(len(n.AIWorkerManager.liveAIWorkers))
	assert.Zero(len(n.AIWorkerManager.remoteAIWorkers))
	assert.Zero(len(n.Capabilities.constraints.perCapability))
}

func TestRemoteAIWorkerManager(t *testing.T) {
	m := NewRemoteAIWorkerManager()
	initAIWorker := func() (*RemoteAIWorker, *StubAIWorkerServer) {
		strm := &StubAIWorkerServer{manager: m}
		caps := createAIWorkerCapabilities()
		wkr := NewRemoteAIWorker(m, strm, caps, nil)
		return wkr, strm
	}
	//create worker and connect to manager
	wkr, strm := initAIWorker()

	go func() {
		m.Manage(strm, wkr.capabilities.ToNetCapabilities(), nil)
	}()
	time.Sleep(1 * time.Millisecond) // allow the workers to activate

	//check workers connected
	assert.Equal(t, 1, len(m.remoteAIWorkers))
	assert.NotNil(t, m.liveAIWorkers[strm])
	//create request
	req := worker.GenTextToImageJSONRequestBody{}
	req.Prompt = "a titan carrying steel ball with livepeer logo"

	// happy path
	res, err := m.Process(context.TODO(), "request_id1", "text-to-image", "livepeer/model1", "", AIJobRequestData{Request: req})
	results, ok := res.Results.(worker.ImageResponse)
	assert.True(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, "image_url", results.Images[0].Url)

	// error on remote
	strm.JobError = fmt.Errorf("JobError")
	_, err = m.Process(context.TODO(), "request_id2", "text-to-image", "livepeer/model1", "", AIJobRequestData{Request: req})
	assert.NotNil(t, err)
	strm.JobError = nil

	//check worker is still connected
	assert.Equal(t, 1, len(m.remoteAIWorkers))

	// simulate error with sending
	// m.Process keeps retrying since error is not fatal
	strm.SendError = ErrNoWorkersAvailable
	_, err = m.Process(context.TODO(), "request_id3", "text-to-image", "livepeer/model1", "", AIJobRequestData{Request: req})
	_, fatal := err.(RemoteAIWorkerFatalError)
	if !fatal && err.Error() != strm.SendError.Error() {
		t.Error("Unexpected error ", err, fatal)
	}
	strm.SendError = nil

	//check worker is disconnected
	assert.Equal(t, 0, len(m.remoteAIWorkers))
	assert.Nil(t, m.liveAIWorkers[strm])
}

func TestSelectAIWorker(t *testing.T) {
	m := NewRemoteAIWorkerManager()
	strm := &StubAIWorkerServer{manager: m, DelayResults: false}
	strm2 := &StubAIWorkerServer{manager: m}

	capabilities := createAIWorkerCapabilities()

	extraModelCapabilities := createAIWorkerCapabilities()
	extraModelCapabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"] = &ModelConstraint{Warm: true, Capacity: 2}
	extraModelCapabilities.constraints.perCapability[Capability_ImageToImage] = &CapabilityConstraints{Models: make(ModelConstraints)}
	extraModelCapabilities.constraints.perCapability[Capability_ImageToImage].Models["livepeer/model2"] = &ModelConstraint{Warm: true, Capacity: 1}

	// sanity check that ai worker is not in liveAIWorkers or remoteAIWorkers
	assert := assert.New(t)
	assert.Nil(m.liveAIWorkers[strm])
	assert.Empty(m.remoteAIWorkers)

	// register ai workers, which adds ai worker to liveAIWorkers and remoteAIWorkers
	wg := newWg(1)
	go func() { m.Manage(strm, capabilities.ToNetCapabilities(), nil) }()
	time.Sleep(1 * time.Millisecond) // allow time for first stream to register
	go func() { m.Manage(strm2, extraModelCapabilities.ToNetCapabilities(), nil); wg.Done() }()
	time.Sleep(1 * time.Millisecond) // allow time for second stream to register e for third stream to register

	//update worker.addr to be different
	m.remoteAIWorkers[0].addr = string(RandomManifestID())
	m.remoteAIWorkers[1].addr = string(RandomManifestID())

	assert.NotNil(m.liveAIWorkers[strm])
	assert.NotNil(m.liveAIWorkers[strm2])
	assert.Len(m.remoteAIWorkers, 2)

	testRequestId := "testID"
	testRequestId2 := "testID2"
	testRequestId3 := "testID3"
	testRequestId4 := "testID4"

	// ai worker is returned from selectAIWorker
	currentWorker, err := m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	assert.NotNil(currentWorker)
	assert.NotNil(m.liveAIWorkers[strm])
	assert.Len(m.remoteAIWorkers, 2)
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model1")

	// check selecting model for one pipeline does not impact other pipeline with same model
	_, err = m.selectWorker(testRequestId, "image-to-image", "livepeer/model2")
	assert.Nil(err)
	assert.Equal(0, m.remoteAIWorkers[1].capabilities.constraints.perCapability[Capability_ImageToImage].Models["livepeer/model2"].Capacity)
	assert.Equal(2, m.remoteAIWorkers[1].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)
	m.completeAIRequest(testRequestId, "image-to-image", "livepeer/model2")

	// select all of capacity for ai workers model1
	_, err = m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	_, err = m.selectWorker(testRequestId2, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	w1, err := m.selectWorker(testRequestId3, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	w2, err := m.selectWorker(testRequestId4, "text-to-image", "livepeer/model1")
	assert.Nil(err)

	assert.Equal(0, w1.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	assert.Equal(0, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	assert.Equal(2, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)
	// Capacity is zero for model, confirm no workers selected
	w1, err = m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.Nil(w1)
	assert.EqualError(err, ErrNoCompatibleWorkersAvailable.Error())
	//return one capacity, check requestSessions is cleared for request_id
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model1")
	_, requestIDHasWorker := m.requestSessions[testRequestId]
	assert.False(requestIDHasWorker)
	//return another one capacity, check combined capacity is 2
	m.completeAIRequest(testRequestId3, "text-to-image", "livepeer/model1")
	w1Cap := m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity
	w2Cap := m.remoteAIWorkers[1].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity
	assert.Equal(2, w1Cap+w2Cap)
	// return the rest to capacity, check capacity is 4 again
	m.completeAIRequest(testRequestId2, "text-to-image", "livepeer/model1")
	m.completeAIRequest(testRequestId4, "text-to-image", "livepeer/model1")
	w1Cap = m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity
	w2Cap = m.remoteAIWorkers[1].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity
	assert.Equal(4, w1Cap+w2Cap)

	// select model 2 and check capacities
	w2, err = m.selectWorker(testRequestId, "text-to-image", "livepeer/model2")
	assert.Nil(err)
	assert.Equal(2, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	assert.Equal(1, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model2")

	// no ai workers available for unsupported pipeline
	worker, err := m.selectWorker(testRequestId, "new-pipeline", "livepeer/model1")
	assert.NotNil(err)
	assert.Nil(worker)
	m.completeAIRequest(testRequestId, "new-pipeline", "livepeer/model1")

	// capacity does not change if wrong request id
	w2, err = m.selectWorker(testRequestId, "text-to-image", "livepeer/model2")
	assert.Nil(err)
	m.completeAIRequest(testRequestId2, "text-to-image", "liveeer/model2")
	assert.Equal(1, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)
	// capacity returned if correct request id
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model2")
	assert.Equal(2, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)

	// unregister ai worker
	m.liveAIWorkers[strm2].eof <- struct{}{}
	assert.True(wgWait(wg), "Wait timed out for ai worker to terminate")
	assert.Nil(m.liveAIWorkers[strm2])
	assert.NotNil(m.liveAIWorkers[strm])
	// check that model only on disconnected worker is not available
	w, err := m.selectWorker(testRequestId, "text-to-image", "livepeer/model2")
	assert.Nil(w)
	assert.NotNil(err)
	assert.EqualError(err, ErrNoCompatibleWorkersAvailable.Error())

	// reconnect worker and check pipeline only on second worker is available
	go func() { m.Manage(strm2, extraModelCapabilities.ToNetCapabilities(), nil); wg.Done() }()
	time.Sleep(1 * time.Millisecond)
	w, err = m.selectWorker(testRequestId, "image-to-image", "livepeer/model2")
	assert.NotNil(w)
	assert.Nil(err)
	m.completeAIRequest(testRequestId, "image-to-image", "livepeer/model2")
}

func TestManageAIWorkers(t *testing.T) {
	m := NewRemoteAIWorkerManager()
	strm := &StubAIWorkerServer{}
	strm2 := &StubAIWorkerServer{manager: m}

	// sanity check that liveTranscoders and remoteTranscoders is empty
	assert := assert.New(t)
	assert.Nil(m.liveAIWorkers[strm])
	assert.Nil(m.liveAIWorkers[strm2])
	assert.Empty(m.remoteAIWorkers)
	assert.Equal(0, len(m.liveAIWorkers))

	capabilities := createAIWorkerCapabilities()

	// test that transcoder is added to liveTranscoders and remoteTranscoders
	wg1 := newWg(1)
	go func() { m.Manage(strm, capabilities.ToNetCapabilities(), nil); wg1.Done() }()
	time.Sleep(1 * time.Millisecond) // allow the manager to activate

	assert.NotNil(m.liveAIWorkers[strm])
	assert.Len(m.liveAIWorkers, 1)
	assert.Len(m.remoteAIWorkers, 1)
	assert.Equal(2, m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	assert.Equal("TestAddress", m.remoteAIWorkers[0].addr)

	// test that additional transcoder is added to liveTranscoders and remoteTranscoders
	wg2 := newWg(1)
	go func() { m.Manage(strm2, capabilities.ToNetCapabilities(), nil); wg2.Done() }()
	time.Sleep(1 * time.Millisecond) // allow the manager to activate

	assert.NotNil(m.liveAIWorkers[strm])
	assert.NotNil(m.liveAIWorkers[strm2])
	assert.Len(m.liveAIWorkers, 2)
	assert.Len(m.remoteAIWorkers, 2)

	// test that transcoders are removed from liveTranscoders and remoteTranscoders
	m.liveAIWorkers[strm].eof <- struct{}{}
	assert.True(wgWait(wg1)) // time limit
	assert.Nil(m.liveAIWorkers[strm])
	assert.NotNil(m.liveAIWorkers[strm2])
	assert.Len(m.liveAIWorkers, 1)
	assert.Len(m.remoteAIWorkers, 2)

	m.liveAIWorkers[strm2].eof <- struct{}{}
	assert.True(wgWait(wg2)) // time limit
	assert.Nil(m.liveAIWorkers[strm])
	assert.Nil(m.liveAIWorkers[strm2])
	assert.Len(m.liveAIWorkers, 0)
	assert.Len(m.remoteAIWorkers, 2)
}

func TestRemoteAIWorkerTimeout(t *testing.T) {
	m := NewRemoteAIWorkerManager()
	initAIWorker := func() (*RemoteAIWorker, *StubAIWorkerServer) {
		strm := &StubAIWorkerServer{manager: m}
		//create capabilities and constraints the ai worker sends to orch
		caps := createAIWorkerCapabilities()
		wkr := NewRemoteAIWorker(m, strm, caps, nil)
		return wkr, strm
	}
	//create a new worker
	wkr, strm := initAIWorker()
	//create request
	req := worker.GenTextToImageJSONRequestBody{}
	req.Prompt = "a titan carrying steel ball with livepeer logo"

	// check default timeout
	strm.DelayResults = true
	m.taskCount = 1001
	oldTimeout := aiWorkerRequestTimeout
	defer func() { aiWorkerRequestTimeout = oldTimeout }()
	aiWorkerRequestTimeout = 2 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		start := time.Now()
		_, timeoutErr := wkr.Process(context.TODO(), "text-to-image", "livepeer/model", "", AIJobRequestData{Request: req})
		took := time.Since(start)
		assert.Greater(t, took, aiWorkerRequestTimeout)
		assert.NotNil(t, timeoutErr)
		assert.Equal(t, RemoteAIWorkerFatalError{ErrRemoteWorkerTimeout}.Error(), timeoutErr.Error())
		wg.Done()
	}()
	assert.True(t, wgWait(&wg), "worker took too long to timeout")
}

func TestRemoveFromRemoteAIWorkers(t *testing.T) {
	remoteWorkerList := []*RemoteAIWorker{}
	assert := assert.New(t)

	// Create 6 ai workers
	wkr := make([]*RemoteAIWorker, 5)
	for i := 0; i < 5; i++ {
		wkr[i] = &RemoteAIWorker{addr: "testAddress" + strconv.Itoa(i)}
	}

	// Add to list
	remoteWorkerList = append(remoteWorkerList, wkr...)
	assert.Len(remoteWorkerList, 5)

	// Remove ai worker from the head of the list
	remoteWorkerList = removeFromRemoteWorkers(wkr[0], remoteWorkerList)
	assert.Equal(remoteWorkerList[0], wkr[1])
	assert.Equal(remoteWorkerList[1], wkr[2])
	assert.Equal(remoteWorkerList[2], wkr[3])
	assert.Equal(remoteWorkerList[3], wkr[4])
	assert.Len(remoteWorkerList, 4)

	// Remove ai worker from the middle of the list
	remoteWorkerList = removeFromRemoteWorkers(wkr[3], remoteWorkerList)
	assert.Equal(remoteWorkerList[0], wkr[1])
	assert.Equal(remoteWorkerList[1], wkr[2])
	assert.Equal(remoteWorkerList[2], wkr[4])
	assert.Len(remoteWorkerList, 3)

	// Remove ai worker from the middle of the list
	remoteWorkerList = removeFromRemoteWorkers(wkr[2], remoteWorkerList)
	assert.Equal(remoteWorkerList[0], wkr[1])
	assert.Equal(remoteWorkerList[1], wkr[4])
	assert.Len(remoteWorkerList, 2)

	// Remove ai worker from the end of the list
	remoteWorkerList = removeFromRemoteWorkers(wkr[4], remoteWorkerList)
	assert.Equal(remoteWorkerList[0], wkr[1])
	assert.Len(remoteWorkerList, 1)

	// Remove the last ai worker
	remoteWorkerList = removeFromRemoteWorkers(wkr[1], remoteWorkerList)
	assert.Len(remoteWorkerList, 0)

	// Remove a ai worker when list is empty
	remoteWorkerList = removeFromRemoteWorkers(wkr[1], remoteWorkerList)
	emptyTList := []*RemoteAIWorker{}
	assert.Equal(remoteWorkerList, emptyTList)
}
func TestAITaskChan(t *testing.T) {
	n := NewRemoteAIWorkerManager()
	// Sanity check task ID
	if n.taskCount != 0 {
		t.Error("Unexpected taskid")
	}
	if len(n.taskChans) != int(n.taskCount) {
		t.Error("Unexpected task chan length")
	}

	// Adding task chans
	const MaxTasks = 1000
	for i := 0; i < MaxTasks; i++ {
		go n.addTaskChan() // hopefully concurrently...
	}
	for j := 0; j < 10; j++ {
		n.taskMutex.RLock()
		tid := n.taskCount
		n.taskMutex.RUnlock()
		if tid >= MaxTasks {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n.taskCount != MaxTasks {
		t.Error("Time elapsed")
	}
	if len(n.taskChans) != int(n.taskCount) {
		t.Error("Unexpected task chan length")
	}

	// Accessing task chans
	existingIds := []int64{0, 1, MaxTasks / 2, MaxTasks - 2, MaxTasks - 1}
	for _, id := range existingIds {
		_, err := n.getTaskChan(int64(id))
		if err != nil {
			t.Error("Unexpected error getting task chan for ", id, err)
		}
	}
	missingIds := []int64{-1, MaxTasks}
	testNonexistentChans := func(ids []int64) {
		for _, id := range ids {
			_, err := n.getTaskChan(int64(id))
			if err == nil || err.Error() != "No AI Worker channel" {
				t.Error("Did not get expected error for ", id, err)
			}
		}
	}
	testNonexistentChans(missingIds)

	// Removing task chans
	for i := 0; i < MaxTasks; i++ {
		go n.removeTaskChan(int64(i)) // hopefully concurrently...
	}
	for j := 0; j < 10; j++ {
		n.taskMutex.RLock()
		tlen := len(n.taskChans)
		n.taskMutex.RUnlock()
		if tlen <= 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(n.taskChans) != 0 {
		t.Error("Time elapsed")
	}
	testNonexistentChans(existingIds) // sanity check for removal
}
func TestCheckAICapacity(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	o := NewOrchestrator(n, nil)
	wkr := stubAIWorker{}
	n.Capabilities = createAIWorkerCapabilities()
	n.AIWorker = &wkr
	// Test when local AI worker has capacity
	hasCapacity, releaseCapacity := o.CheckAICapacity("text-to-image", "livepeer/model1")
	assert.True(t, hasCapacity)
	releaseCapacity <- true

	o.node.AIWorker = nil
	o.node.AIWorkerManager = NewRemoteAIWorkerManager()
	initAIWorker := func() (*RemoteAIWorker, *StubAIWorkerServer) {
		strm := &StubAIWorkerServer{manager: o.node.AIWorkerManager}
		caps := createAIWorkerCapabilities()
		wkr := NewRemoteAIWorker(o.node.AIWorkerManager, strm, caps, nil)
		return wkr, strm
	}
	//create worker and connect to manager
	wkr2, strm := initAIWorker()

	go func() {
		o.node.AIWorkerManager.Manage(strm, wkr2.capabilities.ToNetCapabilities(), nil)
	}()
	time.Sleep(1 * time.Millisecond) // allow the workers to activate

	hasCapacity, releaseCapacity = o.CheckAICapacity("text-to-image", "livepeer/model1")
	assert.True(t, hasCapacity)
	assert.NotNil(t, releaseCapacity)
	releaseCapacity <- true

	// Test when remote AI worker does not have capacity
	hasCapacity, releaseCapacity = o.CheckAICapacity("text-to-image", "livepeer/model2")
	assert.False(t, hasCapacity)
	assert.Nil(t, releaseCapacity)
}
func TestRemoteAIWorkerProcessPipelines(t *testing.T) {
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities = NewCapabilities(DefaultCapabilities(), nil)
	n.Capabilities.version = "1.0"
	n.Capabilities.SetPerCapabilityConstraints(make(PerCapabilityConstraints))
	n.AIWorkerManager = NewRemoteAIWorkerManager()
	o := NewOrchestrator(n, nil)

	initAIWorker := func() (*RemoteAIWorker, *StubAIWorkerServer) {
		strm := &StubAIWorkerServer{manager: o.node.AIWorkerManager}
		caps := createAIWorkerCapabilities()
		wkr := NewRemoteAIWorker(o.node.AIWorkerManager, strm, caps, nil)
		return wkr, strm
	}
	//create worker and connect to manager
	wkr, strm := initAIWorker()
	go o.node.serveAIWorker(strm, wkr.capabilities.ToNetCapabilities(), nil)
	time.Sleep(5 * time.Millisecond) // allow the workers to activate

	//check workers connected
	assert.Equal(t, 1, len(o.node.AIWorkerManager.remoteAIWorkers))
	assert.NotNil(t, o.node.AIWorkerManager.liveAIWorkers[strm])

	//test text-to-image
	modelID := "livepeer/model1"
	req := worker.GenTextToImageJSONRequestBody{}
	req.Prompt = "a titan carrying steel ball with livepeer logo"
	req.ModelId = &modelID
	o.CreateStorageForRequest("request_id1")
	res, err := o.TextToImage(context.TODO(), "request_id1", req)
	results, ok := res.(worker.ImageResponse)
	assert.True(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, "/stream/request_id1/image_url", results.Images[0].Url)
	// remove worker
	wkr.eof <- struct{}{}
	time.Sleep(1 * time.Second)

}
func TestReserveAICapability(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities = createAIWorkerCapabilities()

	pipeline := "audio-to-text"
	modelID := "livepeer/model1"

	// Add AI capability and model
	caps := NewCapabilities(DefaultCapabilities(), nil)
	caps.SetPerCapabilityConstraints(PerCapabilityConstraints{
		Capability_AudioToText: {
			Models: ModelConstraints{
				modelID: {Warm: true, Capacity: 2},
			},
		},
	})
	n.AddAICapabilities(caps)

	// Reserve AI capability
	err := n.ReserveAICapability(pipeline, modelID)
	assert.Nil(t, err)

	// Check capacity is reduced
	cap := n.Capabilities.constraints.perCapability[Capability_AudioToText]
	assert.Equal(t, 1, cap.Models[modelID].Capacity)

	// Reserve AI capability again
	err = n.ReserveAICapability(pipeline, modelID)
	assert.Nil(t, err)

	// Check capacity is further reduced
	cap = n.Capabilities.constraints.perCapability[Capability_AudioToText]
	assert.Equal(t, 0, cap.Models[modelID].Capacity)

	// Reserve AI capability when capacity is already zero
	err = n.ReserveAICapability(pipeline, modelID)
	assert.NotNil(t, err)
	assert.EqualError(t, err, fmt.Sprintf("failed to reserve AI capability capacity, model capacity is 0 pipeline=%v modelID=%v", pipeline, modelID))

	// Reserve AI capability for non-existent pipeline
	err = n.ReserveAICapability("invalid-pipeline", modelID)
	assert.NotNil(t, err)
	assert.EqualError(t, err, "pipeline not available")

	// Reserve AI capability for non-existent model
	err = n.ReserveAICapability(pipeline, "invalid-model")
	assert.NotNil(t, err)
	assert.EqualError(t, err, fmt.Sprintf("failed to reserve AI capability capacity, model does not exist pipeline=%v modelID=invalid-model", pipeline))
}

func createAIWorkerCapabilities() *Capabilities {
	//create capabilities and constraints the ai worker sends to orch
	constraints := make(PerCapabilityConstraints)
	constraints[Capability_TextToImage] = &CapabilityConstraints{Models: make(ModelConstraints)}
	constraints[Capability_TextToImage].Models["livepeer/model1"] = &ModelConstraint{Warm: true, Capacity: 2}
	caps := NewCapabilities(DefaultCapabilities(), MandatoryOCapabilities())
	caps.SetPerCapabilityConstraints(constraints)
	caps.version = "1.0"
	return caps
}

type stubAIWorker struct{}

func (a *stubAIWorker) GetLiveAICapacity(pipeline, modelID string) worker.Capacity {
	return worker.Capacity{}
}

func (a *stubAIWorker) TextToImage(ctx context.Context, req worker.GenTextToImageJSONRequestBody) (*worker.ImageResponse, error) {
	return &worker.ImageResponse{
		Images: []worker.Media{
			{Url: "http://example.com/image.png"},
		},
	}, nil
}

func (a *stubAIWorker) ImageToImage(ctx context.Context, req worker.GenImageToImageMultipartRequestBody) (*worker.ImageResponse, error) {
	return &worker.ImageResponse{
		Images: []worker.Media{
			{Url: "http://example.com/image.png"},
		},
	}, nil
}

func (a *stubAIWorker) ImageToVideo(ctx context.Context, req worker.GenImageToVideoMultipartRequestBody) (*worker.VideoResponse, error) {
	return &worker.VideoResponse{
		Frames: [][]worker.Media{
			{
				{Url: "http://example.com/frame1.png", Nsfw: false},
				{Url: "http://example.com/frame2.png", Nsfw: false},
			},
			{
				{Url: "http://example.com/frame3.png", Nsfw: false},
				{Url: "http://example.com/frame4.png", Nsfw: false},
			},
		},
	}, nil
}

func (a *stubAIWorker) Upscale(ctx context.Context, req worker.GenUpscaleMultipartRequestBody) (*worker.ImageResponse, error) {
	return &worker.ImageResponse{
		Images: []worker.Media{
			{Url: "http://example.com/image.png"},
		},
	}, nil
}

func (a *stubAIWorker) AudioToText(ctx context.Context, req worker.GenAudioToTextMultipartRequestBody) (*worker.TextResponse, error) {
	return &worker.TextResponse{Text: "Transcribed text"}, nil
}

func (a *stubAIWorker) SegmentAnything2(ctx context.Context, req worker.GenSegmentAnything2MultipartRequestBody) (*worker.MasksResponse, error) {
	return &worker.MasksResponse{Logits: "logits", Masks: "masks", Scores: "scores"}, nil
}

func (a *stubAIWorker) LLM(ctx context.Context, req worker.GenLLMJSONRequestBody) (interface{}, error) {
	var choices []worker.LLMChoice
	choices = append(choices, worker.LLMChoice{Delta: &worker.LLMMessage{Content: "choice1", Role: "assistant"}, Index: 0})
	tokensUsed := worker.LLMTokenUsage{PromptTokens: 40, CompletionTokens: 10, TotalTokens: 50}
	return &worker.LLMResponse{Choices: choices, Created: 1, Model: "llm_model", Usage: tokensUsed}, nil
}

func (a *stubAIWorker) ImageToText(ctx context.Context, req worker.GenImageToTextMultipartRequestBody) (*worker.ImageToTextResponse, error) {
	return &worker.ImageToTextResponse{Text: "Transcribed text"}, nil
}

func (a *stubAIWorker) TextToSpeech(ctx context.Context, req worker.GenTextToSpeechJSONRequestBody) (*worker.AudioResponse, error) {
	return &worker.AudioResponse{Audio: worker.MediaURL{Url: "http://example.com/audio.wav"}}, nil
}

func (a *stubAIWorker) LiveVideoToVideo(ctx context.Context, req worker.GenLiveVideoToVideoJSONRequestBody) (*worker.LiveVideoToVideoResponse, error) {
	return &worker.LiveVideoToVideoResponse{}, nil
}

func (a *stubAIWorker) Warm(ctx context.Context, arg1, arg2 string, endpoint worker.RunnerEndpoint, flags worker.OptimizationFlags) error {
	return nil
}

func (a *stubAIWorker) Stop(ctx context.Context) error {
	return nil
}

func (a *stubAIWorker) HasCapacity(pipeline, modelID string) bool {
	return true
}

func (a *stubAIWorker) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	return nil
}

func (a *stubAIWorker) HardwareInformation() []worker.HardwareInformation {
	return nil
}

func (a *stubAIWorker) Version() []worker.Version {
	return nil
}

type StubAIWorkerServer struct {
	manager      *RemoteAIWorkerManager
	SendError    error
	JobError     error
	DelayResults bool

	common.StubServerStream
}

func (s *StubAIWorkerServer) Send(n *net.NotifyAIJob) error {
	var images []worker.Media
	media := worker.Media{Nsfw: false, Seed: 111, Url: "image_url"}
	images = append(images, media)
	res := RemoteAIWorkerResult{
		Results: worker.ImageResponse{Images: images},
		Files:   make(map[string][]byte),
		Err:     nil,
	}
	if s.JobError != nil {
		res.Err = s.JobError
	}
	if s.SendError != nil {
		return s.SendError
	}

	if !s.DelayResults {
		s.manager.aiResults(n.TaskId, &res)
	}

	return nil

}

// Utility function to create a temporary file for file-based configurations
func mockFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock file: %v", err)
	}
	return filePath
}

func TestParseAIModelConfigs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		fileData    string
		expected    []AIModelConfig
		expectedErr string
	}{{
		name:  "Valid Inline String Config",
		input: "pipeline1:model1:true,pipeline2:model2:false",
		expected: []AIModelConfig{
			{Pipeline: "pipeline1", ModelID: "model1", Warm: true},
			{Pipeline: "pipeline2", ModelID: "model2", Warm: false},
		},
	},
		{
			name:        "Invalid Inline String Config Missing Parts",
			input:       "pipeline1:model1",
			expectedErr: "invalid AI model config expected <pipeline>:<model_id>:<warm>",
		},
		{
			name:     "Valid File-Based Config",
			fileData: `[{"pipeline": "pipeline1", "model_id": "model1", "warm": true}, {"pipeline": "pipeline2", "model_id": "model2", "warm": false}]`,
			expected: []AIModelConfig{
				{Pipeline: "pipeline1", ModelID: "model1", Warm: true},
				{Pipeline: "pipeline2", ModelID: "model2", Warm: false},
			},
		},
		{
			name:        "Invalid File Config Corrupted JSON",
			fileData:    `[{"pipeline": "pipeline1", "model_id": "model1", "warm": true`,
			expectedErr: "unexpected end of JSON input",
		},
		{
			name:        "File Not Found",
			input:       "nonexistent.json",
			expectedErr: "invalid AI model config expected <pipeline>:<model_id>:<warm>",
		},
		{
			name:        "Invalid Boolean Value in Inline String Config",
			input:       "pipeline1:model1:invalid_bool",
			expectedErr: "strconv.ParseBool: parsing \"invalid_bool\": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []AIModelConfig
			var err error

			// Mock file handling if fileData is provided
			if tt.fileData != "" {
				mockFilePath := mockFile(t, tt.fileData)
				result, err = ParseAIModelConfigs(mockFilePath)
			} else {
				result, err = ParseAIModelConfigs(tt.input)
			}

			// Verify error messages match
			assert := assert.New(t)
			if tt.expectedErr != "" {
				assert.Equal(err.Error(), tt.expectedErr)
				assert.Empty(result, err)
			} else {
				assert.Empty(err)
				assert.Equal(tt.expected, result)
			}
		})
	}
}

func TestHardwareInformationFromNetHardware(t *testing.T) {
	netHdwDetail := net.GPUComputeInfo{Id: "gpu-1", Name: "gpu name", Major: 8, Minor: 9, MemoryFree: 1, MemoryTotal: 10}
	netHdwInfo := make(map[string]*net.GPUComputeInfo)
	netHdwInfo["0"] = &netHdwDetail
	netHdw := net.HardwareInformation{Pipeline: "livepeer-pipeline", ModelId: "livepeer/model1", GpuInfo: netHdwInfo}
	var netHdwList []*net.HardwareInformation
	netHdwList = append(netHdwList, &netHdw)
	//create []worker.HardwareInformation
	hdwList := hardwareInformationFromNetHardware(netHdwList)

	netHdwJson, _ := json.Marshal(netHdwList)
	hdwJson, _ := json.Marshal(hdwList)

	var hdw1, hdw2 interface{}
	json.Unmarshal(netHdwJson, &hdw1)
	json.Unmarshal(hdwJson, &hdw2)
	assert.True(t, reflect.DeepEqual(hdw1, hdw2))

}
