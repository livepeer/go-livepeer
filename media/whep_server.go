package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/livepeer/go-livepeer/clog"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

type WHEPServer struct {
	mediaEngine *webrtc.MediaEngine
	settings    func(*webrtc.API)
}

func (s *WHEPServer) CreateWHEP(ctx context.Context, w http.ResponseWriter, r *http.Request, mediaReader io.ReadCloser) {
	clog.Info(ctx, "creating whep", "user-agent", r.Header.Get("User-Agent"), "ip", r.RemoteAddr)

	// Must have Content-Type: application/sdp (the spec strongly recommends it)
	if r.Header.Get("Content-Type") != "application/sdp" {
		http.Error(w, "Unsupported Media Type, expected application/sdp", http.StatusUnsupportedMediaType)
		return
	}

	// Read the SDP offer
	offerBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading offer", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var offer webrtc.SessionDescription
	offer.Type = webrtc.SDPTypeOffer
	offer.SDP = string(offerBytes)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(s.mediaEngine), s.settings)
	peerConnection, err := api.NewPeerConnection(WebrtcConfig)
	if err != nil {
		clog.InfofErr(ctx, "Failed to create peerconnection", err)
		http.Error(w, "Failed to create PeerConnection", http.StatusInternalServerError)
		peerConnection.Close()
		return
	}
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		clog.Info(ctx, "whep ice connection state changed", "state", connectionState)
		if connectionState == webrtc.ICEConnectionStateFailed || connectionState == webrtc.ICEConnectionStateClosed {
			mediaReader.Close()
		}
	})
	mpegtsReader := mpegts.Reader{
		R: mediaReader,
	}
	if err := mpegtsReader.Initialize(); err != nil {
		clog.InfofErr(ctx, "Failed to initialize mpegts reader", err)
		http.Error(w, "Failed to initialize mpegts reader", http.StatusInternalServerError)
		peerConnection.Close()
		return
	}
	tracks := mpegtsReader.Tracks()
	hasAudio, hasVideo, hasIDR := false, false, false
	trackCodecs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		switch track.Codec.(type) {
		case *mpegts.CodecH264:
			// TODO this track can probably be reused for multiple viewers
			webrtcTrack, err := NewLocalTrack(
				webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
				"video", "livepeer",
			)
			if err != nil {
				clog.InfofErr(ctx, "Error creating track for h264", err)
				http.Error(w, "Error creating track for h264", http.StatusInternalServerError)
				peerConnection.Close()
				return
			}
			// TODO use RTPSender.ReplaceTrack on orch swaps
			if _, err := peerConnection.AddTrack(webrtcTrack); err != nil {
				clog.InfofErr(ctx, "Error adding track for video", err)
				http.Error(w, "Error adding track for h264", http.StatusInternalServerError)
				peerConnection.Close()
				return
			}
			mpegtsReader.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
				if !hasIDR {
					if h264.IsRandomAccess(au) {
						hasIDR = true
					} else {
						// drop until next keyframe
						return nil
					}
				}
				return webrtcTrack.WriteSample(au, pts)
			})
			trackCodecs = append(trackCodecs, "h264")
			hasVideo = true

		case *mpegts.CodecOpus:
			webrtcTrack, err := NewLocalTrack(
				// NB: Don't signal sample rate or channel count here. Leave empty to use defaults.
				// Opus RTP RFC 7587 requires opus/48000/2 regardless of the actual content
				webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
				"audio", "livepeer", // NB: can be another MediaID to desync a/v for latency
			)
			if err != nil {
				clog.InfofErr(ctx, "Error creating track for opus", err)
				http.Error(w, "Error creating track for opus", http.StatusInternalServerError)
				peerConnection.Close()
				return
			}
			// TODO use RTPSender.ReplaceTrack on orch swaps
			if _, err := peerConnection.AddTrack(webrtcTrack); err != nil {
				clog.InfofErr(ctx, "Error adding track for audio", err)
				http.Error(w, "Error adding track for audio", http.StatusInternalServerError)
				peerConnection.Close()
				return
			}
			mpegtsReader.OnDataOpus(track, func(pts int64, packets [][]byte) error {
				pts = multiplyAndDivide(pts, 48_000, 90_000)
				return webrtcTrack.WriteSample(packets, pts)
			})
			trackCodecs = append(trackCodecs, "opus")
			hasAudio = true
		}
	}

	if !hasAudio && !hasVideo {
		clog.InfofErr(ctx, "No audio or video in media stream", errors.New("no audio or video"))
		http.Error(w, "No audio or video in media stream", http.StatusInternalServerError)
		peerConnection.Close()
		return
	}
	clog.Info(ctx, "Outputs", "hasVideo", hasVideo, "hasAudio", hasAudio, "tracks", trackCodecs)

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		clog.InfofErr(ctx, "SetRemoteDescription failed", err)
		http.Error(w, "SetRemoteDescription failed", http.StatusBadRequest)
		peerConnection.Close()
		return
	}

	// Create and set local answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		clog.InfofErr(ctx, "CreateAnswer failed", err)
		http.Error(w, "CreateAnswer failed", http.StatusInternalServerError)
		peerConnection.Close()
		return
	}
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		clog.InfofErr(ctx, "SetLocalDescription failed", err)
		http.Error(w, "SetLocalDescription failed", http.StatusInternalServerError)
		peerConnection.Close()
		return
	}

	// Wait for ICE gathering to complete
	<-webrtc.GatheringCompletePromise(peerConnection)

	// Return the SDP answer
	w.Header().Set("Content-Type", "application/sdp")
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte(peerConnection.LocalDescription().SDP)); err != nil {
		clog.InfofErr(ctx, "Failed to write SDP answer", err)
		peerConnection.Close()
		return
	}

	// now start reading output
	go func() {
		for {
			err := mpegtsReader.Read()
			if err != nil {
				clog.InfofErr(ctx, "error reading mpegts", err)
				if err == io.EOF {
					peerConnection.Close()
				}
				return
			}
		}
	}()
}

func NewWHEPServer() *WHEPServer {
	me := &webrtc.MediaEngine{}
	me.RegisterDefaultCodecs()
	se := webrtc.SettingEngine{}
	udpAddr, err := getUDPListenerAddr(os.Getenv("LIVE_AI_WHEP_ADDR"))
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
	return &WHEPServer{
		mediaEngine: me,
		settings:    webrtc.WithSettingEngine(se),
	}
}

// Implements the TrackLocal interface in Pion
// https://github.com/pion/webrtc/blob/7a94394db0b429b0136265f4556512d5f4a05a0b/track_local.go#L102-L105
// This mostly borrows from TrackLocalStaticRTP *except* WriteSample takes a timestamp instead of a duration
type LocalTrack struct {
	packetizer rtp.Packetizer
	sequencer  rtp.Sequencer
	rtpTrack   *webrtc.TrackLocalStaticRTP

	isH264 bool
}

func NewLocalTrack(
	c webrtc.RTPCodecCapability,
	id, streamID string,
	options ...func(*webrtc.TrackLocalStaticRTP),
) (*LocalTrack, error) {
	rtpTrack, err := webrtc.NewTrackLocalStaticRTP(c, id, streamID, options...)
	if err != nil {
		return nil, err
	}

	return &LocalTrack{
		rtpTrack: rtpTrack,
	}, nil
}

// WriteSample writes a video / audio Sample to PeerConnections via RTP
// If one PeerConnection fails the packets will still be sent to
// all PeerConnections. The error message will contain the ID of the failed
// PeerConnections so you can remove them.
func (s *LocalTrack) WriteSample(au [][]byte, pts int64) error {
	packetizer := s.packetizer

	if packetizer == nil {
		return nil
	}

	var data []byte
	if s.isH264 {
		var annexBStartCode = []byte{0x0, 0x0, 0x0, 0x1}
		data = append(annexBStartCode, bytes.Join(au, annexBStartCode)...)
	} else {
		data = bytes.Join(au, []byte{})
	}
	duration := uint32(0) // samples for audio; video would be framerate
	packets := packetizer.Packetize(data, duration)

	writeErrs := []error{}
	for _, p := range packets {
		// Directly assignment of the timestamp here is
		// the only reason for having this LocalTrack class
		// If Pion ever adds this ability, use it instead
		p.Timestamp = uint32(pts)
		if err := s.rtpTrack.WriteRTP(p); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return errors.Join(writeErrs...)
}

func (s *LocalTrack) Bind(trackContext webrtc.TrackLocalContext) (webrtc.RTPCodecParameters, error) {
	codec, err := s.rtpTrack.Bind(trackContext)
	if err != nil {
		return codec, err
	}

	// We only need one packetizer
	if s.packetizer != nil {
		return codec, nil
	}

	var payloader rtp.Payloader
	switch strings.ToLower(codec.MimeType) {
	case strings.ToLower(webrtc.MimeTypeH264):
		payloader = &codecs.H264Payloader{}
		s.isH264 = true
	case strings.ToLower(webrtc.MimeTypeOpus):
		payloader = &codecs.OpusPayloader{}
	default:
		return codec, webrtc.ErrNoPayloaderForCodec
	}

	s.sequencer = rtp.NewRandomSequencer()

	const outboundMTU = 1200

	s.packetizer = rtp.NewPacketizerWithOptions(
		outboundMTU,
		payloader,
		s.sequencer,
		codec.ClockRate,
	)

	return codec, nil
}

// Unbind implements the teardown logic when the track is no longer needed. This happens
// because a track has been stopped.
func (s *LocalTrack) Unbind(t webrtc.TrackLocalContext) error {
	return s.rtpTrack.Unbind(t)
}

// ID is the unique identifier for this Track. This should be unique for the
// stream, but doesn't have to globally unique. A common example would be 'audio' or 'video'
// and StreamID would be 'desktop' or 'webcam'.
func (s *LocalTrack) ID() string { return s.rtpTrack.ID() }

// StreamID is the group this track belongs too. This must be unique.
func (s *LocalTrack) StreamID() string { return s.rtpTrack.StreamID() }

// RID is the RTP stream identifier.
func (s *LocalTrack) RID() string { return s.rtpTrack.RID() }

// Kind controls if this TrackLocal is audio or video.
func (s *LocalTrack) Kind() webrtc.RTPCodecType { return s.rtpTrack.Kind() }
