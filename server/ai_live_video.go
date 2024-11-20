package server

import (
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
	params.segmentReader.SwitchReader(func(reader io.Reader) {
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
		for {
			segment, err := subscriber.Read()
			if err != nil {
				// TODO if not EOS then signal a new orchestrator is needed
				slog.Info("Error reading trickle subscription", "url", url, "err", err)
				return
			}
			defer segment.Body.Close()
			// TODO send this into ffmpeg
			io.Copy(w, segment.Body)
		}
	}()

	// lpms
	go func() {
		ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
			Fname: fmt.Sprintf("pipe:%d", r.Fd()),
		}, []ffmpeg.TranscodeOptions{{
			Oname:        params.outputRTMPURL,
			AudioEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
			Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
		}})
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
	if err != nil {
		slog.Info("error starting control publisher", "stream", params.stream, "err", err)
		return
	}
	params.node.LiveMu.Lock()
	defer params.node.LiveMu.Unlock()
	params.node.LivePipelines[params.stream] = &core.LivePipeline{ControlPub: controlPub}
}

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
