package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"pgregory.net/rapid"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/drivers"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

func TestPull_ProcessLine(t *testing.T) {
	// test input lines of this expected format:
	// <path-to-segment>,<start-time>,<end-time>

	// preliminaries
	assert := assert.New(t)
	tmpdir, err := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)
	assert.Nil(err)
	fname := filepath.Join(tmpdir, "0.ts")
	err = ioutil.WriteFile(fname, []byte("hello"), 0644)
	assert.Nil(err)
	var expectedDur *float64
	oldFunc := processSegmentFunc
	defer func() { processSegmentFunc = oldFunc }()
	processSegmentFunc = func(cxn *rtmpConnection, seg *stream.HLSSegment) ([]string, error) {
		if expectedDur != nil {
			if !assert.InDelta(*expectedDur, seg.Duration, 0.0001) {
				return nil, errors.New("mismatched duration")
			}
		}
		return nil, nil
	}

	cxn := &rtmpConnection{}

	// test invalid part length : 0, 1, 4
	err = process(cxn, "one part")
	assert.Equal(invalidSegInfo, err)
	err = process(cxn, "two, parts")
	assert.Equal(invalidSegInfo, err)
	err = process(cxn, "four, parts, still, invalid")
	assert.Equal(invalidSegInfo, err)

	// test invalid seq no
	err = process(cxn, "invalid file,1,2")
	assert.Contains(err.Error(), "invalid segment seq:")

	// test acceptable seq numbers for filename
	// verify this by ensuring that it attempts to read the given file
	err = process(cxn, "0,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "0.ext,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "path/0,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "relative/path/to/0,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "relative/path/to/0.ext,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "/absolute/path/0,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "/absolute/path/to/0,1,2")
	assert.Contains(err.Error(), "could not read file:")
	err = process(cxn, "/absolute/path/to/0.ext,1,2")
	assert.Contains(err.Error(), "could not read file:")

	// test invalid begin time
	err = process(cxn, "0, 1,2")
	assert.Contains(err.Error(), "invalid segment begin time:")
	err = process(cxn, "0,1.a,2")
	assert.Contains(err.Error(), "invalid segment begin time:")

	// test invalid end time
	err = process(cxn, fname+",1, 2")
	assert.Contains(err.Error(), "invalid segment end time:")
	err = process(cxn, fname+",1,a")
	assert.Contains(err.Error(), "invalid segment end time:")

	// test valid begin, end times and durations
	rapid.Check(t, func(t *rapid.T) {
		begin := rapid.Float64().Draw(t, "begin").(float64)
		end := rapid.Float64().Draw(t, "end").(float64)
		dur := end - begin
		expectedDur = &dur
		err = process(cxn, fmt.Sprintf("%s,%f,%f", fname, begin, end))
		assert.Nil(err)
	})
}

func TestPull_Registration(t *testing.T) {
	// preliminaries
	assert := assert.New(t)
	ffmpeg.InitFFmpeg()
	defer goleak.VerifyNone(t, ignoreRoutines()...)
	tmpdir, err := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)
	assert.Nil(err)
	oldStorage := drivers.NodeStorage
	defer func() { drivers.NodeStorage = oldStorage }()
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	oldWorkers := nbWorkers
	nbWorkers = 5
	defer func() { nbWorkers = oldWorkers }()
	n, err := core.NewLivepeerNode(nil, tmpdir, nil)
	assert.Nil(err)
	s, err := NewLivepeerServer("", n, false, "")
	assert.Nil(err)

	waitMu := &sync.Mutex{}
	doWait := true
	wait := &doWait
	segmentCh := make(chan struct{})
	oldFunc := processSegmentFunc
	defer func() { processSegmentFunc = oldFunc }()
	processSegmentFunc = func(cxn *rtmpConnection, seg *stream.HLSSegment) ([]string, error) {
		waitMu.Lock()
		w := *wait
		waitMu.Unlock()
		if w {
			<-segmentCh
		}
		return nil, nil
	}

	// ensure mid does not exist
	mid := core.RandomManifestID()
	s.connectionLock.Lock()
	assert.NotContains(mid, s.rtmpConnections)
	s.connectionLock.Unlock()

	// ensure pull dir does not exist
	_, err = os.Stat(filepath.Join(tmpdir, "media", string(mid)))
	assert.IsType(&os.PathError{}, err)

	// begin pull process
	pullCompleteChan := make(chan struct{})
	go func() {
		err := s.Pull("test.flv", mid)
		assert.Nil(err)
		pullCompleteChan <- struct{}{}
	}()

	// ensure mid now exists
	runLoop := true
	midChecked := false
	for runLoop {
		select {
		case segmentCh <- struct{}{}:
			s.connectionLock.Lock()
			assert.Contains(s.rtmpConnections, mid)
			// check pull dir now exists
			stat, err := os.Stat(filepath.Join(tmpdir, "media", string(mid)))
			assert.Nil(err)
			assert.True(stat.IsDir(), "pull dir does not exist")
			s.connectionLock.Unlock()
			midChecked = true
			waitMu.Lock()
			doWait = false
			waitMu.Unlock()
			break
		case <-pullCompleteChan:
			runLoop = false
			break
		default:
		}
	}

	// ensure mid is removed
	s.connectionLock.Lock()
	assert.NotContains(mid, s.rtmpConnections)
	// ensure pull dir has been removed
	_, err = os.Stat(filepath.Join(tmpdir, "media", string(mid)))
	assert.IsType(&os.PathError{}, err)
	s.connectionLock.Unlock()
	assert.True(midChecked, "manifest ID presence was not checked")
}

func TestPull_RunPull(t *testing.T) {
	assert := assert.New(t)
	ffmpeg.InitFFmpeg()
	defer goleak.VerifyNone(t, ignoreRoutines()...)
	oldStorage := drivers.NodeStorage
	defer func() { drivers.NodeStorage = oldStorage }()
	drivers.NodeStorage = drivers.NewMemoryDriver(nil)
	oldWorkers := nbWorkers
	nbWorkers = 5
	defer func() { nbWorkers = oldWorkers }()
	tmpdir, err := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpdir)
	assert.Nil(err)
	oldFunc := processSegmentFunc
	defer func() { processSegmentFunc = oldFunc }()
	mu := &sync.Mutex{}
	segCount := 0
	processSegmentFunc = func(cxn *rtmpConnection, seg *stream.HLSSegment) ([]string, error) {
		time.Sleep(time.Millisecond) // just enough to make the thread yield
		mu.Lock()
		defer mu.Unlock()
		segCount++
		return nil, nil
	}

	n, err := core.NewLivepeerNode(nil, tmpdir, nil)
	assert.Nil(err)
	s, err := NewLivepeerServer("", n, false, "")
	assert.Nil(err)

	// invalid 'video' file
	err = s.Pull("pull_test.go", core.RandomManifestID())
	assert.Contains(err.Error(), "pull segmenter err=Invalid data found")

	// nonexistent file
	err = s.Pull("nonexistent", core.RandomManifestID())
	assert.Equal("pull segmenter err=No such file or directory", err.Error())

	// invalid work dir
	n.WorkDir = "pull_test.go"
	err = s.Pull("test.flv", core.RandomManifestID())
	assert.Contains(err.Error(), "creating pull directory err=mkdir")
	n.WorkDir = tmpdir

	// normal cases. run a bunch of concurrents
	wg := &sync.WaitGroup{}
	nbConc := 75 // Reduce if hitting "too many open files" frequently
	for i := 0; i < nbConc; i++ {
		wg.Add(1)
		go func() {
			err := s.Pull("test.flv", core.RandomManifestID())
			assert.Nil(err)
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(10*nbConc, segCount)
}

// state machine checks for linked list
type segMachine struct {
	segList      *segList
	segLen       int      // length of the segment list
	machineState []string // complete history of all inserts
	machinePos   int      // current head
}

func (sm *segMachine) Init(t *rapid.T) {
	sm.segList = &segList{}
	sm.machineState = []string{}
}
func (sm *segMachine) Insert(t *rapid.T) {
	val := rapid.String().String()
	sm.segList.Insert(val)
	sm.segLen++
	sm.machineState = append(sm.machineState, val)
}

func (sm *segMachine) Remove(t *rapid.T) {
	assert := assert.New(t)
	val, err := sm.segList.Remove()
	if sm.segLen == 0 {
		assert.Empty(val)
		assert.Equal(errSegListEmpty, err)
		return
	}
	assert.Equal(sm.machineState[sm.machinePos], val)
	sm.segLen--
	sm.machinePos++
}
func (sm *segMachine) Check(t *rapid.T) {
	assert := assert.New(t)
	if sm.segLen == 0 {
		assert.Nil(sm.segList.head)
		return
	}
	// ensure head / tail matches
	assert.Equal(sm.machineState[sm.machinePos], sm.segList.head.val)
	assert.Equal(sm.machineState[len(sm.machineState)-1], sm.segList.tail.val)

	// ensure all intermediate elements match
	cursor := sm.segList.head
	for i := 1; i < sm.segLen; i++ {
		cursor = cursor.next
		assert.NotNil(cursor)
		assert.Equal(sm.machineState[sm.machinePos+i], cursor.val)
	}
	assert.Equal(cursor, sm.segList.tail)
	assert.Nil(sm.segList.tail.next)
}

func TestPull_SegmentList(t *testing.T) {
	rapid.Check(t, rapid.Run(&segMachine{}))
}

func TestPull_ConcurrentSegmentList(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		assert := assert.New(t)
		nbWorkers := rapid.IntRange(1, 25).Draw(t, "nbWorkers").(int)
		nbInserts := rapid.IntRange(0, 1000).Draw(t, "nbInserts").(int)
		chComplete := make(chan struct{}, nbInserts)
		segList := MakeSegList()
		wg := sync.WaitGroup{}
		for i := 0; i < nbWorkers; i++ {
			wg.Add(1)
			go func() {
				defer func() { wg.Done() }()
				for {
					_, err := segList.Remove()
					if err == errSegListComplete {
						return
					} else {
						assert.Nil(err)
					}
					chComplete <- struct{}{}
				}
			}()
		}

		// count removals - do this in a thread to ensure we can time out
		go func() {
			wg.Add(1)
			for i := 0; i < nbInserts; i++ {
				<-chComplete
			}
			wg.Done()
		}()

		// insert. do each insert in a separate thread to stress concurrency
		wg.Add(nbInserts)
		insertWg := sync.WaitGroup{}
		insertWg.Add(nbInserts)
		for i := 0; i < nbInserts; i++ {
			go func() {
				defer func() { insertWg.Done(); wg.Done() }()
				segList.Insert(rapid.String().String())
			}()
		}
		assert.True(wgWait(&insertWg), "insert waitgroup timed out")
		segList.MarkComplete() // completed inserting

		assert.True(wgWait(&wg), "main waitgroup timed out")
		assert.True(segList.sl.Empty(), "list was not completely drained")
	})
}
