package media

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/rtpreorderer"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

// TODO handle PATCH/PUT for ICE restarts (new Offers) and DELETE

const (
	keyframeInterval       = 2 * time.Second // TODO make configurable?
	iceDisconnectedTimeout = 5 * time.Second
	iceFailedTimeout       = 10 * time.Second
	iceKeepAliveInterval   = 2 * time.Second
)

// Generate a random ID for new resources
func generateID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// Generate a random ETag (version)
func generateETag() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(buf)) // Weak ETag format
}

// ICE server configuration.
// TODO make this configurable
var WebrtcConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	},
}

type WHIPServer struct {
	mediaEngine *webrtc.MediaEngine
	settings    func(*webrtc.API)
}

// handleCreate implements the POST that creates a new resource.
func (s *WHIPServer) CreateWHIP(ctx context.Context, ssr *SwitchableSegmentReader, whepURL string, w http.ResponseWriter, r *http.Request) *MediaState {
	clog.Infof(ctx, "creating whip")

	userAgent := r.Header.Get("User-Agent")
	clog.Info(ctx, "Client info", "user-agent", userAgent)
	if strings.Contains(userAgent, "Headless") {
		clog.AddVal(ctx, "e2e", "true")
	}

	// Must have Content-Type: application/sdp (the spec strongly recommends it)
	if r.Header.Get("Content-Type") != "application/sdp" {
		http.Error(w, "Unsupported Media Type, expected application/sdp", http.StatusUnsupportedMediaType)
		return NewMediaStateError(errors.New("unsupported media type"))
	}

	// Read the SDP offer
	offerBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading offer", http.StatusBadRequest)
		return NewMediaStateError(errors.New("error reading offer"))
	}
	defer r.Body.Close()

	// Create a new PeerConnection
	interceptors, statsGetter := genInterceptors(s.mediaEngine)
	api := webrtc.NewAPI(webrtc.WithMediaEngine(s.mediaEngine), interceptors, s.settings)
	peerConnection, err := api.NewPeerConnection(WebrtcConfig)
	if err != nil {
		clog.InfofErr(ctx, "Failed to create peerconnection", err)
		http.Error(w, "Failed to create PeerConnection", http.StatusInternalServerError)
		return NewMediaStateError(errors.New("failed to create peerconnection"))
	}
	mediaState := NewMediaState(peerConnection)

	// OnTrack callback: handle incoming media
	trackCh := make(chan *webrtc.TrackRemote)
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		clog.Info(ctx, "New track", "codec", track.Codec().MimeType, "ssrc", track.SSRC(), "rate", track.Codec().ClockRate)
		trackCh <- track
	})

	// PeerConnection state management
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		clog.Info(ctx, "whip ice connection state changed", "state", connectionState)
		if connectionState == webrtc.ICEConnectionStateFailed {
			mediaState.CloseError(errors.New("ICE connection state failed"))
		} else if connectionState == webrtc.ICEConnectionStateClosed {
			// Business logic when PeerConnection done
		}
	})

	// Setup the remote description (the incoming offer)
	sdpOffer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(offerBytes),
	}
	if err := peerConnection.SetRemoteDescription(sdpOffer); err != nil {
		// usually because the offer is incomplete or malformed
		e := fmt.Sprintf("SetRemoteDescription failed: %v", err)
		http.Error(w, e, http.StatusBadRequest)
		mediaState.CloseError(errors.New(e))
		return mediaState
	}

	// Create the answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		e := fmt.Sprintf("CreateAnswer failed: %v", err)
		http.Error(w, e, http.StatusInternalServerError)
		mediaState.CloseError(errors.New(e))
		return mediaState
	}

	// Gather ICE candidates and set local description
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		e := fmt.Sprintf("SetLocalDescription failed: %v", err)
		http.Error(w, e, http.StatusInternalServerError)
		mediaState.CloseError(errors.New(e))
		return mediaState
	}
	// Wait for ICE gathering if you want the full candidate set in the SDP
	<-gatherComplete

	localSDP := peerConnection.LocalDescription()

	// Create a resource ID and ETag
	resourceID := generateID()
	etag := generateETag()

	// Respond with 201 Created
	resourceURL := fmt.Sprintf("%s/%s", getRequestURL(r), resourceID)
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", resourceURL)
	w.Header().Set("ETag", etag)
	w.Header()["Link"] = GenICELinkHeaders(WebrtcConfig.ICEServers)
	w.Header().Set("Livepeer-Playback-URL", whepURL)
	w.Header().Set("Access-Control-Expose-Headers", "Location, ETag, Link, Livepeer-Playback-URL")
	w.WriteHeader(http.StatusCreated)

	// Write the full SDP answer
	if localSDP != nil {
		_, _ = w.Write([]byte(localSDP.SDP))
	}

	clog.Info(ctx, "waiting for A/V")
	gatherStartTime := time.Now()

	// wait for audio or video
	go func() {
		audioTrack, videoTrack, err := gatherIncomingTracks(ctx, peerConnection, trackCh)
		if err != nil {
			clog.Info(ctx, "error gathering tracks", "took", time.Since(gatherStartTime), "err", err)
			mediaState.CloseError(fmt.Errorf("error gathering tracks: %w", err))
			return
		}
		if videoTrack == nil {
			clog.Info(ctx, "no video! disconnecting", "took", time.Since(gatherStartTime))
			mediaState.CloseError(errors.New("missing video"))
			return
		}
		if videoTrack.Codec().MimeType != webrtc.MimeTypeH264 {
			clog.Info(ctx, "Expected H.264 video", "mime", videoTrack.Codec().MimeType)
			mediaState.CloseError(errors.New("non-h264 video"))
			return
		}
		tracks := []SegmenterTrack{NewSegmenterTrack(videoTrack)}
		if audioTrack != nil {
			tracks = append(tracks, NewSegmenterTrack(audioTrack))
		}
		minSegDur := 1 * time.Second
		segDurEnv := os.Getenv("LIVE_AI_MIN_SEG_DUR")
		if segDurEnv != "" {
			if parsed, err := time.ParseDuration(segDurEnv); err == nil {
				minSegDur = parsed
			}
		}
		trackCodecs := make([]string, len(tracks))
		timeDecoder := rtptime.NewGlobalDecoder2()
		segmenter := NewRTPSegmenter(tracks, ssr, minSegDur)
		var wg sync.WaitGroup // to wait for all tracks to complete
		for i, track := range tracks {
			trackCodecs[i] = track.Codec().MimeType
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleRTP(ctx, segmenter, timeDecoder, track, userAgent)
			}()
		}
		gatherDuration := time.Since(gatherStartTime)
		clog.Infof(ctx, "Gathered %d tracks (%s) took=%v", len(trackCodecs), strings.Join(trackCodecs, ", "), gatherDuration)

		mediaState.SetTracks(*statsGetter, tracks)

		wg.Wait()
		segmenter.CloseSegment()
		mediaState.Close()
	}()
	return mediaState
}

func handleRTP(ctx context.Context, segmenter *RTPSegmenter, timeDecoder *rtptime.GlobalDecoder2, track SegmenterTrack, userAgent string) {
	var frame rtp.Depacketizer
	var tsCorrector *TimestampCorrector
	codec := track.Codec().MimeType
	webrtcTrack := track.(*segmenterTrack).RTPTrack.(*webrtc.TrackRemote)
	incomingTrack := &IncomingTrack{track: webrtcTrack}
	isAudio := false
	switch codec {
	case webrtc.MimeTypeH264:
		frame = &codecs.H264Packet{IsAVC: true}
		tsCorrector = NewTimestampCorrector(TimestampCorrectorConfig{
			UserAgent: userAgent,
			Disable:   disableTSCorrection(),
		})
	case webrtc.MimeTypeOpus:
		frame = &codecs.OpusPacket{}
		isAudio = true
	default:
		clog.Info(ctx, "Unsupported codec", "mime", codec)
		return
	}
	gotAudio, gotVideo := false, false

	ro := rtpreorderer.New()
	au := [][]byte{}
	currentTS := uint32(0)
	for {
		pkt, _, err := webrtcTrack.ReadRTP()
		if err != nil {
			clog.Info(ctx, "Track read complete or error", "track", codec, "err", err)
			return
		}
		pkts, lost := ro.Process(pkt)
		if lost > 0 {
			clog.Info(ctx, "RTP lost packets", "track", codec, "count", lost)
		}
		for _, p := range pkts {
			if len(p.Payload) == 0 {
				// ignore empty packets
				continue
			}
			d, err := frame.Unmarshal(p.Payload)
			if err != nil {
				clog.InfofErr(ctx, "Depacketizer error", err)
			}
			if len(d) == 0 {
				// probably fragmented
				continue
			}
			if isAudio && !segmenter.IsReady() {
				// drop early audio packets until we have video
				// this is a hack to force video track to lead
				// so the time decoder automatically uses rescales audio to
				// the 90khz video timebase which matches mpegts
				continue
			}
			pts, ok := timeDecoder.Decode(incomingTrack, p)
			if !ok {
				clog.Info(ctx, "RTP: error decoding packet time", "track", codec)
				continue
			}
			if isAudio {
				segmenter.WriteAudio(track, pts, [][]byte{d})
				if !gotAudio {
					clog.Info(ctx, "First packet type=audio", "pts", pts, "rtp_ts", p.Timestamp)
					gotAudio = true
				}
				continue
			}

			// h264 video from here on
			// https://datatracker.ietf.org/doc/html/rfc6184

			pts = tsCorrector.Process(ctx, pts)

			if currentTS != p.Timestamp && len(au) > 0 {
				// received a new frame, but previous frame was incomplete (lost marker bit)
				clog.Info(ctx, "RTP: Previous frame was incomplete, sending anyway", "seq", p.SequenceNumber, "pts", pts)
				if h264.IsRandomAccess(au) && segmenter.ShouldStartSegment(pts, track.Codec().ClockRate) {
					segmenter.StartSegment(pts)
				}
				if segmenter.IsReady() {
					segmenter.WriteVideo(track, pts, pts, au)
				}
				au = [][]byte{}
			}
			currentTS = p.Timestamp

			nalus, err := splitH264NALUs(d)
			if err != nil {
				clog.InfofErr(ctx, "RTP: error splitting NALUs", err)
				continue
			}
			if len(nalus) <= 0 {
				clog.Info(ctx, "empty nalus", "len", len(d), "payload", len(p.Payload), "seq", p.SequenceNumber)
				continue
			}

			for _, nalu := range nalus {
				if len(nalu) <= 0 {
					clog.Info(ctx, "empty nalu", "seq", p.SequenceNumber)
					continue
				}
				/// check for sps / pps and ensure bframes are not allowed
				if h264.NALUTypeSPS == h264.NALUType(nalu[0]&0x1F) {
					if allowBFrames(nalu) {
						clog.Info(ctx, "B-frames may be allowed; could lead to DTS/PTS mismatch", "seq", p.SequenceNumber, "pts", pts)
					}
				}
			}

			au = append(au, nalus...)

			if !p.Marker {
				// frame is not complete yet
				// mpegts needs complete frames, so continue
				continue
			}

			// Have a complete frame, so reset the AU
			frameAU := au
			au = [][]byte{}

			dts := pts

			idr := h264.IsRandomAccess(frameAU)
			if idr && segmenter.ShouldStartSegment(dts, track.Codec().ClockRate) {
				segmenter.StartSegment(dts)
			}

			if segmenter.IsReady() {
				segmenter.WriteVideo(track, pts, dts, frameAU)
				if !gotVideo {
					clog.Info(ctx, "First packet type=video", "pts", pts, "rtp_ts", p.Timestamp)
					gotVideo = true
				}
			}
		}
	}
}

func gatherIncomingTracks(ctx context.Context, pc *webrtc.PeerConnection, trackCh chan *webrtc.TrackRemote) (*webrtc.TrackRemote, *webrtc.TrackRemote, error) {
	// Waits for video and audio
	// or video + 500ms for audio - whichever comes first
	videoTimeoutStr := os.Getenv("LIVE_AI_VIDEO_GATHER_TIMEOUT")
	videoTimeoutSec := 5 // default 5 seconds
	if videoTimeoutStr != "" {
		if vt, err := strconv.Atoi(videoTimeoutStr); err == nil && vt > 0 {
			videoTimeoutSec = vt
		} else {
			clog.InfofErr(ctx, "invalid video timeout, using default", err)
		}
	}
	VideoTimeout := time.Duration(videoTimeoutSec) * time.Second
	AudioOnlyTimeout := VideoTimeout
	AudioTimeout := 500 * time.Millisecond
	videoTimer := time.NewTimer(VideoTimeout)
	audioTimer := time.NewTimer(AudioOnlyTimeout)
	sdp, err := pc.RemoteDescription().Unmarshal()
	if err != nil {
		clog.InfofErr(ctx, "error unmarshaling remote sdp", err)
		return nil, nil, fmt.Errorf("error unmarshaling sdp: %w", err)
	}
	for _, md := range sdp.MediaDescriptions {
		for _, attr := range md.Attributes {
			if attr.Key == "rtpmap" {
				log.Println("CODEC ", attr.Value) // e.g. H264/90000
			}
		}
	}
	expectVideo := getMediaDescriptionByType(*sdp, "video") != nil
	expectAudio := getMediaDescriptionByType(*sdp, "audio") != nil
	awaitingVideo := expectVideo
	awaitingAudio := expectAudio
	var audioTrack *webrtc.TrackRemote
	var videoTrack *webrtc.TrackRemote
	for {
		select {
		case <-videoTimer.C:
			return audioTrack, nil, nil
		case <-audioTimer.C:
			return nil, videoTrack, nil
		case track := <-trackCh:
			switch track.Kind() {
			case webrtc.RTPCodecTypeAudio:
				if !awaitingAudio {
					clog.Info(ctx, "Received unexpected audio", "expected", expectAudio, "duplicate", expectAudio && !awaitingVideo)
				} else {
					audioTrack = track
				}
				awaitingAudio = false
				if !awaitingVideo {
					// got audio, don't have to wait for video, so leave
					return audioTrack, videoTrack, nil
				}
				audioTimer.Stop()
			case webrtc.RTPCodecTypeVideo:
				if !awaitingVideo {
					clog.Info(ctx, "Received unexpected  video", "expected", expectVideo, "duplicate", expectVideo && !awaitingVideo)
				} else {
					videoTrack = track
				}
				awaitingVideo = false
				if !awaitingAudio {
					// got video, don't have to wait for audio, so leave
					return audioTrack, videoTrack, nil
				}
				videoTimer.Stop()
				audioTimer.Stop()
				audioTimer.Reset(AudioTimeout)
			default:
				clog.Info(ctx, "unknown track", "kind", track.Kind())
			}
		}
	}
}

func allowBFrames(nalu []byte) bool {
	sps := new(h264.SPS)
	sps.Unmarshal(nalu)
	if sps.VUI != nil && sps.VUI.BitstreamRestriction != nil {
		return sps.VUI.BitstreamRestriction.MaxNumReorderFrames > 0
	}
	// Rec. ITU-T H.264 (08/21) Page 439
	//
	// When the max_num_reorder_frames syntax element is not present, the value of
	// max_num_reorder_frames value shall be inferred as follows:
	//
	//  – If profile_idc is equal to 44, 86, 100, 110, 122, or 244 and constraint_set3_flag is
	//    equal to 1, the value of  max_num_reorder_frames shall be inferred to be equal to 0.
	//
	// – Otherwise (profile_idc is not equal to 44, 86, 100, 110, 122, or 244 or
	//   constraint_set3_flag is equal to 0), the value of max_num_reorder_frames shall be
	//   inferred to be equal to MaxDpbFrames.
	//
	//
	switch sps.ProfileIdc {
	// baseline, constrained baseline and scalable-baseline profiles
	// only contain I/P frames
	// https://gitlab.freedesktop.org/gstreamer/gstreamer/-/blob/main/subprojects/gst-plugins-bad/gst/codectimestamper/gsth264timestamper.c?ref_type=heads#L278-305
	case 66:
		fallthrough
	case 83:
		return false
	// per RFC
	case 44:
		fallthrough
	case 86:
		fallthrough
	case 100:
		fallthrough
	case 110:
		fallthrough
	case 122:
		fallthrough
	case 244:
		if sps.ConstraintSet3Flag {
			// constraint_set3_flag may mean the -intra only profile.
			return false
		}
	}
	// TODO if an actual calculation is ever needed, figure MaxReorderFrames = MaxDpbFrames eg
	// https://github.com/FFmpeg/FFmpeg/blob/c6364b711bad1fe2fbd90e5b2798f87080ddf5ea/libavcodec/h264_ps.c#L594-L595
	// Assume true for now; diagnose any occurrences later
	return true
}

func getMediaDescriptionByType(sdp sdp.SessionDescription, mediaType string) *sdp.MediaDescription {
	for _, md := range sdp.MediaDescriptions {
		if md.MediaName.Media == mediaType {
			return md
		}
	}
	return nil
}

// Reconstruct the request URL as a string
func getRequestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.Path)
}

// split h264 nalus
func splitH264NALUs(buf []byte) ([][]byte, error) {
	var parts [][]byte
	offset := 0

	for {
		// If less than 4 bytes remain, we can't read the length anymore
		if len(buf[offset:]) < 4 {
			// If there's leftover data but not enough for a length,
			// you might consider returning an error. Or if you
			// expect partial data, handle that differently.
			if len(buf[offset:]) != 0 {
				return nil, errors.New("truncated length prefix at end of buffer")
			}
			// Otherwise, we're at the exact end — break gracefully
			break
		}

		// Read big-endian length
		partLen := binary.BigEndian.Uint32(buf[offset : offset+4])
		offset += 4

		// Check if enough bytes remain for this part
		if uint32(len(buf[offset:])) < partLen {
			return nil, errors.New("truncated part data, buffer ends early")
		}

		// Slice out the part data
		partData := buf[offset : offset+int(partLen)]
		offset += int(partLen)

		// Append to our results
		parts = append(parts, partData)
	}

	return parts, nil
}

func GenICELinkHeaders(iceServers []webrtc.ICEServer) []string {
	// https://github.com/bluenviron/mediamtx/blob/4dfe274239a5a37198ce108250ae8db04f34cc3e/internal/protocols/whip/link_header.go#L24-L37
	ret := make([]string, len(iceServers))

	for i, server := range iceServers {
		link := "<" + server.URLs[0] + ">; rel=\"ice-server\""
		if server.Username != "" {
			link += "; username=\"" + quoteCredential(server.Username) + "\"" +
				"; credential=\"" + quoteCredential(server.Credential.(string)) + "\"; credential-type=\"password\""
		}
		ret[i] = link
	}

	return ret
}

func quoteCredential(v string) string {
	b, _ := json.Marshal(v)
	s := string(b)
	return s[1 : len(s)-1]
}

func disableTSCorrection() bool {
	s := os.Getenv("LIVE_AI_DISABLE_TS_CORRECTION")
	v, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return v
}

func getUDPListenerAddr(addrStr string) (*net.UDPAddr, error) {
	if addrStr == "" {
		// Default to all interfaces, port 7290
		return &net.UDPAddr{
			IP:   net.IP{0, 0, 0, 0},
			Port: 7290,
		}, nil
	}

	// Handle the ":PORT" shorthand notation
	if strings.HasPrefix(addrStr, ":") {
		port := 0
		_, err := fmt.Sscanf(addrStr, ":%d", &port)
		if err != nil {
			return nil, fmt.Errorf("invalid UDP binding port: %v", err)
		}
		return &net.UDPAddr{
			IP:   net.IP{0, 0, 0, 0},
			Port: port,
		}, nil
	}

	// Parse as a full IP:PORT address
	udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid UDP address : %v", err)
	}
	return udpAddr, nil
}

func genParams() (*webrtc.MediaEngine, func(*webrtc.API)) {
	// Taken from Pion default codecs, but limited to Opus and H.264

	m := &webrtc.MediaEngine{}

	// audio codecs
	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil},
			PayloadType:        111,
		},
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeAudio); err != nil {
			// this should really never happen
			log.Fatal("could not register default codecs", err)
		}
	}

	// video codecs
	videoRTCPFeedback := []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}
	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				webrtc.MimeTypeH264, 90000, 0,
				"level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
				videoRTCPFeedback,
			},
			PayloadType: 102,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeRTX, 90000, 0, "apt=102", nil},
			PayloadType:        103,
		},

		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				webrtc.MimeTypeH264, 90000, 0,
				"level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f",
				videoRTCPFeedback,
			},
			PayloadType: 104,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeRTX, 90000, 0, "apt=104", nil},
			PayloadType:        105,
		},

		// TODO verify other H.264 profile-level-id such as 42e01f won't produle bframes
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
			// this should really never happen
			log.Fatal("could not register default codecs", err)
		}
	}

	se := webrtc.SettingEngine{}

	// Get UDP listener address from environment or use default
	udpAddr, err := getUDPListenerAddr(os.Getenv("LIVE_AI_WHIP_ADDR"))
	if err != nil {
		log.Fatal("could not get UDP listener address: ", err)
	}

	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("could not set udp listener: ", err)
	}

	se.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))
	natIP := os.Getenv("LIVE_AI_NAT_IP")
	if natIP != "" {
		se.SetNAT1To1IPs([]string{natIP}, webrtc.ICECandidateTypeHost)
	}
	se.SetICETimeouts(
		iceDisconnectedTimeout,
		iceFailedTimeout,
		iceKeepAliveInterval,
	)

	return m, webrtc.WithSettingEngine(se)
}

func genInterceptors(m *webrtc.MediaEngine) (func(*webrtc.API), *stats.Getter) {
	statsGetter := new(stats.Getter)
	ic := &interceptor.Registry{}
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor(intervalpli.GeneratorInterval(keyframeInterval))
	if err != nil {
		log.Fatal("could not register pli intercetpr")
	}
	ic.Add(intervalPliFactory)
	statsInterceptorFactory, err := stats.NewInterceptor()
	if err != nil {
		log.Fatal("could not register stats interceptor", err)
	}
	ic.Add(statsInterceptorFactory)
	statsInterceptorFactory.OnNewPeerConnection(func(id string, g stats.Getter) {
		*statsGetter = g
	})
	err = webrtc.RegisterDefaultInterceptors(m, ic)
	if err != nil {
		// this should really never happen
		log.Fatal("could not register default codecs", err)
	}
	return webrtc.WithInterceptorRegistry(ic), statsGetter
}

func NewWHIPServer() *WHIPServer {
	mediaEngine, settings := genParams()
	return &WHIPServer{
		mediaEngine: mediaEngine,
		settings:    settings,
	}
}

type IncomingTrack struct {
	track *webrtc.TrackRemote
}

// ClockRate returns the clock rate. Needed by rtptime.GlobalDecoder
func (t *IncomingTrack) ClockRate() int {
	return int(t.track.Codec().ClockRate)
}

// PTSEqualsDTS returns whether PTS equals DTS. Needed by rtptime.GlobalDecoder
// TODO handle bframes; look at mediamtx
func (*IncomingTrack) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
