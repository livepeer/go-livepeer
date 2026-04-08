package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/trickle"
)

type ServerlessWorker struct {
	mu         sync.Mutex
	inUse      int
	capacity   int
	wsURL      string
	trickleSrv *trickle.Server
}

type wsMessage struct {
	Type string `json:"type"`
}

type wsHandshakeMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// ServerlessHandshakeError preserves structured websocket handshake failures.
type ServerlessHandshakeError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServerlessHandshakeError) Error() string {
	if e == nil {
		return "serverless handshake failed"
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("serverless handshake failed (%s): %s", e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("serverless handshake failed: %s", e.Message)
	}
	if e.Code != "" {
		return fmt.Sprintf("serverless handshake failed (%s)", e.Code)
	}
	return "serverless handshake failed"
}

type createChannelsMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	MimeType  string `json:"mime_type"`
	Direction string `json:"direction"`
}

type closeChannelsMessage struct {
	Type     string   `json:"type"`
	Channels []string `json:"channels"`
}

type wsResponseMessage struct {
	Type      string        `json:"type"`
	RequestID string        `json:"request_id"`
	Channels  []channelInfo `json:"channels"`
}

type channelInfo struct {
	URL       string `json:"url"`
	Direction string `json:"direction"`
	MimeType  string `json:"mime_type"`
}

// NewServerlessWorker creates a new ServerlessWorker instance
func NewServerlessWorker(wsURL string, capacity int) (*ServerlessWorker, error) {
	if wsURL == "" {
		return nil, fmt.Errorf("serverless worker requires a websocket URL")
	}
	if capacity <= 0 {
		return nil, fmt.Errorf("serverless worker requires a positive capacity")
	}

	return &ServerlessWorker{
		capacity: capacity,
		wsURL:    wsURL,
	}, nil
}

func (f *ServerlessWorker) SetTrickleServer(srv *trickle.Server) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trickleSrv = srv
}

func (f *ServerlessWorker) TextToImage(ctx context.Context, req GenTextToImageJSONRequestBody) (*ImageResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.TextToImage not implemented")
}

func (f *ServerlessWorker) ImageToImage(ctx context.Context, req GenImageToImageMultipartRequestBody) (*ImageResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.ImageToImage not implemented")
}

func (f *ServerlessWorker) ImageToVideo(ctx context.Context, req GenImageToVideoMultipartRequestBody) (*VideoResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.ImageToVideo not implemented")
}

func (f *ServerlessWorker) Upscale(ctx context.Context, req GenUpscaleMultipartRequestBody) (*ImageResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.Upscale not implemented")
}

func (f *ServerlessWorker) AudioToText(ctx context.Context, req GenAudioToTextMultipartRequestBody) (*TextResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.AudioToText not implemented")
}

func (f *ServerlessWorker) LLM(ctx context.Context, req GenLLMJSONRequestBody) (interface{}, error) {
	return nil, fmt.Errorf("ServerlessWorker.LLM not implemented")
}

func (f *ServerlessWorker) SegmentAnything2(ctx context.Context, req GenSegmentAnything2MultipartRequestBody) (*MasksResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.SegmentAnything2 not implemented")
}

func (f *ServerlessWorker) ImageToText(ctx context.Context, req GenImageToTextMultipartRequestBody) (*ImageToTextResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.ImageToText not implemented")
}

func (f *ServerlessWorker) TextToSpeech(ctx context.Context, req GenTextToSpeechJSONRequestBody) (*AudioResponse, error) {
	return nil, fmt.Errorf("ServerlessWorker.TextToSpeech not implemented")
}

func (f *ServerlessWorker) LiveVideoToVideo(ctx context.Context, req GenLiveVideoToVideoJSONRequestBody) (*LiveVideoToVideoResponse, error) {
	manifestID := ""
	if req.ManifestId != nil {
		manifestID = *req.ManifestId
	}
	if manifestID == "" {
		return nil, fmt.Errorf("serverless worker requires manifest ID for live-video-to-video")
	}

	if req.ControlUrl == nil || *req.ControlUrl == "" {
		return nil, fmt.Errorf("serverless worker requires control URL for live-video-to-video")
	}
	controlURL, err := url.Parse(*req.ControlUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid control URL: %w", err)
	}
	controlPathSuffix := manifestID + "-control"
	if !strings.HasSuffix(controlURL.Path, controlPathSuffix) {
		return nil, fmt.Errorf("control URL path %q must end with manifest ID suffix %q", controlURL.Path, controlPathSuffix)
	}
	trickleBasePath := strings.TrimSuffix(controlURL.Path, controlPathSuffix)
	buildTrickleURL := func(channelName string) string {
		if trickleBasePath == "" {
			return ""
		}
		u := *controlURL
		u.Path = trickleBasePath
		u = *u.JoinPath(channelName)
		return u.String()
	}
	if buildTrickleURL(manifestID+"-test") == "" {
		return nil, fmt.Errorf("could not construct trickle channel URL from control URL %q", *req.ControlUrl)
	}

	wsURL := f.wsURL
	wsURLPrefix := strings.ToLower(strings.TrimSpace(os.Getenv("LIVE_AI_WS_PREFIX")))
	if req.Params != nil && wsURLPrefix != "" {
		if wsURLRaw, ok := (*req.Params)["ws_url"]; ok {
			wsURLOverride, ok := wsURLRaw.(string)
			if !ok {
				return nil, fmt.Errorf("invalid params.ws_url: must be a string")
			}
			if wsURLOverride != "" {
				wsURLOverride = strings.ToLower(wsURLOverride)
				if !strings.HasPrefix(wsURLOverride, wsURLPrefix) {
					return nil, fmt.Errorf("unacceptable params.ws_url")
				}
				parsedWSURL, err := url.Parse(wsURLOverride)
				if err != nil || parsedWSURL.Host == "" {
					return nil, fmt.Errorf("invalid params.ws_url")
				}
				wsURL = wsURLOverride
			}
			delete(*req.Params, "ws_url")
		}
	}

	f.mu.Lock()
	trickleSrv := f.trickleSrv
	if trickleSrv == nil {
		f.mu.Unlock()
		return nil, fmt.Errorf("serverless worker requires trickle server before live-video-to-video")
	}
	f.inUse++
	currentInUse := f.inUse
	f.mu.Unlock()

	slog.Info("Starting new job", "inUse", currentInUse, "capacity", f.capacity, "url", wsURL)

	// Prepare headers with authorization
	headers := http.Header{}
	if authToken := os.Getenv("FAL_API_KEY"); authToken != "" {
		headers.Add("Authorization", "Key "+authToken)
		slog.Info("Added authorization header from FAL_API_KEY")
	}

	// Dial before starting goroutines so errors can be returned to caller
	websocketConn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		f.mu.Lock()
		f.inUse--
		remainingInUse := f.inUse
		f.mu.Unlock()
		slog.Error("Failed to connect to websocket", "error", err, "inUse", remainingInUse, "capacity", f.capacity)
		return nil, fmt.Errorf("failed to connect to websocket: %w", err)
	}

	// Marshal request to JSON and send.
	reqJSON, err := json.Marshal(req)
	if err != nil {
		_ = websocketConn.Close()
		f.mu.Lock()
		f.inUse--
		remainingInUse := f.inUse
		f.mu.Unlock()
		slog.Error("Failed to marshal request", "error", err, "inUse", remainingInUse, "capacity", f.capacity)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Wait for connection readiness before starting background processing or returning.
	_, readyMsg, err := websocketConn.ReadMessage()
	if err != nil {
		_ = websocketConn.Close()
		f.mu.Lock()
		f.inUse--
		remainingInUse := f.inUse
		f.mu.Unlock()
		slog.Error("Failed to read ready message", "error", err, "inUse", remainingInUse, "capacity", f.capacity)
		return nil, fmt.Errorf("failed to read ready message: %w", err)
	}

	err = performHandshake(
		readyMsg,
		func() ([]byte, error) {
			_, msg, readErr := websocketConn.ReadMessage()
			return msg, readErr
		},
		func() error {
			slog.Info("Sending request to websocket", "request", string(reqJSON))
			writeErr := websocketConn.WriteMessage(websocket.TextMessage, reqJSON)
			if writeErr == nil {
				slog.Info("Sent request to websocket", "request", string(reqJSON))
			}
			return writeErr
		},
	)
	if err != nil {
		_ = websocketConn.Close()
		f.mu.Lock()
		f.inUse--
		remainingInUse := f.inUse
		f.mu.Unlock()
		slog.Error("Failed handshake", "error", err, "message", string(readyMsg), "inUse", remainingInUse, "capacity", f.capacity)
		return nil, err
	}

	// Start websocket processing in a goroutine
	go func() {
		var writeMu sync.Mutex
		openChannels := map[string]*trickle.TrickleLocalPublisher{}

		// Ensure we decrement the counter and close all channels when this goroutine exits
		defer func() {
			_ = websocketConn.Close()

			for name, ch := range openChannels {
				if err := ch.Close(); err != nil {
					slog.Warn("Failed to close trickle channel", "channel", name, "error", err)
				}
			}
			eventsChannelName := manifestID + "-events"
			eventsCh := trickle.NewLocalPublisher(trickleSrv, eventsChannelName, "application/json")
			if err := eventsCh.Close(); err != nil {
				slog.Warn("Failed to close events trickle channel", "channel", eventsChannelName, "error", err)
			}
			controlChannelName := manifestID + "-control"
			controlCh := trickle.NewLocalPublisher(trickleSrv, controlChannelName, "application/json")
			if err := controlCh.Close(); err != nil {
				slog.Warn("Failed to close control trickle channel", "channel", controlChannelName, "error", err)
			}

			f.mu.Lock()
			f.inUse--
			remainingInUse := f.inUse
			f.mu.Unlock()
			slog.Info("Job ended", "inUse", remainingInUse, "capacity", f.capacity)
		}()

		slog.Info("Connected to websocket successfully")

		// Subscribe to the events trickle stream to detect when stream ends
		if req.EventsUrl != nil && *req.EventsUrl != "" {
			go func() {
				eventsUrl := *req.EventsUrl
				slog.Info("Subscribing to events stream", "url", eventsUrl)

				subscriber, err := trickle.NewTrickleSubscriber(trickle.TrickleSubscriberConfig{
					URL: eventsUrl,
				})
				if err != nil {
					slog.Error("Failed to create events subscriber", "error", err)
					return
				}

				const maxRetries = 5
				const retryPause = 300 * time.Millisecond
				retries := 0

				for {
					segment, err := subscriber.Read()
					if err == nil {
						retries = 0
					} else {
						if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
							slog.Info("Events stream closed, closing websocket", "reason", err)
							_ = websocketConn.Close() // This will cause ReadMessage to return an error
							return
						}

						var seqErr *trickle.SequenceNonexistent
						if errors.As(err, &seqErr) {
							// Stream exists but segment doesn't, so skip to leading edge.
							subscriber.SetSeq(seqErr.Latest)
						}

						if retries > maxRetries {
							slog.Error("Too many errors reading events stream, closing websocket", "error", err)
							_ = websocketConn.Close()
							return
						}

						slog.Warn("Error reading events stream", "error", err, "retry", retries)
						retries++
						time.Sleep(retryPause)
						continue
					}

					// Read the event data to drain the stream, but avoid logging full payload contents.
					data, err := io.ReadAll(segment.Body)
					_ = segment.Body.Close()
					if err != nil {
						slog.Warn("Error reading event body", "error", err)
						continue
					}
					slog.Info("Received event from trickle stream", "size_bytes", len(data))
				}
			}()
		} else {
			slog.Warn("No events URL provided, cannot detect stream end via trickle")
		}

		// Track missed pongs and terminate after 3 missed pongs
		const (
			pingInterval   = 10 * time.Second
			maxMissedPongs = int32(3)
		)
		var missedPongs atomic.Int32
		websocketConn.SetPongHandler(func(string) error {
			missedPongs.Store(0)
			return nil
		})
		pingDone := make(chan struct{})
		defer close(pingDone)
		go func() {
			ticker := time.NewTicker(pingInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					writeMu.Lock()
					err := websocketConn.WriteMessage(websocket.PingMessage, nil)
					writeMu.Unlock()
					if err != nil {
						slog.Warn("Failed to send ping", "error", err)
						return
					}
					if missedPongs.Add(1) >= maxMissedPongs {
						slog.Warn("Too many missed pongs, closing websocket", "missed", missedPongs.Load())
						_ = websocketConn.Close()
						return
					}
				case <-pingDone:
					return
				}
			}
		}()

		// Create a 1-hour timeout as a fail-safe to avoid big bills
		timeout := time.NewTimer(1 * time.Hour)
		defer timeout.Stop()

		// Create a channel to receive messages
		messageChan := make(chan struct {
			messageType int
			message     []byte
			err         error
		})

		// Start goroutine to read websocket messages
		go func() {
			for {
				messageType, message, err := websocketConn.ReadMessage()
				messageChan <- struct {
					messageType int
					message     []byte
					err         error
				}{messageType, message, err}
				if err != nil {
					return
				}
			}
		}()

		streamCount := 0
		const (
			// Retries may be expected due to plugin install/uninstall, so be generous.
			maxHandshakeRetries  = 20
			handshakeRetryWindow = 1 * time.Minute
		)
		handshakeRetryTimes := make([]time.Time, 0, maxHandshakeRetries)

		// Receive and handle messages indefinitely (or until events stream closes or timeout)
		for {
			select {
			case <-timeout.C:
				slog.Info("Websocket connection timeout reached (1 hour), closing connection")
				return

			case msg := <-messageChan:
				if msg.err != nil {
					slog.Info("Websocket read ended", "error", msg.err)
					return
				}

				slog.Info("Received message from websocket",
					"type", msg.messageType,
					"message", string(msg.message))

				var generic wsMessage
				if err := json.Unmarshal(msg.message, &generic); err != nil {
					slog.Warn("Failed to parse message", "error", err)
					continue
				}

				switch generic.Type {
				case "ready":
					// This is a retry, maybe due to plugin install / uninstall.

					// First check whether this is too many retries in a short time
					now := time.Now()
					cutoff := now.Add(-handshakeRetryWindow)
					filteredRetryTimes := handshakeRetryTimes[:0]
					for _, retryAt := range handshakeRetryTimes {
						if retryAt.After(cutoff) {
							filteredRetryTimes = append(filteredRetryTimes, retryAt)
						}
					}
					handshakeRetryTimes = filteredRetryTimes
					if len(handshakeRetryTimes) >= maxHandshakeRetries {
						slog.Error("Too many handshake retries in short period", "retries", len(handshakeRetryTimes), "window", handshakeRetryWindow)
						return
					}
					handshakeRetryTimes = append(handshakeRetryTimes, now)

					// Close all media channels; active streams do not survive a retry.
					for channelName, ch := range openChannels {
						if err := ch.Close(); err != nil {
							slog.Warn("Failed to close media trickle channel before handshake retry", "channel", channelName, "error", err)
							continue
						}
						delete(openChannels, channelName)
					}
					streamCount = 0

					// Perform the handshake
					err := performHandshake(
						msg.message,
						func() ([]byte, error) {
							nextMsg := <-messageChan
							if nextMsg.err != nil {
								return nil, nextMsg.err
							}
							return nextMsg.message, nil
						},
						func() error {
							slog.Info("Retrying handshake request over websocket", "request", string(reqJSON))
							writeMu.Lock()
							defer writeMu.Unlock()
							writeErr := websocketConn.WriteMessage(websocket.TextMessage, reqJSON)
							if writeErr == nil {
								slog.Info("Sent retried handshake request to websocket", "request", string(reqJSON))
							}
							return writeErr
						},
					)
					if err != nil {
						slog.Error("Handshake retry failed", "error", err)
						return
					}
					slog.Info("Handshake retry completed")

				case "create_channels":
					var startMsg createChannelsMessage
					if err := json.Unmarshal(msg.message, &startMsg); err != nil {
						slog.Warn("Failed to parse create_channels message", "error", err)
						continue
					}
					mimeType := startMsg.MimeType
					if mimeType == "" {
						mimeType = "video/MP2T"
					}

					streamCount++
					streamCountStr := strconv.Itoa(streamCount)
					var newChannels []channelInfo
					createInbound := false
					createOutbound := false
					switch strings.ToLower(startMsg.Direction) {
					case "in":
						createInbound = true
					case "out":
						createOutbound = true
					case "bidirectional":
						createInbound = true
						createOutbound = true
					default:
						slog.Warn("Ignoring create_channels with unsupported direction", "direction", startMsg.Direction)
						continue
					}

					if createInbound {
						channelName := manifestID + "-" + streamCountStr + "-in"
						ch := trickle.NewLocalPublisher(trickleSrv, channelName, mimeType)
						ch.CreateChannel()
						openChannels[channelName] = ch
						newChannels = append(newChannels, channelInfo{
							URL:       buildTrickleURL(channelName),
							Direction: "in",
							MimeType:  mimeType,
						})
					}
					if createOutbound {
						channelName := manifestID + "-" + streamCountStr + "-out"
						ch := trickle.NewLocalPublisher(trickleSrv, channelName, mimeType)
						ch.CreateChannel()
						openChannels[channelName] = ch
						newChannels = append(newChannels, channelInfo{
							URL:       buildTrickleURL(channelName),
							Direction: "out",
							MimeType:  mimeType,
						})
					}

					resp := wsResponseMessage{
						Type:      "response",
						RequestID: startMsg.RequestID,
						Channels:  newChannels,
					}
					respJSON, err := json.Marshal(resp)
					if err != nil {
						slog.Warn("Failed to marshal response message", "error", err)
						continue
					}
					writeMu.Lock()
					err = websocketConn.WriteMessage(websocket.TextMessage, respJSON)
					writeMu.Unlock()
					if err != nil {
						slog.Warn("Failed to send response message", "error", err)
						return
					}
					slog.Info("Created trickle channels for create_channels", "count", len(newChannels), "direction", startMsg.Direction, "mimeType", mimeType)

				case "close_channels":
					var stopMsg closeChannelsMessage
					if err := json.Unmarshal(msg.message, &stopMsg); err != nil {
						slog.Warn("Failed to parse close_channels message", "error", err)
						continue
					}
					for _, channelName := range stopMsg.Channels {
						ch, ok := openChannels[channelName]
						if !ok {
							slog.Warn("close_channels channel not found", "channel", channelName)
							continue
						}
						if err := ch.Close(); err != nil {
							slog.Warn("Failed to close close_channels channel", "channel", channelName, "error", err)
							continue
						}
						delete(openChannels, channelName)
						slog.Info("Closed trickle channel from close_channels", "channel", channelName)
					}
				default:
					slog.Error("Received unrecognized websocket message type", "type", generic.Type, "message", string(msg.message))
					return
				}
			}
		}
	}()

	return &LiveVideoToVideoResponse{}, nil
}

func (f *ServerlessWorker) Warm(ctx context.Context, pipeline string, modelID string, endpoint RunnerEndpoint, optimizationFlags OptimizationFlags) error {
	// Serverless workers don't need warming
	return nil
}

func (f *ServerlessWorker) Stop(ctx context.Context) error {
	// No containers to stop for serverless workers
	return nil
}

func (f *ServerlessWorker) HasCapacity(pipeline string, modelID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	hasCapacity := f.inUse < f.capacity
	clog.Info(context.Background(), "HasCapacity",
		"pipeline", pipeline,
		"modelID", modelID,
		"inUse", f.inUse,
		"capacity", f.capacity,
		"hasCapacity", hasCapacity)

	return hasCapacity
}

func (f *ServerlessWorker) EnsureImageAvailable(ctx context.Context, pipeline string, modelID string) error {
	// No images to download for serverless workers
	return nil
}

func (f *ServerlessWorker) HardwareInformation() []HardwareInformation {
	// Return empty for serverless workers
	return []HardwareInformation{}
}

func (f *ServerlessWorker) GetLiveAICapacity(pipeline, modelID string) Capacity {
	f.mu.Lock()
	defer f.mu.Unlock()

	idleCapacity := f.capacity - f.inUse
	if idleCapacity < 0 {
		idleCapacity = 0
	}

	clog.Info(context.Background(), "GetLiveAICapacity",
		"pipeline", pipeline,
		"modelID", modelID,
		"inUse", f.inUse,
		"idleCapacity", idleCapacity)

	return Capacity{
		ContainersInUse: f.inUse,
		ContainersIdle:  idleCapacity,
	}
}

func (f *ServerlessWorker) Version() []Version {
	// Return a default version for serverless workers
	return []Version{
		{
			Version: "serverless-1.0.0",
		},
	}
}

func performHandshake(
	readyMsg []byte,
	readMsg func() ([]byte, error),
	sendReq func() error,
) error {
	var readyResponse wsHandshakeMessage
	if err := json.Unmarshal(readyMsg, &readyResponse); err != nil {
		return fmt.Errorf("failed to parse ready message: %w", err)
	}
	if readyResponse.Type != "ready" {
		return fmt.Errorf("did not receive ready message from websocket")
	}
	slog.Info("Received ready message", "message", readyResponse.Message)

	if err := sendReq(); err != nil {
		return fmt.Errorf("failed to send request message: %w", err)
	}

	startedMsg, err := readMsg()
	if err != nil {
		return fmt.Errorf("failed to read started message: %w", err)
	}

	var startedResponse wsHandshakeMessage
	if err := json.Unmarshal(startedMsg, &startedResponse); err != nil {
		return fmt.Errorf("failed to parse started message: %w", err)
	}
	if startedResponse.Type == "error" {
		statusCode := http.StatusInternalServerError
		if startedResponse.Code == "ACCESS_DENIED" {
			statusCode = http.StatusUnauthorized
		}
		return &ServerlessHandshakeError{
			StatusCode: statusCode,
			Code:       startedResponse.Code,
			Message:    firstNonEmpty(startedResponse.Error, startedResponse.Message),
		}
	}
	if startedResponse.Type != "started" {
		return fmt.Errorf("unexpected message type %q between ready and started", startedResponse.Type)
	}
	slog.Info("Received started message")

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
