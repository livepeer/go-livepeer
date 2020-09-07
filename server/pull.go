package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"

	"github.com/livepeer/lpms/ffmpeg"
	"github.com/livepeer/lpms/stream"
)

type segs struct {
	val  string
	next *segs
}

var invalidSegInfo = errors.New("invalid segment info")

var processSegmentFunc = processSegment

// Control how many segments to transcode in parallel for a given pull.
// Set to 1 until we have a better way of modulating the concurrency factor
// eg, by looking at the number of segments allowed inflight
// Roughly ~4 for a full lineup of 8 orchestrators but this could well vary
// Note that unit tests set this to a higher number to ensure things work
var nbWorkers = 1

func (s *LivepeerServer) Pull(fname string, streamName core.ManifestID) error {
	glog.V(common.VERBOSE).Info(fmt.Sprintf("Pulling fname=%s workers=%d profiles=%v", fname, nbWorkers, BroadcastJobVideoProfiles))

	// TODO this isn't rtmp anymore
	strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: streamName, Profiles: BroadcastJobVideoProfiles})

	cxn, err := s.registerConnection(strm)
	if err != nil {
		return err
	}

	base := []string{s.LivepeerNode.WorkDir, "media", string(cxn.params.ManifestID)}
	baseDir := filepath.Join(base...)
	err = os.MkdirAll(baseDir, 0744)
	if err != nil {
		return fmt.Errorf("creating pull directory err=%w", err)
	}
	defer os.RemoveAll(baseDir) // TODO integrate with proper filesystem OS

	if err := runSegmenter(s.LivepeerNode, cxn, fname); err != nil {
		return err
	}
	if err := removeRTMPStream(s, cxn.mid); err != nil {
		return err
	}
	// TODO remove based on prefix?
	return nil
}

// segInfo is a csv style string containing:
// <path-to-segment>,<start-time>,<end-time>
func process(cxn *rtmpConnection, segInfo string) error {
	// TODO is it possible that we have commas as part of file path?
	// csv should quote the string. TODO use a proper csv parser.
	parts := strings.Split(strings.TrimSpace(segInfo), ",")
	glog.V(common.VERBOSE).Info(parts)
	if len(parts) != 3 {
		return invalidSegInfo
	}
	segPath := parts[0]
	fname := filepath.Base(segPath)
	seq, err := strconv.Atoi(strings.TrimSuffix(fname, filepath.Ext(fname)))
	if err != nil {
		return fmt.Errorf("invalid segment seq: %w", err)
	}
	segBegin, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return fmt.Errorf("invalid segment begin time: %w", err)
	}
	segEnd, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return fmt.Errorf("invalid segment end time: %w", err)
	}
	segData, err := ioutil.ReadFile(segPath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}
	seg := &stream.HLSSegment{
		Name:     fname,
		Data:     segData,
		SeqNo:    uint64(seq),
		Duration: segEnd - segBegin,
	}
	_, err = processSegmentFunc(cxn, seg)
	return err
}

func runSegmenter(node *core.LivepeerNode, cxn *rtmpConnection, fname string) error {

	rd, wr, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create pipe err=%w", err)
	}
	closeFds := func() {
		wr.Close()
		rd.Close()
	}
	wg := sync.WaitGroup{}
	segList := MakeSegList()
	defer func() {
		closeFds()
		wg.Wait()
	}()

	// TODO Catch interrupt to gracefully terminate the pull (helpful for live)
	// Close pipes, drain transcoding queue, produce any outputs

	// ffmpeg io queue
	wg.Add(1)
	go func(reader io.Reader) {
		var str string
		var err error
		defer wg.Done()
		buf := bufio.NewReader(reader)
		segCount := 0
		for str, err = buf.ReadString('\n'); err == nil; str, err = buf.ReadString('\n') {
			segCount++
			segList.Insert(str)
		}
		glog.V(common.DEBUG).Info("io queue exit segs=", segCount)
		segList.MarkComplete()
	}(rd)

	// worker queue.
	for i := 0; i < nbWorkers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for {
				seg, err := segList.Remove()
				if err == errSegListComplete {
					return
				}
				if err := process(cxn, seg); err != nil {
					glog.Error("pull error err=", err)
					// TODO Re-insert into list for special cases, eg
					// no sessions available (sleep for refresh then retry)
					// There is no reason for VOD to have missing segs
					// unless the segments themselves are un-processable
					// TODO distinguish "non-retryable" error types
				}
			}
		}(i)
	}

	base := []string{node.WorkDir, "media", string(cxn.params.ManifestID)}
	// baseDir := filepath.Join(base...)
	// err = os.MkdirAll(baseDir, 0744)
	// if err != nil {
	// 	return fmt.Errorf("creating pull directory err=%w", err)
	// }
	// defer os.RemoveAll(baseDir) // TODO integrate with proper filesystem OS

	oname := filepath.Join(append(base, "%d.ts")...)
	err = ffmpeg.Transcode2(&ffmpeg.TranscodeOptionsIn{Fname: fname},
		[]ffmpeg.TranscodeOptions{{
			Oname:        oname,
			VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			Muxer: ffmpeg.ComponentOptions{
				Name: "stream_segment",
				Opts: map[string]string{
					"segment_list":      fmt.Sprintf("pipe:%d", wr.Fd()),
					"segment_list_type": "csv",
					"segment_time":      "2",
					// TODO OS-agnostic fix for trailing slash
					"segment_list_entry_prefix": filepath.Join(base...) + "/",
				}}}})
	if err != nil {
		return fmt.Errorf("pull segmenter err=%w", err)
	}

	wr.Close() // needed to cleanly terminate the read end of the pipe
	wg.Wait()  // wait for all threads to terminate before closing read fd
	rd.Close() // now clean up the read fd

	return nil
}

type concurrentSegList struct {
	mu *sync.Mutex
	cv *sync.Cond
	sl *segList

	complete bool
}

type segList struct {
	head *segs
	tail *segs
}

var errSegListComplete = errors.New("segments complete")
var errSegListEmpty = errors.New("empty segment list")

func MakeSegList() *concurrentSegList {
	mu := &sync.Mutex{}
	return &concurrentSegList{mu: mu, cv: sync.NewCond(mu), sl: &segList{}}
}

func (sl *segList) Empty() bool {
	return sl.head == nil
}
func (sl *segList) Insert(seg string) {
	if sl.head == nil {
		sl.tail = &segs{val: seg}
		sl.head = sl.tail
	} else {
		sl.tail.next = &segs{val: seg}
		sl.tail = sl.tail.next
	}
}
func (sl *segList) Remove() (string, error) {
	if sl == nil || sl.head == nil {
		return "", errSegListEmpty
	}
	v := sl.head.val
	sl.head = sl.head.next
	return v, nil
}

func (csl *concurrentSegList) MarkComplete() {
	csl.mu.Lock()
	defer csl.mu.Unlock()
	csl.complete = true
	csl.cv.Broadcast()
}
func (csl *concurrentSegList) Insert(seg string) {
	csl.mu.Lock()
	defer csl.mu.Unlock()
	if csl.complete {
		// TODO log??
		return
	}
	csl.sl.Insert(seg)
	csl.cv.Signal()
}
func (csl *concurrentSegList) Remove() (string, error) {
	csl.mu.Lock()
	defer csl.mu.Unlock()
	for !csl.complete && csl.sl.Empty() {
		csl.cv.Wait()
	}
	if csl.complete && csl.sl.Empty() {
		return "", errSegListComplete
	}
	return csl.sl.Remove()
}
