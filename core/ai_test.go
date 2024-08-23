package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/livepeer/ai-worker/worker"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/net"

	"github.com/stretchr/testify/assert"
)

func TestPipelineToCapability(t *testing.T) {
	good := "audio-to-text"
	bad := "i-love-tests"

	cap, err := PipelineToCapability(good)
	assert.Nil(t, err)
	assert.Equal(t, cap, Capability_AudioToText)

	cap, err = PipelineToCapability(bad)
	assert.Error(t, err)
	assert.Equal(t, cap, Capability_Unused)
}

func TestServeAIWorker(t *testing.T) {
	n, _ := NewLivepeerNode(nil, "", nil)
	n.Capabilities.SetPerCapabilityConstraints(make(PerCapabilityConstraints))

	n.AIWorkerManager = NewRemoteAIWorkerManager()
	strm := &StubAIWorkerServer{}

	// test that a ai worker was created
	caps := createAIWorkerCapabilities()
	netCaps := caps.ToNetCapabilities()
	go n.serveAIWorker(strm, netCaps)
	time.Sleep(1 * time.Second)

	wkr, ok := n.AIWorkerManager.liveAIWorkers[strm]
	if !ok {
		t.Error("Unexpected transcoder type")
	}

	// test shutdown
	wkr.eof <- struct{}{}
	time.Sleep(1 * time.Second)

	// stream should be removed
	_, ok = n.AIWorkerManager.liveAIWorkers[strm]
	if ok {
		t.Error("Unexpected ai worker presence")
	}
}
func TestRemoteAIWorkerManager(t *testing.T) {
	m := NewRemoteAIWorkerManager()
	initAIWorker := func() (*RemoteAIWorker, *StubAIWorkerServer) {
		strm := &StubAIWorkerServer{manager: m}
		caps := createAIWorkerCapabilities()
		netCaps := caps.ToNetCapabilities()
		wkr := NewRemoteAIWorker(m, strm, netCaps.Constraints.PerCapability, caps)
		return wkr, strm
	}
	//create worker and connect to manager
	wkr, strm := initAIWorker()

	go func() {
		m.Manage(strm, wkr.capabilities.ToNetCapabilities())
	}()
	time.Sleep(1 * time.Millisecond) // allow the workers to activate

	//check workers connected
	assert.Equal(t, 1, len(m.remoteAIWorkers))
	assert.NotNil(t, m.liveAIWorkers[strm])
	//create request
	req := worker.TextToImageJSONRequestBody{}
	req.Prompt = "a titan carrying steel ball with livepeer logo"

	// happy path
	res, err := m.Process(context.TODO(), "request_id1", "text-to-image", "livepeer/model", "", req)
	results, ok := res.Results.(worker.ImageResponse)
	assert.True(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, "image_url", results.Images[0].Url)

	// error on remote
	strm.JobError = fmt.Errorf("JobError")
	res, err = m.Process(context.TODO(), "request_id2", "text-to-image", "livepeer/model", "", req)
	assert.NotNil(t, err)
	strm.JobError = nil

	//check worker is still connected
	assert.Equal(t, 1, len(m.remoteAIWorkers))

	// simulate error with sending
	// m.Process keeps retrying since error is not fatal
	strm.SendError = ErrNoWorkersAvailable
	_, err = m.Process(context.TODO(), "request_id3", "text-to-image", "livepeer/model", "", req)
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

	LivepeerVersion = "0.4.1"
	capabilities := createAIWorkerCapabilities()
	LivepeerVersion = "undefined"

	extraModelCapabilities := capabilities
	extraModelCapabilities.constraints.perCapability[Capability_TextToImage].Models["livpeer/model2"] = &ModelConstraint{Warm: true, Capacity: 2}

	// sanity check that ai worker is not in liveAIWorkers or remoteAIWorkers
	assert := assert.New(t)
	assert.Nil(m.liveAIWorkers[strm])
	assert.Empty(m.remoteAIWorkers)

	// register ai workers, which adds ai worker to liveAIWorkers and remoteAIWorkers
	wg := newWg(1)
	go func() { m.Manage(strm, capabilities.ToNetCapabilities()) }()
	time.Sleep(1 * time.Millisecond) // allow time for first stream to register
	go func() { m.Manage(strm2, extraModelCapabilities.ToNetCapabilities()); wg.Done() }()
	time.Sleep(1 * time.Millisecond) // allow time for second stream to register e for third stream to register

	assert.NotNil(m.liveAIWorkers[strm])
	assert.NotNil(m.liveAIWorkers[strm2])
	assert.Len(m.remoteAIWorkers, 2)

	testRequestId := "testID"
	testRequestId2 := "testID2"

	// assert ai worker is returned from selectAIWorker
	currentWorker, err := m.selectWorker(testRequestId, "text-to-image", "livepeer/model2")
	assert.Nil(err)
	assert.NotNil(currentWorker)
	assert.NotNil(m.liveAIWorkers[strm])
	assert.Len(m.remoteAIWorkers, 2)

	// assert that ai workers are selected according to capabilities
	m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"] = &ModelConstraint{Warm: true, Capacity: 1}
	w1, err := m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	assert.Equal(0, w1.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model1")
	assert.Equal(1, w1.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)

	w2, err := m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.Nil(err)
	assert.Equal(0, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	m.completeAIRequest(testRequestId2, "text-to-image", "livepeer/model2")
	assert.Equal(1, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)
	assert.Nil(err)
	assert.NotEqual(w1.addr, w2.addr)
	m.completeAIRequest(testRequestId, "text-to-image", "livepeer/model1")
	m.completeAIRequest(testRequestId2, "text-to-image", "livepeer/model2")
	assert.Equal(1, w1.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	assert.Equal(2, w2.capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model2"].Capacity)

	// assert no ai workers available for unsupported pipeline
	worker, err := m.selectWorker(testRequestId, "new-pipeline", "livepeer/model")
	assert.NotNil(err)
	assert.Nil(worker)
	m.completeAIRequest(testRequestId, "new-pipeline", "livepeer/model1")

	// assert no ai workers returned if no capacity
	w1, err = m.selectWorker(testRequestId, "text-to-image", "livepeer/model1")
	assert.NotNil(w1)
	w2, err = m.selectWorker(testRequestId2, "text-to-image", "livepeer/model1")
	assert.Nil(w2)
	assert.Equal(err, ErrNoCompatibleWorkersAvailable)

	// assert capacity does not change if wrong request id
	m.completeAIRequest(testRequestId2, "text-to-image", "liveeer/model1")
	assert.Equal(0, m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)
	m.completeAIRequest(testRequestId, "text-to-image", "liveeer/model1")
	assert.Equal(1, m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model1"].Capacity)

	// unregister ai worker
	m.liveAIWorkers[strm2].eof <- struct{}{}
	assert.True(wgWait(wg), "Wait timed out for ai worker to terminate")
	assert.Nil(m.liveAIWorkers[strm2])
	assert.NotNil(m.liveAIWorkers[strm])

	// assert one transcoder with the correct Livepeer version is selected
	//minVersionCapabilities := NewCapabilities(DefaultCapabilities(), []Capability{})
	//minVersionCapabilities.SetMinVersionConstraint("0.4.0")
	//currentTranscoder, err = m.selectTranscoder(testSessionId, minVersionCapabilities)
	//assert.Nil(err)
	//m.completeStreamSession(testSessionId)

	// assert no transcoders available for min version higher than any transcoder
	//minVersionHighCapabilities := NewCapabilities(DefaultCapabilities(), []Capability{})
	//minVersionHighCapabilities.SetMinVersionConstraint("0.4.2")
	//currentTranscoder, err = m.selectTranscoder(testSessionId, minVersionHighCapabilities)
	//assert.NotNil(err)
	//m.completeStreamSession(testSessionId)
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
	go func() { m.Manage(strm, capabilities.ToNetCapabilities()); wg1.Done() }()
	time.Sleep(1 * time.Millisecond) // allow the manager to activate

	assert.NotNil(m.liveAIWorkers[strm])
	assert.Len(m.liveAIWorkers, 1)
	assert.Len(m.remoteAIWorkers, 1)
	assert.Equal(2, m.remoteAIWorkers[0].capabilities.constraints.perCapability[Capability_TextToImage].Models["livepeer/model"].Capacity)
	assert.Equal("TestAddress", m.remoteAIWorkers[0].addr)

	// test that additional transcoder is added to liveTranscoders and remoteTranscoders
	wg2 := newWg(1)
	go func() { m.Manage(strm2, capabilities.ToNetCapabilities()); wg2.Done() }()
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
		netCaps := caps.ToNetCapabilities()
		wkr := NewRemoteAIWorker(m, strm, netCaps.Constraints.PerCapability, caps)
		return wkr, strm
	}
	//create a new worker
	wkr, strm := initAIWorker()
	//create request
	req := worker.TextToImageJSONRequestBody{}
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
		_, timeoutErr := wkr.Process(context.TODO(), "text-to-image", "livepeer/model", "", req)
		took := time.Since(start)
		assert.Greater(t, took, aiWorkerRequestTimeout)
		assert.NotNil(t, timeoutErr)
		assert.Equal(t, RemoteAIWorkerFatalError{ErrRemoteWorkerTimeout}.Error(), timeoutErr.Error())
		wg.Done()
	}()
	assert.True(t, wgWait(&wg), "worker took too long to timeout")
}

func createAIWorkerCapabilities() *Capabilities {
	//create capabilities and constraints the ai worker sends to orch
	constraints := make(PerCapabilityConstraints)
	constraints[Capability_TextToImage] = &CapabilityConstraints{Models: make(ModelConstraints)}
	constraints[Capability_TextToImage].Models["livepeer/model"] = &ModelConstraint{Warm: true, Capacity: 2}
	caps := NewCapabilities(DefaultCapabilities(), MandatoryOCapabilities())
	caps.SetPerCapabilityConstraints(constraints)

	return caps
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
