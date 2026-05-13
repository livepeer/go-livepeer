# Live Streaming Architecture: BYOC E2E Pipeline

## Overview

Real-time video-to-video AI inference pipeline using Livepeer's trickle protocol, connecting browser webcam input to GPU-accelerated processing on fal.ai H100, with MPEG-TS transport and MSE browser playback.

## Component Diagram

```
+---------------+     +----------------+     +----------------+     +---------------+     +------------------+
|    Browser    |     |  SDK Service   |     |  Orchestrator  |     |    Adapter    |     | scope-trickle    |
|  (Storyboard) |     |  (Cloud Run)   |     |   (VM:8935)    |     |   (VM:9093)   |     |  (fal.ai H100)   |
+-------+-------+     +-------+--------+     +-------+--------+     +-------+-------+     +--------+---------+
        |                      |                      |                      |                       |
        |  HTTPS               |  HTTPS (self-sign)   |  Docker network      |  HTTPS (fal.run)      |
        |  (nip.io TLS)        |  via _orch_request    |  byoc_network        |  query params         |
        +----------------------+----------------------+----------------------+-----------------------+
```

## Components & State Management

| Component | Runs On | Session State | Trickle Channels |
|-----------|---------|---------------|------------------|
| **SDK Service** | Cloud Run (`--session-affinity`) | `_stream_sessions` dict: `MediaPublish` (input encoder), `MediaOutput` (output decoder), `latest_frame_jpeg` | Reads/writes via `ORCH_URL` (self-signed TLS) |
| **Orchestrator** | VM Docker `byoc_orch:8935` | `ExternalCapabilities.AddStream()`, 5-segment circular buffer per channel, 5-min idle sweep | **Creates** all channels: `{id}`, `{id}-out`, `{id}-control`, `{id}-events` |
| **Adapter** | VM Docker `byoc_stream_adapter:9093` | `StreamSession` per stream, lifecycle-only (no media path) | None (just calls scope-trickle `/start`) |
| **scope-trickle** | fal.ai H100 GPU | `_frame_processors` dict, `_session_status` dict, `FrameProcessor` per session | Subscribes `{id}` (input), publishes `{id}-out` (output), subscribes `{id}-control` |

## Endpoints

### SDK Service (Cloud Run)
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/stream/start` | Start stream, create trickle channels, init MPEG-TS session |
| POST | `/stream/{id}/publish` | Browser JPEG -> MPEG-TS encode -> trickle input |
| GET | `/stream/{id}/frame` | Poll latest decoded output JPEG (from MPEG-TS) |
| POST | `/stream/{id}/stop` | Stop stream, cleanup sessions |
| POST | `/stream/{id}/whep` | WHEP proxy (for future WebRTC playback) |

### Orchestrator (VM:8935)
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/ai/stream/start` | Create trickle channels, dispatch to adapter |
| POST | `/ai/stream/stop` | Tear down trickle channels |
| GET/POST | `/ai/trickle/{channel}/{seq}` | Trickle protocol (subscribe/publish) |
| POST | `/capability/register` | Adapter capability registration |

### Adapter (VM:9093)
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/stream/start` | Receive trickle URLs, call scope-trickle `/start` |
| POST | `/stream/stop` | Stop stream session |
| GET | `/health` | Health check + active stream count |
| GET | `/capabilities` | List registered capabilities |

### scope-trickle (fal.ai)
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/start?subscribe_url=...&publish_url=...` | Start GPU session (query params!) |
| POST | `/stop` | Stop session |
| POST | `/update-params?prompt=...` | Update FrameProcessor parameters |
| POST | `/session-status` | Get frames_in/out per session |
| POST | `/debug` | Pipeline status, connectivity test |

## Sequence Diagram

### Stream Start

```
Browser              SDK Service           Orchestrator          Adapter            scope-trickle
  |                  (Cloud Run)            (VM:8935)           (VM:9093)           (fal H100)
  |                      |                      |                   |                    |
  | POST /stream/start   |                      |                   |                    |
  |--------------------->|                      |                   |                    |
  |                      | POST /ai/stream/start|                   |                    |
  |                      | + Livepeer header     |                   |                    |
  |                      | (base64 job request)  |                   |                    |
  |                      |--------------------->|                   |                    |
  |                      |                      |                   |                    |
  |                      |                      | 1. Create trickle channels:            |
  |                      |                      |    {id}         (video input)           |
  |                      |                      |    {id}-out     (video output)          |
  |                      |                      |    {id}-control (JSON control)          |
  |                      |                      |    {id}-events  (JSON events)           |
  |                      |                      |                   |                    |
  |                      |                      | 2. Rewrite URLs:  |                    |
  |                      |                      |    byoc_orch:8935 |                    |
  |                      |                      |    -> nip.io:443  |                    |
  |                      |                      |                   |                    |
  |                      |                      | 3. POST /stream/start                  |
  |                      |                      |    {subscribe_url, publish_url,         |
  |                      |                      |     control_url, events_url}            |
  |                      |                      |------------------>|                    |
  |                      |                      |                   |                    |
  |                      |                      |                   | 4. POST /start?    |
  |                      |                      |                   |    subscribe_url=  |
  |                      |                      |                   |    publish_url=    |
  |                      |                      |                   |    manifest_id=    |
  |                      |                      |                   |    (query params!) |
  |                      |                      |                   |------------------->|
  |                      |                      |                   |                    |
  |                      |                      |                   |                    | 5. In background thread:
  |                      |                      |                   |                    |    Create FrameProcessor
  |                      |                      |                   |                    |    Start _media_input_loop
  |                      |                      |                   |                    |    Start _media_output_loop
  |                      |                      |                   |                    |    Start _control_loop
  |                      |                      |                   |                    |
  |                      |                      |                   | 200 {session_id}   |
  |                      |                      |                   |<-------------------|
  |                      |                      | 200               |                    |
  |                      |                      |<------------------|                    |
  |                      |                      |                   |                    |
  |                      | 200 + trickle URLs   |                   |                    |
  |                      | (rewritten to nip.io)|                   |                    |
  |                      |<---------------------|                   |                    |
  |                      |                      |                   |                    |
  |                      | 6. _init_stream_session():               |                    |
  |                      |    Create MediaPublish (MPEG-TS encoder) |                    |
  |                      |    Create MediaOutput (MPEG-TS decoder)  |                    |
  |                      |    Start _output_decode_loop (background)|                    |
  |                      |                      |                   |                    |
  | 200 {stream_id,      |                      |                   |                    |
  |  subscribe_url,      |                      |                   |                    |
  |  publish_url,        |                      |                   |                    |
  |  control_url}        |                      |                   |                    |
  |<---------------------|                      |                   |                    |
```

### Media Streaming (Continuous)

```
Browser              SDK Service           Orchestrator                              scope-trickle
  |                  (Cloud Run)            (VM:8935)                                (fal H100)
  |                      |                      |                                        |
  |======================|======================|========================================|
  |                    INPUT PATH (Browser -> GPU)                                       |
  |======================|======================|========================================|
  |                      |                      |                                        |
  | POST /stream/{id}/   |                      |                                        |
  |      publish         |                      |                                        |
  | (JPEG from webcam)   |                      |                                        |
  |--------------------->|                      |                                        |
  |                      |                      |                                        |
  |                      | JPEG -> PIL Image    |                                        |
  |                      | -> numpy RGB24       |                                        |
  |                      | -> pyav VideoFrame   |                                        |
  |                      | -> MediaPublish      |                                        |
  |                      |   .write_frame()     |                                        |
  |                      | (H264 encode ->      |                                        |
  |                      |  MPEG-TS segment)    |                                        |
  |                      |                      |                                        |
  |                      | POST /ai/trickle/    |                                        |
  |                      |  {id}/{seq}          |                                        |
  |                      | (MPEG-TS, video/MP2T)|                                        |
  |                      |--------------------->|                                        |
  |                      |                      | Store in circular                      |
  |                      |                      | buffer (5 segments)                    |
  |                      |                      |                                        |
  |                      |                      |            GET /ai/trickle/{id}/{seq}  |
  |                      |                      |            (long-poll, blocks until    |
  |                      |                      |             data available)            |
  |                      |                      |<---------------------------------------|
  |                      |                      |                                        |
  |                      |                      | Return MPEG-TS segment                 |
  |                      |                      |--------------------------------------->|
  |                      |                      |                                        |
  |                      |                      |              MediaOutput.frames()      |
  |                      |                      |              -> decode MPEG-TS         |
  |                      |                      |              -> VideoFrame             |
  |                      |                      |                                        |
  |                      |                      |              FrameProcessor.put(frame) |
  |                      |                      |                     |                  |
  |                      |                      |              +------v------+           |
  |                      |                      |              |  H100 GPU   |           |
  |                      |                      |              |  longlive   |           |
  |                      |                      |              |  pipeline   |           |
  |                      |                      |              +------+------+           |
  |                      |                      |                     |                  |
  |                      |                      |              FrameProcessor.get()      |
  |                      |                      |              -> processed tensor       |
  |                      |                      |                                        |
  |======================|======================|========================================|
  |                   OUTPUT PATH (GPU -> Browser)                                       |
  |======================|======================|========================================|
  |                      |                      |                                        |
  |                      |                      |              VideoFrame.from_ndarray() |
  |                      |                      |              -> MediaPublish           |
  |                      |                      |                .write_frame()          |
  |                      |                      |              (H264 encode ->           |
  |                      |                      |               MPEG-TS segment)         |
  |                      |                      |                                        |
  |                      |                      |   POST /ai/trickle/{id}-out/{seq}     |
  |                      |                      |   (MPEG-TS segment, via nip.io HTTPS) |
  |                      |                      |<---------------------------------------|
  |                      |                      |                                        |
  |                      |                      | Store in circular                      |
  |                      |                      | buffer (5 segments)                    |
  |                      |                      |                                        |
  | GET /ai/trickle/     |                      |                                        |
  |  {id}-out/{seq}      |                      |                                        |
  | (direct HTTPS via    |                      |                                        |
  |  nip.io + nginx)     |                      |                                        |
  |------------------------------------->------->|                                        |
  |                      |                      |                                        |
  | MPEG-TS segment      |                      |                                        |
  |<-------------------------------------<-------|                                        |
  |                      |                      |                                        |
  | mux.js transmux:     |                      |                                        |
  | MPEG-TS -> fMP4      |                      |                                        |
  | -> MSE SourceBuffer  |                      |                                        |
  | -> <video> playback  |                      |                                        |
  | (hardware H264 decode)|                     |                                        |
```

### Prompt Update

```
Browser                                                                          scope-trickle
  |                                                                               (fal H100)
  |                                                                                    |
  | POST https://fal.run/Daydream/scope-trickle/update-params?prompt=cyberpunk+city    |
  |------------------------------------------------------------------------>---------->|
  |                                                                                    |
  |                                                                   frame_processor  |
  |                                                                   .update_params() |
  |                                                                   GPU applies new  |
  |                                                                   prompt instantly  |
  |                                                                                    |
  | 200 {"status": "updated", "params": {"prompts": [...]}}                            |
  |<------------------------------------------------------------------------<----------|
```

## Media Encoding Pipeline

Each hop in the pipeline converts between these formats:

```
Browser:        JPEG (webcam capture, quality=0.6)
                  |
SDK Service:    JPEG -> PIL Image -> numpy RGB24 -> pyav VideoFrame
                  |
MediaPublish:   VideoFrame -> H264 encode -> MPEG-TS segment
                  |
Trickle:        MPEG-TS bytes (HTTP POST/GET, Content-Type: video/MP2T)
                  |
scope-trickle:  MPEG-TS -> MediaOutput.frames() -> VideoFrame
                  |
FrameProcessor: VideoFrame -> numpy tensor -> GPU (longlive) -> tensor
                  |
MediaPublish:   tensor -> VideoFrame -> H264 encode -> MPEG-TS segment
                  |
Trickle:        MPEG-TS bytes (HTTP, via nip.io nginx proxy)
                  |
Browser:        MPEG-TS -> mux.js transmux -> fMP4 -> MSE -> <video>
                (hardware H264 decode)
```

## Trickle Protocol Details

### Channel Naming
```
{stream_id}          - Video input (browser publishes, scope subscribes)
{stream_id}-out      - Video output (scope publishes, browser subscribes)
{stream_id}-control  - JSON control messages (parameter updates)
{stream_id}-events   - JSON status events
{stream_id}-data     - Optional auxiliary data output
```

### HTTP Protocol
```
POST /{channel}/{seq}  - Publish segment (blocks until body is fully sent)
GET  /{channel}/{seq}  - Subscribe/read segment (blocks until data available)
GET  /{channel}/-1     - Get next segment (live start)
GET  /{channel}/-2     - Get current/latest segment

Response headers:
  Lp-Trickle-Seq: <int>      - Actual segment sequence received
  Lp-Trickle-Latest: <int>   - Latest available sequence on server

Status codes:
  200 - Data available
  404 - Channel not found (deleted or expired)
  470 - Channel exists but segment not in buffer (skip to Latest)
```

### Server Internals
- **Circular buffer**: 5 segments per channel (`maxSegmentsPerStream`)
- **Idle sweep**: channels with no writes for 5 minutes are deleted
- **Preconnect**: publisher/subscriber can establish HTTP connection before data, reducing latency

## Configuration

### SDK Service (Cloud Run)
```
ORCH_URL=https://34.134.195.88:8935          # Direct to orch (self-signed TLS)
ORCH_PUBLIC_URL=https://34-134-195-88.nip.io  # Public URL for trickle URL rewriting
--session-affinity                             # Same client -> same instance
--min-instances 1                              # Keep warm for MediaPublish sessions
```

### Orchestrator (Docker)
```
-orchestrator -network offchain
-serviceAddr 0.0.0.0:8935
-orchSecret offchain-test-secret-2026
-liveAITrickleHostForRunner 34-134-195-88.nip.io:443   # Public trickle URLs for workers
```

### Adapter (Docker)
```
ORCH_URL=https://byoc_orch:8935               # Internal Docker network
ORCH_SECRET=offchain-test-secret-2026
ADAPTER_CALLBACK_URL=http://byoc_stream_adapter:9093
SCOPE_TRICKLE_ENDPOINT=https://fal.run/Daydream/scope-trickle  # Enables lifecycle-only mode
FAL_KEY=<fal-api-key>
CAPABILITIES=[{"name":"scope-live","model_id":"daydream/scope-app","capacity":2}]
REGISTER_INTERVAL=15                           # Re-register every 15s
```

### scope-trickle (fal.ai)
```python
machine_type = "GPU-H100"
max_multiplexing = 10
keep_alive = 300                               # Keep runner warm 5 min
requirements = [
    "livepeer-gateway @ git+https://github.com/j0sh/livepeer-python-gateway",
    "Pillow", "numpy", "requests", "httpx"
]
# MediaPublish output config:
fps = 30.0
keyframe_interval_s = 2.0                      # 2s MPEG-TS segments
```

### nginx (VM, TLS termination)
```
server_name 34-134-195-88.nip.io;
ssl_certificate /etc/letsencrypt/live/.../fullchain.pem;

location / {
    proxy_pass https://127.0.0.1:8935;        # Forward to orch
    proxy_ssl_verify off;                       # Self-signed OK
    proxy_read_timeout 300s;                    # Long-poll support
    add_header 'Access-Control-Allow-Origin' '*';
    add_header 'Access-Control-Expose-Headers' 'Lp-Trickle-Seq, Lp-Trickle-Latest';
}
```

## Infrastructure

```
VM: 34.134.195.88 (GCE byoc-node, us-central1-a)
+-----------------------------------------------------------+
| nginx (:80/:443) -- Let's Encrypt TLS                     |
|   -> proxy to orch :8935 (self-signed)                    |
|                                                           |
| Docker (byoc_network):                                    |
|   byoc_orch (:8935/:7935) -- go-livepeer orchestrator     |
|   byoc_adapter (:9090) -- batch inference adapter (fal)   |
|   byoc_stream_adapter (:9093) -- live stream adapter      |
|   byoc_proxy (:8080) -- serverless proxy                  |
+-----------------------------------------------------------+

Cloud Run: livepeer-sdk-service (us-central1)
+-----------------------------------------------------------+
| SDK Service (FastAPI/uvicorn)                             |
| Session-affinity enabled, min-instances=1                 |
| Handles MPEG-TS encode/decode for browser                 |
+-----------------------------------------------------------+

fal.ai: Daydream/scope-trickle (H100 GPU)
+-----------------------------------------------------------+
| scope-trickle (fal App, keep_alive=300)                   |
| In-process PipelineManager + FrameProcessor               |
| longlive pipeline (StreamDiffusion-based)                 |
+-----------------------------------------------------------+

DNS: 34-134-195-88.nip.io -> 34.134.195.88 (wildcard)
```

## Known Limitations

1. **Browser playback jitter**: HTTP long-polling trickle adds ~500ms per segment fetch. Each 2s MPEG-TS segment plays smoothly at 30fps internally, but there's a brief stall between segments.

2. **WHEP not available**: BYOC orchestrator has `//TODO: add WHEP support`. WebRTC playback would eliminate the segment-boundary jitter but requires Go code changes to subscribe to trickle output and create a RingBuffer.

3. **Cloud Run session affinity**: The `MediaPublish` MPEG-TS encoder is stateful. Without session affinity, different Cloud Run instances can't share encoder state, falling back to raw JPEG (which scope-trickle can't decode).

4. **Trickle channel expiry**: Channels expire after 5 minutes of no writes. If scope-trickle is slow to start (cold start ~2min), the channel may expire before processing begins.

5. **fal endpoint params**: fal's `@fal.endpoint` only parses query string parameters, not JSON body fields. The adapter must URL-encode trickle URLs in query params.
