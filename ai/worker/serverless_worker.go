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

	if req.SubscribeUrl == "" {
		return nil, fmt.Errorf("serverless worker requires subscribe URL for live-video-to-video")
	}
	subscribeURL, err := url.Parse(req.SubscribeUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid subscribe URL: %w", err)
	}
	if !strings.HasSuffix(subscribeURL.Path, manifestID) {
		return nil, fmt.Errorf("subscribe URL path %q must end with manifest ID %q", subscribeURL.Path, manifestID)
	}
	trickleBasePath := strings.TrimSuffix(subscribeURL.Path, manifestID)
	buildTrickleURL := func(channelName string) string {
		if trickleBasePath == "" {
			return ""
		}
		u := *subscribeURL
		u.Path = trickleBasePath
		u = *u.JoinPath(channelName)
		return u.String()
	}
	if buildTrickleURL(manifestID+"-test") == "" {
		return nil, fmt.Errorf("could not construct trickle channel URL from subscribe URL %q", req.SubscribeUrl)
	}

	wsURL := f.wsURL
	if req.Params != nil {
		if wsURLRaw, ok := (*req.Params)["ws_url"]; ok {
			wsURLOverride, ok := wsURLRaw.(string)
			if !ok {
				return nil, fmt.Errorf("invalid params.ws_url: must be a string")
			}
			if wsURLOverride != "" {
				parsedWSURL, err := url.Parse(wsURLOverride)
				if err != nil || parsedWSURL.Host == "" || (parsedWSURL.Scheme != "ws" && parsedWSURL.Scheme != "wss") {
					return nil, fmt.Errorf("invalid params.ws_url: must be a valid ws or wss URL")
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
		headers.Add("Authorization", authToken)
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

	// Start websocket processing in a goroutine
	go func() {
		var writeMu sync.Mutex
		openChannels := map[string]*trickle.TrickleLocalPublisher{}

		// Ensure we decrement the counter and close all local channels when this goroutine exits
		defer func() {
			_ = websocketConn.Close()

			for name, ch := range openChannels {
				if err := ch.Close(); err != nil {
					slog.Warn("Failed to close trickle channel", "channel", name, "error", err)
				}
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
					Ctx: context.Background(),
				})
				if err != nil {
					slog.Error("Failed to create events subscriber", "error", err)
					return
				}

				for {
					segment, err := subscriber.Read()
					if err != nil {
						if errors.Is(err, trickle.EOS) || errors.Is(err, trickle.StreamNotFoundErr) {
							slog.Info("Events stream closed, closing websocket", "reason", err)
							_ = websocketConn.Close() // This will cause ReadMessage to return an error
							return
						}
						slog.Warn("Error reading events stream", "error", err)
						// Continue on other errors (like SequenceNonexistent)
						continue
					}

					// Read and print the event data for debugging
					data, err := io.ReadAll(segment.Body)
					_ = segment.Body.Close()
					if err != nil {
						slog.Warn("Error reading event body", "error", err)
						continue
					}
					slog.Info("Received event from trickle stream", "data", string(data))
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

		// Wait for connection_established message from server
		_, readyMsg, err := websocketConn.ReadMessage()
		if err != nil {
			slog.Error("Failed to read ready message", "error", err)
			return
		}

		// Parse the ready message
		var readyResponse map[string]interface{}
		if err := json.Unmarshal(readyMsg, &readyResponse); err != nil {
			slog.Error("Failed to parse ready message", "error", err, "message", string(readyMsg))
			return
		}

		// Verify it's the connection_established message.
		if msgType, ok := readyResponse["type"].(string); ok && msgType == "connection_established" {
			slog.Info("Received connection ready message", "message", readyResponse["message"])
		} else {
			slog.Warn("Unexpected ready message format", "message", string(readyMsg))
		}

		// Marshal request to JSON and send.
		reqJSON, err := json.Marshal(req)
		if err != nil {
			slog.Error("Failed to marshal request", "error", err)
			return
		}

		slog.Info("Sending request to websocket", "request", string(reqJSON))

		// TODO check response message and retry on failure.
		writeMu.Lock()
		err = websocketConn.WriteMessage(websocket.TextMessage, reqJSON)
		writeMu.Unlock()
		if err != nil {
			slog.Error("Failed to send message", "error", err)
			return
		}

		slog.Info("Sent request to websocket", "request", string(reqJSON))

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
					slog.Debug("Ignoring websocket message type", "type", generic.Type)
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
