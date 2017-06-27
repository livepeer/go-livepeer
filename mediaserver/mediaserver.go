//mediaserver is the place we set up the handlers for network requests.

package mediaserver

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ericxtang/m3u8"
	"github.com/golang/glog"
	"github.com/livepeer/libp2p-livepeer/core"
	"github.com/livepeer/libp2p-livepeer/net"
	"github.com/livepeer/lpms"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
	"github.com/nareix/joy4/av"
)

var ErrNotFound = errors.New("NotFound")
var ErrAlreadyExists = errors.New("StreamAlreadyExists")
var ErrRTMPPublish = errors.New("ErrRTMPPublish")
var HLSWaitTime = time.Second * 10
var HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
var HLSBufferWindow = uint(5)
var SegOptions = segmenter.SegmenterOptions{SegLength: 8 * time.Second}
var HLSUnsubWorkerFreq = time.Second * 5

type LivepeerMediaServer struct {
	LPMS         *lpms.LPMS
	HttpPort     string
	RtmpPort     string
	FfmpegPath   string
	LivepeerNode *core.LivepeerNode

	hlsSubTimer           map[core.StreamID]time.Time
	hlsWorkerRunning      bool
	broadcastRtmpToHLSMap map[string]string
}

func NewLivepeerMediaServer(rtmpPort string, httpPort string, ffmpegPath string, lpNode *core.LivepeerNode) *LivepeerMediaServer {
	server := lpms.New(rtmpPort, httpPort, ffmpegPath, "")
	return &LivepeerMediaServer{LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, FfmpegPath: ffmpegPath, LivepeerNode: lpNode}
}

//StartLPMS starts the LPMS server
func (s *LivepeerMediaServer) StartLPMS(ctx context.Context) error {
	s.hlsSubTimer = make(map[core.StreamID]time.Time)
	go s.startHlsUnsubscribeWorker(time.Second*10, HLSUnsubWorkerFreq)

	s.broadcastRtmpToHLSMap = make(map[string]string)

	s.LPMS.HandleRTMPPublish(s.makeCreateRTMPStreamIDHandler(), s.makeGotRTMPStreamHandler(), s.makeEndRTMPStreamHandler())

	s.LPMS.HandleHLSPlay(s.makeGetHLSMasterPlaylistHandler(), s.makeGetHLSMediaPlaylistHandler(), s.makeGetHLSSegmentHandler())

	s.LPMS.HandleRTMPPlay(s.makeGetRTMPStreamHandler())

	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		//Tell the LivepeerNode to create a job
	})

	http.HandleFunc("/createStream", func(w http.ResponseWriter, r *http.Request) {
	})

	http.HandleFunc("/localStreams", func(w http.ResponseWriter, r *http.Request) {
	})

	http.HandleFunc("/peersCount", func(w http.ResponseWriter, r *http.Request) {
	})

	http.HandleFunc("/streamerStatus", func(w http.ResponseWriter, r *http.Request) {
	})

	go s.LPMS.Start(ctx)

	return nil
}

//RTMP Publish Handlers
func (s *LivepeerMediaServer) makeCreateRTMPStreamIDHandler() func(url *url.URL) (strmID string) {
	return func(url *url.URL) (strmID string) {
		id := core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "")
		return id.String()
	}
}

func (s *LivepeerMediaServer) makeGotRTMPStreamHandler() func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
	return func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
		if s.LivepeerNode.StreamDB.GetStream(core.StreamID(rtmpStrm.GetStreamID())) != nil {
			return ErrAlreadyExists
		}

		var b net.Broadcaster

		//Add stream to StreamDB
		if err := s.LivepeerNode.StreamDB.AddStream(core.StreamID(rtmpStrm.GetStreamID()), rtmpStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
			return ErrRTMPPublish
		}

		//Create a new HLS Stream
		hlsStrm, err := s.LivepeerNode.StreamDB.AddNewStream(core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), ""), stream.HLS)
		if err != nil {
			glog.Errorf("Error creating HLS stream for segmentation: %v", err)
		}

		//Create Segmenter
		glog.Infof("Segmenting rtmp stream:%v to hls stream:%v", rtmpStrm.GetStreamID(), hlsStrm.GetStreamID())
		go func() {
			err := s.LPMS.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions) //TODO: do we need to cancel this thread when the stream finishes?
			if err != nil {
				glog.Infof("Error in segmenter, broadcasting finish message")
				err := b.Finish()
				if err != nil {
					glog.Errorf("Error broadcasting finish: %v", err)
				}
			}
		}()

		// if err := s.LivepeerNode.BroadcastToNetwork(context.Background(), hlsStrm); err != nil {
		// 	glog.Errorf("Error broadcasting to network: %v", err)
		// }
		//Kick off go routine to broadcast the hls stream.
		go func() {
			b = s.LivepeerNode.VideoNetwork.GetBroadcaster(hlsStrm.GetStreamID())
			// glog.Infof("Getting broadcaster, got %v", b)
			if b == nil {
				// 	glog.Infof("Creating broadcaster")
				b = s.LivepeerNode.VideoNetwork.NewBroadcaster(hlsStrm.GetStreamID())
				// 	glog.Infof("Got: %v", b)
			}
			counter := uint64(0)
			for {
				seg, err := hlsStrm.ReadHLSSegment()
				if err != nil {
					// glog.Errorf("Error reading broadcast HLS Segment: %v", err)
					time.Sleep(time.Second)
					continue
				}

				//Encode segment into []byte, broadcast it
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err = enc.Encode(seg)
				if err != nil {
					glog.Errorf("Error encoding segment to []byte: %v", err)
					continue
				}
				glog.Infof("Calling broadcast on broadcaster: %v", b)
				err = b.Broadcast(counter, buf.Bytes())
				if err != nil {
					glog.Errorf("Error broadcasting segment to network: %v", err)
				}
				counter++
			}
		}()

		//Store HLS Stream into StreamDB, remember HLS stream so we can remove later
		s.LivepeerNode.StreamDB.AddStream(core.StreamID(hlsStrm.GetStreamID()), hlsStrm)
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = hlsStrm.GetStreamID()

		return nil
	}
}

func (s *LivepeerMediaServer) makeEndRTMPStreamHandler() func(url *url.URL, rtmpStrm *stream.VideoStream) error {
	return func(url *url.URL, rtmpStrm *stream.VideoStream) error {
		//Remove RTMP stream
		s.LivepeerNode.StreamDB.DeleteStream(core.StreamID(rtmpStrm.GetStreamID()))
		//Remove HLS stream associated with the RTMP stream
		s.LivepeerNode.StreamDB.DeleteStream(core.StreamID(s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()]))
		return nil
	}
}

//End RTMP Publish Handlers

//HLS Play Handlers
func (s *LivepeerMediaServer) makeGetHLSMasterPlaylistHandler() func(url *url.URL) (*m3u8.MasterPlaylist, error) {
	return func(url *url.URL) (*m3u8.MasterPlaylist, error) {
		strmID := parseStreamID(url.Path)
		if !strmID.IsMasterPlaylistID() {
			return nil, nil
		}

		//Look for master playlist locally.  If not found, ask the network.
		// strm := s.LivepeerNode.StreamDB.GetStream(strmID)
		return nil, nil
	}
}

func (s *LivepeerMediaServer) makeGetHLSMediaPlaylistHandler() func(url *url.URL) (*m3u8.MediaPlaylist, error) {
	return func(url *url.URL) (*m3u8.MediaPlaylist, error) {
		strmID := parseStreamID(url.Path)
		if strmID.IsMasterPlaylistID() {
			return nil, nil
		}

		// _, err := s.LivepeerNode.SubscribeFromNetwork(context.Background(), strmID)
		// if err != nil {
		// 	glog.Errorf("Error subscribing from network: %v", err)
		// }
		//Look for media playlist locally.  If not found, ask the network, create a new local buffer.
		// strm := s.LivepeerNode.StreamDB.GetStream(strmID)
		buf := s.LivepeerNode.StreamDB.GetHLSBuffer(strmID)
		if buf == nil {
			//Create buf, subscribe from stream
			// buf = s.LivepeerNode.StreamDB.AddNewHLSBuffer(strmID)
			// s.LivepeerNode.StreamDB.SubscribeToHLSStream(strmID.String(), "local", buf)

			// s.LivepeerNode.SubscribeFromNetwork(context.Background(), func(seqNo uint64, data []byte, eof bool) {
			glog.Infof("buf is nil, creating subscription, asking the network")
			sub := s.LivepeerNode.VideoNetwork.GetSubscriber(strmID.String())
			if sub == nil {
				sub = s.LivepeerNode.VideoNetwork.NewSubscriber(strmID.String())
			}
			sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
				if eof {
					glog.Infof("Got EOF, writing to buf")
					buf.WriteEOF()
				}

				//Decode data into HLSSegment
				dec := gob.NewDecoder(bytes.NewReader(data))
				var seg stream.HLSSegment
				err := dec.Decode(&seg)
				if err != nil {
					glog.Errorf("Error decoding byte array into segment: %v", err)
				}

				//Add segment into a HLS buffer in StreamDB
				if buf == nil {
					buf = s.LivepeerNode.StreamDB.AddNewHLSBuffer(strmID)
					glog.Infof("Creating new buf in StreamDB: %v", s.LivepeerNode.StreamDB)
				}
				glog.Infof("Inserting seg %v into buf %v", seg.Name, buf)
				buf.WriteSegment(seg.SeqNo, seg.Name, seg.Duration, seg.Data)
			})
		}

		//Wait for the HLSBuffer gets populated, get the playlist from the buffer, and return it.
		//Also update the hlsSubTimer.
		start := time.Now()
		for time.Since(start) < time.Second*10 {
			buf = s.LivepeerNode.StreamDB.GetHLSBuffer(strmID)
			if buf == nil {
				glog.Infof("Got nothing - sleeping: %v", s.LivepeerNode.StreamDB.GetHLSBuffer(strmID))
				time.Sleep(500 * time.Millisecond)
				continue
			} else {
				pl, err := buf.LatestPlaylist()
				if err != nil {
					glog.Infof("Waiting for playlist... err: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				} else {
					glog.Infof("Found playlist. Returning")
					s.hlsSubTimer[strmID] = time.Now()
					return pl, err
				}
			}
		}

		return nil, ErrNotFound
	}
}

func (s *LivepeerMediaServer) makeGetHLSSegmentHandler() func(url *url.URL) ([]byte, error) {
	return func(url *url.URL) ([]byte, error) {
		strmID := parseStreamID(url.Path)
		if strmID.IsMasterPlaylistID() {
			return nil, nil
		}
		//Look for buffer in StreamDB, if not found return error (should already be here because of the mediaPlaylist request)
		buf := s.LivepeerNode.StreamDB.GetHLSBuffer(strmID)
		if buf == nil {
			return nil, ErrNotFound
		}

		segName := parseSegName(url.Path)
		if segName == "" {
			return nil, ErrNotFound
		}

		return buf.WaitAndPopSegment(context.Background(), segName)
	}
}

//End HLS Play Handlers

//PLay RTMP Play Handlers
func (s *LivepeerMediaServer) makeGetRTMPStreamHandler() func(ctx context.Context, reqPath string, dst av.MuxCloser) error {

	return func(ctx context.Context, reqPath string, dst av.MuxCloser) error {
		glog.Infof("Got req: ", reqPath)
		//Look for stream in StreamDB,
		strmID := parseStreamID(reqPath)
		strm := s.LivepeerNode.StreamDB.GetStream(strmID)
		if strm == nil {
			glog.Errorf("Cannot find RTMP stream")
			return ErrNotFound
		}

		//Could use a subscriber, but not going to here because the RTMP stream doesn't need to be available for consumption by multiple views.  It's only for the segmenter.
		return strm.ReadRTMPFromStream(ctx, dst)
	}
}

//End RTMP Handlers

func (s *LivepeerMediaServer) startHlsUnsubscribeWorker(limit time.Duration, freq time.Duration) {
	s.hlsWorkerRunning = true
	defer func() { s.hlsWorkerRunning = false }()
	for {
		time.Sleep(freq)
		for sid, t := range s.hlsSubTimer {
			if time.Since(t) > limit {
				glog.Infof("HLS Stream %v inactive - unsubscribing", sid)
				// streamDB.GetStream(sid).Unsubscribe()
				s.LivepeerNode.UnsubscribeFromNetwork(sid)
				delete(s.hlsSubTimer, sid)
			}
		}
	}
}

func parseStreamID(reqPath string) core.StreamID {
	var strmID string
	regex, _ := regexp.Compile("\\/stream\\/([[:alpha:]]|\\d)*")
	match := regex.FindString(reqPath)
	if match != "" {
		strmID = strings.Replace(match, "/stream/", "", -1)
	}
	return core.StreamID(strmID)
}

func parseSegName(reqPath string) string {
	var segName string
	regex, _ := regexp.Compile("\\/stream\\/.*\\.ts")
	match := regex.FindString(reqPath)
	if match != "" {
		segName = strings.Replace(match, "/stream/", "", -1)
	}
	return segName
}
