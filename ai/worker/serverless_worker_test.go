package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gorilla/websocket"
)

func newTestLockedWebSocket(t *testing.T) (*lockedWebSocket, <-chan []byte, func()) {
	t.Helper()

	recv := make(chan []byte, 32)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			defer close(recv)
			defer conn.Close()
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				recv <- append([]byte(nil), msg...)
			}
		}()
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("failed to dial test websocket: %v", err)
	}
	client := newLockedWebSocket(clientConn)

	cleanup := func() {
		_ = client.Close()
		server.Close()
	}
	return client, recv, cleanup
}

func TestPerformHandshakeSuccess(t *testing.T) {
	t.Parallel()

	ws, recv, cleanup := newTestLockedWebSocket(t)
	defer cleanup()

	reqJSON := []byte(`{"type":"request"}`)
	err := performHandshake(
		context.Background(),
		[]byte(`{"type":"ready","message":"ok"}`),
		func() ([]byte, error) {
			return []byte(`{"type":"started"}`), nil
		},
		ws,
		reqJSON,
	)
	if err != nil {
		t.Fatalf("performHandshake() returned error: %v", err)
	}
	select {
	case sent := <-recv:
		if string(sent) != string(reqJSON) {
			t.Fatalf("performHandshake() sent wrong request: got=%s want=%s", string(sent), string(reqJSON))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("performHandshake() did not send request")
	}
}

func TestPerformHandshakeAccessDenied(t *testing.T) {
	t.Parallel()

	ws, _, cleanup := newTestLockedWebSocket(t)
	defer cleanup()

	err := performHandshake(
		context.Background(),
		[]byte(`{"type":"ready"}`),
		func() ([]byte, error) {
			return []byte(`{"type":"error","code":"ACCESS_DENIED","error":"Access denied"}`), nil
		},
		ws,
		[]byte(`{"type":"request"}`),
	)
	if err == nil {
		t.Fatal("performHandshake() expected error, got nil")
	}

	var hsErr *ServerlessHandshakeError
	if !errors.As(err, &hsErr) {
		t.Fatalf("performHandshake() expected ServerlessHandshakeError, got %T: %v", err, err)
	}
	if hsErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", hsErr.StatusCode, http.StatusUnauthorized)
	}
	if hsErr.Code != "ACCESS_DENIED" {
		t.Fatalf("unexpected handshake code: %q", hsErr.Code)
	}
	if hsErr.Message != "Access denied" {
		t.Fatalf("unexpected handshake message: %q", hsErr.Message)
	}
}

func TestLiveVideoToVideoRequiresControlURL(t *testing.T) {
	t.Parallel()

	worker := &ServerlessWorker{}
	manifestID := "manifest"

	_, err := worker.LiveVideoToVideo(t.Context(), GenLiveVideoToVideoJSONRequestBody{
		ManifestId:   &manifestID,
		SubscribeUrl: "https://example.com/ai/trickle/manifest",
	})
	if err == nil {
		t.Fatal("LiveVideoToVideo() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires control URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLiveVideoToVideoUsesControlURLForTrickleBase(t *testing.T) {
	t.Parallel()

	worker := &ServerlessWorker{}
	manifestID := "manifest"
	controlURL := "https://example.com/ai/trickle/manifest-control"

	_, err := worker.LiveVideoToVideo(t.Context(), GenLiveVideoToVideoJSONRequestBody{
		ManifestId:   &manifestID,
		ControlUrl:   &controlURL,
		SubscribeUrl: "not a valid url",
	})
	if err == nil {
		t.Fatal("LiveVideoToVideo() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires trickle server") {
		t.Fatalf("expected trickle server error after control URL validation, got: %v", err)
	}
}

func TestLiveVideoToVideoInvalidWSURLReturnsBadRequest(t *testing.T) {
	tests := []struct {
		name       string
		wsURLValue any
		wantErr    string
	}{
		{
			name:       "non-string override",
			wsURLValue: 123,
			wantErr:    "invalid params.ws_url: must be a string",
		},
		{
			name:       "unacceptable override",
			wsURLValue: "ws://other-host/socket",
			wantErr:    "unacceptable params.ws_url",
		},
		{
			name:       "invalid override",
			wsURLValue: "ws://allowed-host:badport/socket",
			wantErr:    "invalid params.ws_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LIVE_AI_WS_PREFIX", "ws://allowed-host")

			worker := &ServerlessWorker{wsURL: "ws://default-host/socket"}
			manifestID := "manifest"
			controlURL := "https://example.com/ai/trickle/manifest-control"
			params := map[string]interface{}{"ws_url": tt.wsURLValue}

			_, err := worker.LiveVideoToVideo(t.Context(), GenLiveVideoToVideoJSONRequestBody{
				ManifestId: &manifestID,
				ControlUrl: &controlURL,
				Params:     &params,
			})
			if err == nil {
				t.Fatal("LiveVideoToVideo() expected error, got nil")
			}

			var hsErr *ServerlessHandshakeError
			if !errors.As(err, &hsErr) {
				t.Fatalf("LiveVideoToVideo() expected ServerlessHandshakeError, got %T: %v", err, err)
			}
			if hsErr.StatusCode != http.StatusBadRequest {
				t.Fatalf("unexpected status code: got=%d want=%d", hsErr.StatusCode, http.StatusBadRequest)
			}
			if hsErr.Code != "INVALID_WS" {
				t.Fatalf("unexpected handshake code: got=%q want=%q", hsErr.Code, "INVALID_WS")
			}
			if hsErr.Message != tt.wantErr {
				t.Fatalf("unexpected handshake message: got=%q want=%q", hsErr.Message, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("unexpected error string: got=%q want=%q", err.Error(), tt.wantErr)
			}
			if _, ok := params["ws_url"]; !ok {
				t.Fatalf("ws_url should remain in params on validation failure: params=%v", params)
			}
		})
	}
}

func TestGetServerlessTimeoutDefault(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT", "")

	if got := getServerlessTimeout(); got != time.Hour {
		t.Fatalf("unexpected default serverless timeout: got=%v want=%v", got, time.Hour)
	}
}

func TestGetServerlessTimeoutFromEnv(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT", "90m")

	if got := getServerlessTimeout(); got != 90*time.Minute {
		t.Fatalf("unexpected configured serverless timeout: got=%v want=%v", got, 90*time.Minute)
	}
}

func TestGetServerlessTimeoutInvalidEnvFallsBack(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT", "not-a-duration")

	if got := getServerlessTimeout(); got != time.Hour {
		t.Fatalf("unexpected fallback serverless timeout: got=%v want=%v", got, time.Hour)
	}
}

func TestGetServerlessTimeoutNonPositiveFallsBack(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT", "0s")

	if got := getServerlessTimeout(); got != time.Hour {
		t.Fatalf("unexpected fallback serverless timeout for non-positive value: got=%v want=%v", got, time.Hour)
	}
}

func TestGetServerlessTimeoutGracePeriodDefault(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT_GRACE_PERIOD", "")

	if got := getServerlessTimeoutGracePeriod(); got != 5*time.Second {
		t.Fatalf("unexpected default serverless timeout grace period: got=%v want=%v", got, 5*time.Second)
	}
}

func TestGetServerlessTimeoutGracePeriodFromEnv(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT_GRACE_PERIOD", "2500ms")

	if got := getServerlessTimeoutGracePeriod(); got != 2500*time.Millisecond {
		t.Fatalf("unexpected configured serverless timeout grace period: got=%v want=%v", got, 2500*time.Millisecond)
	}
}

func TestGetServerlessTimeoutGracePeriodInvalidEnvFallsBack(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT_GRACE_PERIOD", "not-a-duration")

	if got := getServerlessTimeoutGracePeriod(); got != 5*time.Second {
		t.Fatalf("unexpected fallback serverless timeout grace period: got=%v want=%v", got, 5*time.Second)
	}
}

func TestGetServerlessTimeoutGracePeriodNonPositiveFallsBack(t *testing.T) {
	t.Setenv("LIVE_AI_SERVERLESS_TIMEOUT_GRACE_PERIOD", "0s")

	if got := getServerlessTimeoutGracePeriod(); got != 5*time.Second {
		t.Fatalf("unexpected fallback serverless timeout grace period for non-positive value: got=%v want=%v", got, 5*time.Second)
	}
}

func TestServerlessClosingMessagePayload(t *testing.T) {
	payload, err := json.Marshal(wsClosingMessage{
		Type:    "closing",
		Message: "live-video-to-video session timeout reached; terminating shortly",
	})
	if err != nil {
		t.Fatalf("failed to marshal closing message JSON: %v", err)
	}

	var msg wsClosingMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("failed to unmarshal closing message JSON: %v", err)
	}
	if msg.Type != "closing" {
		t.Fatalf("unexpected closing message type: got=%q want=%q", msg.Type, "closing")
	}
	if msg.Message != "live-video-to-video session timeout reached; terminating shortly" {
		t.Fatalf("unexpected closing message text: got=%q", msg.Message)
	}
}

func TestRestartMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var missedPongs atomic.Int32
		missedPongs.Store(2)

		go func() {
			time.Sleep(20 * time.Millisecond)
			missedPongs.Store(0)
		}()

		start := time.Now()
		respJSON, err := handleRestartingMessage(
			context.Background(),
			[]byte(`{"type":"restarting","request_id":"restart-1"}`),
			&missedPongs,
		)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("handleRestartingMessage() returned error: %v", err)
		}
		if elapsed < 100*time.Millisecond {
			t.Fatalf("handleRestartingMessage() should wait for pong recovery, elapsed=%v", elapsed)
		}
		if elapsed >= 200*time.Millisecond {
			t.Fatalf("handleRestartingMessage() did not observe concurrent pong recovery, elapsed=%v", elapsed)
		}
		if got := missedPongs.Load(); got != 0 {
			t.Fatalf("handleRestartingMessage() should reset missed pongs, got %d", got)
		}

		var resp wsResponseMessage
		if err := json.Unmarshal(respJSON, &resp); err != nil {
			t.Fatalf("failed to unmarshal restarting response JSON: %v", err)
		}
		if resp.Type != "response" {
			t.Fatalf("unexpected response type: got=%q want=%q", resp.Type, "response")
		}
		if resp.RequestID != "restart-1" {
			t.Fatalf("unexpected request_id: got=%q want=%q", resp.RequestID, "restart-1")
		}
	})
}

func TestStartPingingStopIsIdempotent(t *testing.T) {
	ws, _, cleanup := newTestLockedWebSocket(t)
	defer cleanup()

	var missedPongs atomic.Int32
	stopPings := startPinging(
		context.Background(),
		20*time.Millisecond,
		3,
		&missedPongs,
		ws,
	)

	stopPings()
	stopPings()
}

func TestStartPingingRestartsWithFullIntervalGrace(t *testing.T) {
	ws, _, cleanup := newTestLockedWebSocket(t)
	defer cleanup()

	const interval = 40 * time.Millisecond
	var missedPongs atomic.Int32

	stopPings := startPinging(
		context.Background(),
		interval,
		100, // keep this test focused on timer behavior
		&missedPongs,
		ws,
	)

	time.Sleep(interval - 10*time.Millisecond)
	if got := missedPongs.Load(); got != 0 {
		t.Fatalf("expected no ping before first interval, got=%d", got)
	}

	// Simulate restart: stop the existing pinger before its next tick.
	stopPings()

	// Simulate successful re-handshake: start a fresh pinger.
	stopPings = startPinging(
		context.Background(),
		interval,
		100,
		&missedPongs,
		ws,
	)
	defer stopPings()

	time.Sleep(interval - 10*time.Millisecond)
	if got := missedPongs.Load(); got != 0 {
		t.Fatalf("expected full interval grace after restart, got=%d", got)
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if missedPongs.Load() == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected first ping after full interval, got=%d", missedPongs.Load())
}
