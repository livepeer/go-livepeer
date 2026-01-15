package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/livepeer/go-livepeer/clog"
	"github.com/livepeer/go-livepeer/trickle"
)

type ServerlessWorker struct {
}

// NewServerlessWorker creates a new ServerlessWorker instance
func NewServerlessWorker() (*ServerlessWorker, error) {
	return &ServerlessWorker{}, nil
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
	// Start websocket connection in a goroutine
	go func() {
		wsURL := "wss://fal.run/Daydream/scope-runner-app/live-video-to-video"
		slog.Info("Connecting to websocket", "url", wsURL)

		// Prepare headers with authorization
		headers := http.Header{}
		if authToken := os.Getenv("FAL_API_KEY"); authToken != "" {
			headers.Add("Authorization", authToken)
			slog.Info("Added authorization header from FAL_API_KEY")
		}

		// Connect to websocket
		websocketConn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			slog.Error("Failed to connect to websocket", "error", err)
			return
		}
		defer websocketConn.Close()

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
							websocketConn.Close() // This will cause ReadMessage to return an error
							return
						}
						slog.Warn("Error reading events stream", "error", err)
						// Continue on other errors (like SequenceNonexistent)
						continue
					}

					// Read and print the event data for debugging
					data, err := io.ReadAll(segment.Body)
					segment.Body.Close()
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

		slog.Info("Connected to websocket successfully")

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

		// Verify it's the connection_established message
		if msgType, ok := readyResponse["type"].(string); ok && msgType == "connection_established" {
			slog.Info("Received connection ready message", "message", readyResponse["message"])
		} else {
			slog.Warn("Unexpected ready message format", "message", string(readyMsg))
		}

		// Marshal request to JSON and send
		reqJSON, err := json.Marshal(req)
		if err != nil {
			slog.Error("Failed to marshal request", "error", err)
			return
		}

		slog.Info("Sending request to websocket", "request", string(reqJSON))

		// TODO check response message and retry on failure
		err = websocketConn.WriteMessage(websocket.TextMessage, reqJSON)
		if err != nil {
			slog.Error("Failed to send message", "error", err)
			return
		}

		slog.Info("Sent request to websocket", "request", string(reqJSON))

		// Receive and print messages indefinitely (or until events stream closes)
		for {
			messageType, message, err := websocketConn.ReadMessage()
			if err != nil {
				slog.Info("Websocket read ended", "error", err)
				return
			}

			slog.Info("Received message from websocket",
				"type", messageType,
				"message", string(message))

			// Parse the message to check for health_check status
			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err != nil {
				slog.Warn("Failed to parse message", "error", err)
				continue
			}

			// TODO overall long timeout for websocket connection as a fail safe to avoid big bills

			// Check if this is a health_check message
			// TODO handle the states like docker.go watchContainer i.e. anything else apart from ERROR to handle?
			if msgType, ok := msgData["type"].(string); ok && msgType == "health_check" {
				// Extract the nested data.status field
				if data, ok := msgData["data"].(map[string]interface{}); ok {
					if status, ok := data["status"].(string); ok {
						slog.Info("Health check status", "status", status)

						if status == "ERROR" {
							slog.Info("Runner status is ERROR, closing websocket")
							return
						}
					}
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
	// Serverless workers always have capacity
	// TODO implement max capacity
	clog.Info(context.Background(), "HasCapacity", "pipeline", pipeline, "modelID", modelID)
	return true
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
	// Return unlimited capacity for serverless workers
	clog.Info(context.Background(), "GetLiveAICapacity", "pipeline", pipeline, "modelID", modelID)
	// TODO implement
	return Capacity{
		ContainersInUse: 0,
		ContainersIdle:  1,
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
