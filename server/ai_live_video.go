package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
	"github.com/livepeer/lpms/ffmpeg"
)

func startTricklePublish(url *url.URL, params aiRequestParams) {
	publisher, err := trickle.NewTricklePublisher(url.String())
	if err != nil {
		slog.Info("error publishing trickle", "err", err)
	}
	params.liveParams.segmentReader.SwitchReader(func(reader io.Reader) {
		// check for end of stream
		if _, eos := reader.(*media.EOSReader); eos {
			if err := publisher.Close(); err != nil {
				slog.Info("Error closing trickle publisher", "err", err)
			}
			return
		}
		go func() {
			// TODO this blocks! very bad!
			if err := publisher.Write(reader); err != nil {
				slog.Info("Error writing to trickle publisher", "err", err)
			}
		}()
	})
	slog.Info("trickle pub", "url", url)
}

func startTrickleSubscribe(url *url.URL, params aiRequestParams) {
	// subscribe to the outputs and send them into LPMS
	subscriber := trickle.NewTrickleSubscriber(url.String())
	r, w, err := os.Pipe()
	if err != nil {
		slog.Info("error getting pipe for trickle-ffmpeg", "url", url, "err", err)
	}

	// read segments from trickle subscription
	go func() {
		defer w.Close()
		i := 0
		for {
			segment, err := subscriber.Read()
			if err != nil {
				// TODO if not EOS then signal a new orchestrator is needed
				slog.Info("Error reading trickle subscription", "url", url, "err", err)
				return
			}
			defer segment.Body.Close()

			out, err := io.ReadAll(segment.Body)
			if err != nil {
				slog.Info("Error reading segment body", "url", url, "err", err)
				return
			}

			if err := os.WriteFile(fmt.Sprintf("segment-%d.ts", i), out, 0644); err != nil {
				slog.Info("Error writing segment to file", "url", url, "err", err)
				return
			}
			if i > 0 {
				// skip the first segment which is used to startup the stream
				_, err = io.Copy(w, bytes.NewReader(out))
				if err != nil {
					slog.Error("Error copying segment to output ffmpeg", "err", err)
				}
			}
			i++
		}
	}()

	// lpms
	go func() {
		slog.Info("Output RTMP URL", "url", params.liveParams.outputRTMPURL)
		_, err := ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
			Fname: fmt.Sprintf("pipe:%d", r.Fd()),
			Profile: ffmpeg.VideoProfile{
        	Name: "segments",
        	Format: ffmpeg.FormatMPEGTS,
			Resolution: "512x512",
			AspectRatio: "1:1",
    	},
		}, []ffmpeg.TranscodeOptions{{
			Oname:        params.liveParams.outputRTMPURL,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "aac"},
			VideoEncoder: ffmpeg.ComponentOptions{Name: "libx264"},
			Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
		}})
		if err != nil {
			slog.Info("Error transcoding trickle stream", "url", url, "err", err)
		} else {
			slog.Info("Transcoding complete", "url", url)
		}
	}()
}

func mediamtxSourceTypeToString(s string) (string, error) {
	switch s {
	case "webrtcSession":
		return "whip", nil
	case "rtmpConn":
		return "rtmp", nil
	default:
		return "", errors.New("unknown media source")
	}
}

func startControlPublish(control *url.URL, params aiRequestParams) {
	controlPub, err := trickle.NewTricklePublisher(control.String())
	stream := params.liveParams.stream
	if err != nil {
		slog.Info("error starting control publisher", "stream", stream, "err", err)
		return
	}
	params.node.LiveMu.Lock()
	defer params.node.LiveMu.Unlock()
	params.node.LivePipelines[stream] = &core.LivePipeline{ControlPub: controlPub}
}

const (
	mediaMTXControlPort = "9997"
	mediaMTXControlUser = "admin"
)

func (ls *LivepeerServer) kickInputConnection(mediaMTXHost, sourceID, sourceType string) error {
	var apiPath string
	switch sourceType {
	case "webrtcSession":
		apiPath = "webrtcsessions"
	case "rtmpConn":
		apiPath = "rtmpconns"
	default:
		return fmt.Errorf("invalid sourceType: %s", sourceType)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%s/v3/%s/kick/%s", mediaMTXHost, mediaMTXControlPort, apiPath, sourceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create kick request: %w", err)
	}
	req.SetBasicAuth(mediaMTXControlUser, ls.mediaMTXApiPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to kick connection: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick connection failed with status code: %d body: %s", resp.StatusCode, body)
	}
	return nil
}
