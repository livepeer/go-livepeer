package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func (s *LivepeerServer) Pull(fname string, streamName core.ManifestID) {
	glog.V(common.VERBOSE).Info("Pulling ", fname, " to ", BroadcastJobVideoProfiles)

	// TODO this isn't rtmp anymore
	strm := stream.NewBasicRTMPVideoStream(&core.StreamParameters{ManifestID: streamName, Profiles: BroadcastJobVideoProfiles})

	cxn, err := s.registerConnection(strm)
	if err != nil {
		// TODO error handling ?
		return
	}
	runSegmenter(s.LivepeerNode, cxn, fname)
	removeRTMPStream(s, cxn.mid)
	// TODO remove based on prefix?
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
		Data:     segData,
		SeqNo:    uint64(seq),
		Duration: segEnd - segBegin,
	}
	_, err = processSegment(cxn, seg)
	return err
}

func runSegmenter(node *core.LivepeerNode, cxn *rtmpConnection, fname string) {

	rd, wr, err := os.Pipe()
	if err != nil {
		glog.Error("error creating pipe err=", err)
		return
	}
	closeFds := func() {
		rd.Close()
		wr.Close()
	}
	defer closeFds()
	segCh := make(chan string, 10)
	readyCh := make(chan chan string)
	ioTermCh := make(chan struct{})
	feederTermCh := make(chan struct{})
	ctx, ctxCancel := context.WithCancel(context.Background())

	// ffmpeg io queue
	go func(reader io.Reader) {
		var str string
		var err error
		buf := bufio.NewReader(reader)
		for str, err = buf.ReadString('\n'); err == nil; str, err = buf.ReadString('\n') {
			segCh <- str
		}
		glog.V(common.DEBUG).Info("io queue exit")
		ioTermCh <- struct{}{}
	}(rd)

	// feeder queue TODO better name for this ??
	go func() {
		drainSignal := make(chan struct{}, 10)
		var head *segs
		var tail *segs
		readyWorkers := []chan string{}
		ioTerminated := false
		segCount := 0
		for {
			select {
			case seg := <-segCh:
				// invoked when a segment is read; adds segment to list
				segCount++
				if head == nil {
					// if head == nil, we've reached the end of the list
					tail = &segs{val: seg}
					head = tail
				} else {
					tail.next = &segs{val: seg}
					tail = tail.next
				}
				drainSignal <- struct{}{}
			case <-drainSignal:
				// invoked when a segment comes in, or a worker is ready
				if head == nil && ioTerminated {
					// this could get invoked multiple times but that's okay
					ctxCancel()
				}
				if len(readyWorkers) <= 0 || head == nil {
					continue
				}
				segCount--
				workerCh := readyWorkers[len(readyWorkers)-1]
				readyWorkers = readyWorkers[:len(readyWorkers)-1]
				workerCh <- head.val
				head = head.next
			case workerCh := <-readyCh:
				// invoked when a worker is ready to consume a segment
				readyWorkers = append(readyWorkers, workerCh)
				drainSignal <- struct{}{}
			case <-ioTermCh:
				// invoked after IO completes
				ioTerminated = true
				drainSignal <- struct{}{} // in case io returned nothing
			case <-feederTermCh:
				// invoked after all workers have terminated
				if head != nil || !ioTerminated {
					glog.Error("unexpected exit for feeder; may dangle")
				}
				return
			}
		}
	}()

	// worker queue.
	nbWorkers := 3
	workerTermCh := make(chan struct{}, nbWorkers)
	for i := 0; i < nbWorkers; i++ {
		go func(worker int) {
			// TODO terminating condition???
			workerCh := make(chan string)
			readyCh <- workerCh
			for {
				select {
				case seg := <-workerCh:
					if err := process(cxn, seg); err != nil {
						glog.Error("pull error err=", err)
						//workerTermCh <- struct{}{} TODO bail if error?
						return
					}
					readyCh <- workerCh
				case <-ctx.Done():
					glog.V(common.DEBUG).Info("terminating worker ", worker)
					workerTermCh <- struct{}{}
					return
				}
			}
		}(i)
	}

	cxn.params.OS.SaveData("dummy", nil) // hack to create the directory TODO fix
	base := []string{node.MediaDir, string(cxn.params.ManifestID)}
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
					// TODO OS-agnostic fix need for trailing slash
					"segment_list_entry_prefix": filepath.Join(base...) + "/",
				}}}})
	if err != nil {
		glog.Error("error segmenting err=", err)
	}
	closeFds()
	for i := 0; i < nbWorkers; i++ {
		<-workerTermCh
	}
}
