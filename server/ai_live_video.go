package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/lpms/ffmpeg"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/media"
	"github.com/livepeer/go-livepeer/trickle"
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

func startTrickleSubscribe(ctx context.Context, url *url.URL, params aiRequestParams) {
	// subscribe to the outputs and send them into LPMS
	subscriber := trickle.NewTrickleSubscriber(url.String())
	r, w, err := os.Pipe()
	if err != nil {
		clog.Errorf(ctx, "error getting pipe for trickle-ffmpeg url=%s err=%s", url, err)
	}

	// read segments from trickle subscription
	go func() {
		defer w.Close()
		i := 0
		for {
			segment, err := subscriber.Read()
			if err != nil {
				// TODO if not EOS then signal a new orchestrator is needed
				clog.Infof(ctx, "Error reading trickle subscription url=%s err=%s", url, err)
				return
			}
			clog.Infof(ctx, "Got output segment from trickle url=%s", url)
			defer segment.Body.Close()
			// TODO send this into ffmpeg

			if i > 0 {
				io.Copy(w, segment.Body)
			}
			i++
		}
	}()

	// lpms
	go func() {
		for {
			_, err := ffmpeg.Transcode3(&ffmpeg.TranscodeOptionsIn{
				Fname: fmt.Sprintf("pipe:%d", r.Fd()),
			}, []ffmpeg.TranscodeOptions{{
				Oname:        params.liveParams.outputRTMPURL,
				Profile:      ffmpeg.VideoProfile{Format: ffmpeg.FormatMPEGTS},
				VideoEncoder: ffmpeg.ComponentOptions{Name: "copy"},
				Muxer:        ffmpeg.ComponentOptions{Name: "flv"},
			}})
			//err := ff.Input(fmt.Sprintf("pipe:%d", r.Fd())).
			//	Output(params.liveParams.outputRTMPURL, ff.KwArgs{
			//		"c:a": "aac",
			//		"c:v": "libx264",
			//		"f":   "flv",
			//	}).OverWriteOutput().ErrorToStdOut().Run()

			//cmd := exec.Command("ffmpeg", "-y", "-i", fmt.Sprintf("pipe:%d", r.Fd()), "-c:v", "libx264", "-c:a", "aac", "-f", "flv", params.liveParams.outputRTMPURL)
			//
			//// Pass the read end of the pipe to FFmpeg
			//cmd.ExtraFiles = []*os.File{r}
			//
			//// Set stdout and stderr to see FFmpeg output
			//cmd.Stdout = os.Stdout
			//cmd.Stderr = os.Stderr
			//err := cmd.Run()

			if err != nil {
				clog.Errorf(ctx, "Error transcoding: %s", err)
			}
			time.Sleep(5 * time.Second)
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
