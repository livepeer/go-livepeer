package benchmark

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/joy4/av"
	"github.com/livepeer/joy4/av/avutil"
	"github.com/livepeer/joy4/av/pktque"
	"github.com/livepeer/joy4/format/ts"
	"github.com/livepeer/joy4/jerrors"
	// "github.com/livepeer/stream-tester/internal/model"
)

var segLen = 2 * time.Second

// segmenter take video file and cuts it into .ts segments

type hlsSegment struct {
	err      error
	seqNo    int
	pts      time.Duration
	duration time.Duration
	data     []byte
}

func startSegmenting(ctx context.Context, fileName string, stopAtFileEnd bool, stopAfter time.Duration, out chan<- *hlsSegment) error {
	glog.Infof("Starting segmenting file %s", fileName)
	inFile, err := avutil.Open(fileName)
	if err != nil {
		glog.Fatal(err)
	}
	go segmentingLoop(ctx, fileName, inFile, stopAtFileEnd, stopAfter, out)

	return err
}

func createInMemoryTSMuxer() (av.Muxer, *bytes.Buffer) {
	buf := new(bytes.Buffer)
	return ts.NewMuxer(buf), buf
}

func segmentingLoop(ctx context.Context, fileName string, inFileReal av.DemuxCloser, stopAtFileEnd bool, stopAfter time.Duration, out chan<- *hlsSegment) {
	var err error
	var streams []av.CodecData
	var videoidx, audioidx int8

	// ts := &timeShifter{}
	// filters := pktque.Filters{ts, &pktque.FixTime{MakeIncrement: true}, &pktque.Walltime{}}
	filters := pktque.Filters{&pktque.FixTime{MakeIncrement: true}}
	inFile := &pktque.FilterDemuxer{Demuxer: inFileReal, Filter: filters}
	if streams, err = inFile.Streams(); err != nil {
		msg := fmt.Sprintf("Can't get info about file: '%+v', isNoAudio %v isNoVideo %v", err, errors.Is(err, jerrors.ErrNoAudioInfoFound), errors.Is(err, jerrors.ErrNoVideoInfoFound))
		if !(errors.Is(err, jerrors.ErrNoAudioInfoFound) || errors.Is(err, jerrors.ErrNoVideoInfoFound)) {
			glog.Fatal(msg)
		}
		fmt.Println(msg)
		panic(msg)
	}
	for i, st := range streams {
		if st.Type().IsAudio() {
			audioidx = int8(i)
		}
		if st.Type().IsVideo() {
			videoidx = int8(i)
		}
	}
	glog.V(common.VERBOSE).Infof("Video stream index %d, audio stream index %d\n", videoidx, audioidx)

	seqNo := 0
	// var curPTS time.Duration
	var firstFramePacket *av.Packet
	var lastPacket av.Packet
	var prevPTS, curDur time.Duration
	for {
		// segName := fmt.Sprintf("%d.ts", seqNo)
		// segFile, err := avutil.Create(segName)
		// if err != nil {
		// 	glog.Fatal(err)
		// }
		segFile, buf := createInMemoryTSMuxer()
		err = segFile.WriteHeader(streams)
		if err != nil {
			glog.Fatal(err)
		}
		if firstFramePacket != nil {
			err = segFile.WritePacket(*firstFramePacket)
			if err != nil {
				glog.Fatal(err)
			}
			prevPTS = firstFramePacket.Time
			firstFramePacket = nil
		}
		// var curSegStart = curPTS
		var rerr error
		var pkt av.Packet
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			pkt, rerr = inFile.ReadPacket()
			if rerr != nil {
				if rerr == io.EOF {
					if lastPacket.Time != 0 {
						curDur = lastPacket.Time - prevPTS
					}
					break
				}
				glog.Fatal(rerr)
			}
			lastPacket = pkt

			// fmt.Printf("Packet Is Keyframe %v Is Audio %v Is Video %v PTS %s\n", pkt.IsKeyFrame, pkt.Idx == audioidx, pkt.Idx == videoidx, pkt.Time)
			// curPTS = pkt.Time
			// if curPTS-curSegStart > 1900*time.Millisecond && pkt.IsKeyFrame {
			// 	firstFramePacket = &pkt
			// 	break
			// }
			// This matches segmenter algorithm used in ffmpeg
			if pkt.IsKeyFrame && pkt.Time >= time.Duration(seqNo+1)*segLen {
				firstFramePacket = &pkt
				glog.V(common.VERBOSE).Infof("Packet Is Keyframe %v Is Audio %v Is Video %v PTS %s sinc prev %s seqNo %d\n", pkt.IsKeyFrame, pkt.Idx == audioidx, pkt.Idx == videoidx, pkt.Time,
					pkt.Time-prevPTS, seqNo+1)
				// prevPTS = pkt.Time
				curDur = pkt.Time - prevPTS
				break
			}
			err = segFile.WritePacket(pkt)
			if err != nil {
				glog.Fatal(err)
			}
		}
		err = segFile.WriteTrailer()
		if err != nil {
			glog.Fatal(err)
		}
		if rerr == io.EOF && stopAfter > 0 && (prevPTS+curDur) < stopAfter {
			// re-open same file and stream it again
			firstFramePacket = nil
			// ts.timeShift = lastPacket.Time + 30*time.Millisecond
			inf, err := avutil.Open(fileName)
			if err != nil {
				glog.Fatal(err)
			}
			inFile.Demuxer = inf
			// rs.counter.currentSegments = 0
			inFile.Streams()
			hlsSeg := &hlsSegment{
				// err:      rerr,
				seqNo:    seqNo,
				pts:      prevPTS,
				duration: curDur,
				data:     buf.Bytes(),
			}
			out <- hlsSeg
			prevPTS = lastPacket.Time
		} else {
			hlsSeg := &hlsSegment{
				// err:      rerr,
				seqNo:    seqNo,
				pts:      prevPTS,
				duration: curDur,
				data:     buf.Bytes(),
			}
			out <- hlsSeg

			if rerr == io.EOF {
				// hlsSeg := &hlsSegment{
				// 	err:   rerr,
				// 	seqNo: seqNo,
				// }
				// out <- hlsSeg
				hlsSeg := &hlsSegment{
					err:   io.EOF,
					seqNo: seqNo + 1,
					pts:   prevPTS + curDur,
				}
				out <- hlsSeg
				break
			}
		}
		if stopAfter > 0 && (prevPTS+curDur) > stopAfter {
			hlsSeg := &hlsSegment{
				err:   io.EOF,
				seqNo: seqNo + 1,
				pts:   prevPTS + curDur,
			}
			out <- hlsSeg
			break
		}
		seqNo++
	}
	return
}
