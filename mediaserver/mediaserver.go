//mediaserver is the place we set up the handlers for network requests.

package mediaserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"math/big"

	"github.com/ericxtang/m3u8"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
	"github.com/livepeer/golp/core"
	"github.com/livepeer/golp/eth"
	"github.com/livepeer/golp/net"
	lpmscore "github.com/livepeer/lpms/core"
	"github.com/livepeer/lpms/segmenter"
	"github.com/livepeer/lpms/stream"
)

var ErrNotFound = errors.New("NotFound")
var ErrAlreadyExists = errors.New("StreamAlreadyExists")
var ErrRTMPPublish = errors.New("ErrRTMPPublish")
var ErrBroadcast = errors.New("ErrBroadcast")
var HLSWaitTime = time.Second * 10
var HLSBufferCap = uint(43200) //12 hrs assuming 1s segment
var HLSBufferWindow = uint(5)
var SegOptions = segmenter.SegmenterOptions{SegLength: 8 * time.Second}
var HLSUnsubWorkerFreq = time.Second * 5

var EthRpcTimeout = 5 * time.Second
var EthEventTimeout = 5 * time.Second
var EthMinedTxTimeout = 60 * time.Second

var BroadcastPrice = big.NewInt(150)
var BroadcastJobVideoProfile = net.P_240P_30FPS_4_3
var TranscoderFeeCut = uint8(10)
var TranscoderRewardCut = uint8(10)
var TranscoderSegmentPrice = big.NewInt(150)

var CurrentHLSStreamID core.StreamID

type LivepeerMediaServer struct {
	RTMPSegmenter lpmscore.RTMPSegmenter
	LPMS          *lpmscore.LPMS
	HttpPort      string
	RtmpPort      string
	FfmpegPath    string
	LivepeerNode  *core.LivepeerNode

	hlsSubTimer           map[core.StreamID]time.Time
	hlsWorkerRunning      bool
	broadcastRtmpToHLSMap map[string]string
}

func NewLivepeerMediaServer(rtmpPort string, httpPort string, ffmpegPath string, lpNode *core.LivepeerNode) *LivepeerMediaServer {
	server := lpmscore.New(rtmpPort, httpPort, ffmpegPath, "")
	return &LivepeerMediaServer{RTMPSegmenter: server, LPMS: server, HttpPort: httpPort, RtmpPort: rtmpPort, FfmpegPath: ffmpegPath, LivepeerNode: lpNode}
}

//StartLPMS starts the LPMS server
func (s *LivepeerMediaServer) StartMediaServer(ctx context.Context) error {
	if s.LivepeerNode.Eth != nil {
		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfile.Name)
	}

	s.hlsSubTimer = make(map[core.StreamID]time.Time)
	go s.startHlsUnsubscribeWorker(time.Second*10, HLSUnsubWorkerFreq)

	s.broadcastRtmpToHLSMap = make(map[string]string)

	s.LPMS.HandleRTMPPublish(s.makeCreateRTMPStreamIDHandler(), s.makeGotRTMPStreamHandler(), s.makeEndRTMPStreamHandler())

	s.LPMS.HandleHLSPlay(s.makeGetHLSMasterPlaylistHandler(), s.makeGetHLSMediaPlaylistHandler(), s.makeGetHLSSegmentHandler())

	s.LPMS.HandleRTMPPlay(s.makeGetRTMPStreamHandler())

	http.HandleFunc("/transcode", func(w http.ResponseWriter, r *http.Request) {
		//Temporary endpoint just so we can invoke a transcode job.  This should be invoked by transcoders monitoring the smart contract.
		strmID := r.URL.Query().Get("strmID")
		if strmID == "" {
			http.Error(w, "Need to specify strmID", 500)
		}

		// 2 profiles is too much for my tiny laptop...
		// ids, err := s.LivepeerNode.Transcode(net.TranscodeConfig{StrmID: strmID, Profiles: []net.VideoProfile{net.P_144P_30FPS_16_9, net.P_240P_30FPS_16_9}})
		ids, err := s.LivepeerNode.Transcode(net.TranscodeConfig{StrmID: strmID, Profiles: []net.VideoProfile{net.P_240P_30FPS_16_9}}, nil)
		if err != nil {
			glog.Errorf("Error transcoding: %v", err)
			http.Error(w, "Error transcoding.", 500)
		}
		glog.Infof("New Stream IDs: %v", ids)
	})

	http.HandleFunc("/setBroadcastConfig", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("price")
		if p != "" {
			pi, err := strconv.Atoi(p)
			if err != nil {
				glog.Errorf("Price conversion failed: %v", err)
				return
			}
			BroadcastPrice = big.NewInt(int64(pi))
		}

		j := r.URL.Query().Get("job")
		if j != "" {
			jp := net.VideoProfileLookup[j]
			if jp.Name != "" {
				BroadcastJobVideoProfile = jp
			}
		}

		glog.Infof("Transcode Job Price: %v, Transcode Job Type: %v", BroadcastPrice, BroadcastJobVideoProfile.Name)
	})

	http.HandleFunc("/activateTranscoder", func(w http.ResponseWriter, r *http.Request) {
		active, err := s.LivepeerNode.Eth.IsActiveTranscoder()
		if err != nil {
			glog.Errorf("Error getting transcoder state: %v", err)
		}
		if active {
			glog.Error("Transcoder is already active")
			return
		}

		//Wait until a fresh round, register transcoder
		err = s.LivepeerNode.Eth.WaitUntilNextRound(eth.ProtocolBlockPerRound)
		if err != nil {
			glog.Errorf("Failed to wait until next round: %v", err)
			return
		}
		if err := eth.CheckRoundAndInit(s.LivepeerNode.Eth, EthRpcTimeout, EthMinedTxTimeout); err != nil {
			glog.Errorf("%v", err)
			return
		}

		tx, err := s.LivepeerNode.Eth.Transcoder(10, 5, big.NewInt(100))
		if err != nil {
			glog.Errorf("Error creating transcoder: %v", err)
			return
		}

		receipt, err := eth.WaitForMinedTx(s.LivepeerNode.Eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
		if err != nil {
			glog.Errorf("%v", err)
			return
		}
		if tx.Gas().Cmp(receipt.GasUsed) == 0 {
			glog.Errorf("Client 0 failed transcoder registration")
		}

		printStake(s.LivepeerNode.Eth)
	})

	http.HandleFunc("/setTranscoderConfig", func(w http.ResponseWriter, r *http.Request) {
		fc := r.URL.Query().Get("feecut")
		if fc != "" {
			fci, err := strconv.Atoi(fc)
			if err != nil {
				glog.Errorf("Fee cut conversion failed: %v", err)
				return
			}
			TranscoderFeeCut = uint8(fci)
		}

		rc := r.URL.Query().Get("rewardcut")
		if rc != "" {
			rci, err := strconv.Atoi(rc)
			if err != nil {
				glog.Errorf("Reward cut conversion failed: %v", err)
				return
			}
			TranscoderRewardCut = uint8(rci)
		}

		p := r.URL.Query().Get("price")
		if p != "" {
			pi, err := strconv.Atoi(p)
			if err != nil {
				glog.Errorf("Price conversion failed: %v", err)
				return
			}
			TranscoderSegmentPrice = big.NewInt(int64(pi))
		}

		glog.Infof("Transcoder Fee Cut: %v, Transcoder Reward Cut: %v, Transcoder Segment Price: %v", TranscoderFeeCut, TranscoderRewardCut, TranscoderSegmentPrice)
	})

	http.HandleFunc("/bond", func(w http.ResponseWriter, r *http.Request) {
		addrStr := r.URL.Query().Get("addr")
		if addrStr == "" {
			glog.Errorf("Need to provide addr")
			return
		}

		amountStr := r.URL.Query().Get("amount")
		if amountStr == "" {
			glog.Errorf("Need to provide amount")
			return
		}

		addr := common.HexToAddress(addrStr)
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			glog.Errorf("Cannot convert amount: %v", err)
			return
		}

		eth.CheckRoundAndInit(s.LivepeerNode.Eth, EthRpcTimeout, EthMinedTxTimeout)
		tx, err := s.LivepeerNode.Eth.Bond(big.NewInt(int64(amount)), addr)
		if err != nil {
			glog.Errorf("Failed to bond: %v", err)
			return
		}
		receipt, err := eth.WaitForMinedTx(s.LivepeerNode.Eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
		if err != nil {
			glog.Errorf("%v", err)
			return
		}
		if tx.Gas().Cmp(receipt.GasUsed) == 0 {
			glog.Errorf("Failed bonding: Ethereum Exception")
		}
	})

	http.HandleFunc("/printStake", func(w http.ResponseWriter, r *http.Request) {
		printStake(s.LivepeerNode.Eth)
	})

	http.HandleFunc("/streamID", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(CurrentHLSStreamID))
	})

	http.HandleFunc("/peersCount", func(w http.ResponseWriter, r *http.Request) {
	})

	http.HandleFunc("/streamerStatus", func(w http.ResponseWriter, r *http.Request) {
	})

	lpmsCtx, cancel := context.WithCancel(context.Background())
	ec := make(chan error, 1)
	go func() {
		ec <- s.LPMS.Start(lpmsCtx)
	}()

	select {
	case err := <-ec:
		glog.Infof("LPMS Server Error: %v.  Quitting...", err)
		cancel()
		return err
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

//RTMP Publish Handlers
func (s *LivepeerMediaServer) makeCreateRTMPStreamIDHandler() func(url *url.URL) (strmID string) {
	return func(url *url.URL) (strmID string) {
		id, err := core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "")
		if err != nil {
			glog.Errorf("Error making stream ID")
			return ""
		}
		return id.String()
	}
}

func (s *LivepeerMediaServer) makeGotRTMPStreamHandler() func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
	return func(url *url.URL, rtmpStrm *stream.VideoStream) (err error) {
		if s.LivepeerNode.Eth != nil {
			//Check Token Balance
			b, err := s.LivepeerNode.Eth.TokenBalance()
			if err != nil {
				glog.Errorf("Error getting token balance:%v", err)
				return ErrBroadcast
			}
			glog.Infof("Current token balance for is: %v", b)

			if b.Cmp(BroadcastPrice) < 0 {
				glog.Errorf("Low balance (%v) - cannot start broadcast session", b)
				return ErrBroadcast
			}
		}

		//Check if stream ID already exists
		if s.LivepeerNode.StreamDB.GetStream(core.StreamID(rtmpStrm.GetStreamID())) != nil {
			return ErrAlreadyExists
		}

		//Add stream to StreamDB
		if err := s.LivepeerNode.StreamDB.AddStream(core.StreamID(rtmpStrm.GetStreamID()), rtmpStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
			return ErrRTMPPublish
		}

		//Create a new HLS Stream.  If streamID is passed in, use that one.  Otherwise, generate a random ID.
		hlsStrmID := core.StreamID(url.Query().Get("hlsStrmID"))
		if hlsStrmID == "" {
			hlsStrmID, err = core.MakeStreamID(s.LivepeerNode.Identity, core.RandomVideoID(), "")
			if err != nil {
				glog.Errorf("Error making stream ID")
				return ErrRTMPPublish
			}
		}
		if hlsStrmID.GetNodeID() != s.LivepeerNode.Identity {
			glog.Errorf("Cannot create a HLS stream with Node ID: %v", hlsStrmID.GetNodeID())
			return ErrRTMPPublish
		}
		hlsStrm, err := s.LivepeerNode.StreamDB.AddNewStream(hlsStrmID, stream.HLS)
		if err != nil {
			glog.Errorf("Error creating HLS stream for segmentation: %v", err)
		}
		CurrentHLSStreamID = hlsStrmID

		//Create Segmenter
		glog.Infof("\n\nSegmenting rtmp stream:\n%v \nto hls stream:\n%v\n\n", rtmpStrm.GetStreamID(), hlsStrm.GetStreamID())
		go func() {
			err := s.RTMPSegmenter.SegmentRTMPToHLS(context.Background(), rtmpStrm, hlsStrm, SegOptions) //TODO: do we need to cancel this thread when the stream finishes?
			if err != nil {
				glog.Infof("Error in segmenter: %v, broadcasting finish message", err)
				err := hlsStrm.WriteHLSSegmentToStream(stream.HLSSegment{EOF: true})
				if err != nil {
					glog.Errorf("Error broadcasting finish: %v", err)
				}
			}
		}()

		//Kick off go routine to broadcast the hls stream.
		go func() {
			// glog.Infof("Kicking off broadcaster")
			err := s.LivepeerNode.BroadcastToNetwork(hlsStrm)
			if err == core.ErrEOF {
				glog.Info("Broadcast Ended.")
			} else if err != nil {
				glog.Errorf("Error broadcasting to network: %v", err)
			}
		}()

		//Store HLS Stream into StreamDB, remember HLS stream so we can remove later
		if err = s.LivepeerNode.StreamDB.AddStream(core.StreamID(hlsStrm.GetStreamID()), hlsStrm); err != nil {
			glog.Errorf("Error adding stream to streamDB: %v", err)
		}
		s.broadcastRtmpToHLSMap[rtmpStrm.GetStreamID()] = hlsStrm.GetStreamID()

		if s.LivepeerNode.Eth != nil {
			//Create Transcode Job Onchain, record the jobID
			tx, err := createBroadcastJob(s, hlsStrm)
			if err != nil {
				glog.Info("Error creating job.  Waiting for round start and trying again.")
				err = s.LivepeerNode.Eth.WaitUntilNextRound(eth.ProtocolBlockPerRound)
				if err != nil {
					glog.Errorf("Error waiting for round start: %v", err)
					return ErrBroadcast
				}

				tx, err = createBroadcastJob(s, hlsStrm)
				if err != nil {
					glog.Errorf("Error broadcasting: %v", err)
					return ErrBroadcast
				}
			}
			glog.Infof("Created broadcast job.  Price: %v. Type: %v. tx: %v", BroadcastPrice, BroadcastJobVideoProfile.Name, tx.Hash().Hex())

		}
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

		buf := s.LivepeerNode.StreamDB.GetHLSBuffer(strmID)
		if buf == nil {
			//Get subscriber and set up callback for when we get segments.
			sub, err := s.LivepeerNode.VideoNetwork.GetSubscriber(strmID.String())
			if err != nil {
				glog.Errorf("Error getting subscriber: %v", err)
				return nil, err
			}

			sub.Subscribe(context.Background(), func(seqNo uint64, data []byte, eof bool) {
				if eof {
					glog.Infof("Got EOF, writing to buf")
					buf.WriteEOF()
					if err := sub.Unsubscribe(); err != nil {
						glog.Errorf("Unsubscribe error: %v", err)
						return
					}
				}

				//Decode data into HLSSegment
				ss, err := core.BytesToSignedSegment(data)
				if err != nil {
					glog.Errorf("Error decoding byte array into segment: %v", err)
					return
				}

				//Add segment into a HLS buffer in StreamDB
				if buf == nil {
					buf = s.LivepeerNode.StreamDB.AddNewHLSBuffer(strmID)
					glog.Infof("Creating new buf in StreamDB: %v", s.LivepeerNode.StreamDB)
				}
				glog.Infof("Inserting seg %v into buf", ss.Seg.Name)
				buf.WriteSegment(ss.Seg.SeqNo, ss.Seg.Name, ss.Seg.Duration, ss.Seg.Data)
			})
		}

		//Wait for the HLSBuffer gets populated, get the playlist from the buffer, and return it.
		//Also update the hlsSubTimer.
		start := time.Now()
		for time.Since(start) < time.Second*10 {
			buf = s.LivepeerNode.StreamDB.GetHLSBuffer(strmID)
			if buf == nil {
				// glog.Infof("Waiting for playlist - sleeping: %v", s.LivepeerNode.StreamDB.GetHLSBuffer(strmID))
				time.Sleep(2 * time.Second)
				continue
			} else {
				pl, err := buf.LatestPlaylist()
				if err != nil {
					if err == stream.ErrEOF {
						return nil, err
					}

					// glog.Infof("Waiting for playlist... err: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				} else {
					// glog.Infof("Found playlist. Returning")
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
func (s *LivepeerMediaServer) makeGetRTMPStreamHandler() func(url *url.URL) (stream.Stream, error) {

	return func(url *url.URL) (stream.Stream, error) {
		// glog.Infof("Got req: ", url.Path)
		//Look for stream in StreamDB,
		strmID := parseStreamID(url.Path)
		strm := s.LivepeerNode.StreamDB.GetStream(strmID)
		if strm == nil {
			glog.Errorf("Cannot find RTMP stream")
			return nil, ErrNotFound
		}

		//Could use a subscriber, but not going to here because the RTMP stream doesn't need to be available for consumption by multiple views.  It's only for the segmenter.
		return strm, nil
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
				s.LivepeerNode.StreamDB.DeleteHLSBuffer(sid)
				s.LivepeerNode.UnsubscribeFromNetwork(sid)
				delete(s.hlsSubTimer, sid)
			}
		}
	}
}

//Helper Methods

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

func createBroadcastJob(s *LivepeerMediaServer, hlsStrm *stream.VideoStream) (*types.Transaction, error) {
	eth.CheckRoundAndInit(s.LivepeerNode.Eth, EthRpcTimeout, EthMinedTxTimeout)
	tOps := common.BytesToHash([]byte(BroadcastJobVideoProfile.Name))
	tx, err := s.LivepeerNode.Eth.Job(hlsStrm.GetStreamID(), tOps, BroadcastPrice)

	if err != nil {
		glog.Errorf("Error creating test broadcast job: %v", err)
		return nil, ErrBroadcast
	}
	receipt, err := eth.WaitForMinedTx(s.LivepeerNode.Eth.Backend(), EthRpcTimeout, EthMinedTxTimeout, tx.Hash())
	if err != nil {
		glog.Errorf("%v", err)
		return nil, ErrBroadcast
	}
	if tx.Gas().Cmp(receipt.GasUsed) == 0 {
		glog.Errorf("Job Creation Failed")
		return nil, ErrBroadcast
	}
	return tx, nil
}

func printStake(c eth.LivepeerEthClient) {
	if c == nil {
		glog.Errorf("Eth client is not assigned")
		return
	}
	s, err := c.TranscoderStake()
	if err != nil {
		glog.Errorf("Error getting transcoder stake: %v", err)
	}
	glog.Infof("Transcoder Active. Total Stake: %v", s)
}
