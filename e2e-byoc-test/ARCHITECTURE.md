# BYOC E2E Architecture & Design Decisions

> Living design document capturing the architecture, prototyping lessons, and key decisions made during the BYOC (Bring Your Own Capability) end-to-end testing system.

## Table of Contents

1. [System Overview](#system-overview)
2. [Request Flow](#request-flow)
3. [Component Architecture](#component-architecture)
4. [Scope-Live: WebRTC vs HTTP (Design Evolution)](#scope-live-webrtc-vs-http)
5. [Multi-Provider Routing](#multi-provider-routing)
6. [Dependency Chaining (Image-to-Video)](#dependency-chaining)
7. [Storyboard Architecture](#storyboard-architecture)
8. [MCP Integration](#mcp-integration)
9. [Deployment Architecture](#deployment-architecture)
10. [Lessons Learned](#lessons-learned)

---

## 1. System Overview <a name="system-overview"></a>

The BYOC system allows external AI providers (fal.ai, Replicate, RunPod, Google Gemini) to serve inference through the Livepeer orchestrator network. The key insight: the orchestrator doesn't run AI models — it routes requests to registered "adapters" that proxy to external providers.

```
                                          ┌─────────────────┐
                                     ┌───>│ Proxy (fal.ai)  │───> fal.ai API
                                     │    └─────────────────┘
┌──────────┐   ┌──────────────┐   ┌──┴───────┐
│  Client   │──>│ SDK Service  │──>│ Orchestr. │
│(Storyboard│   │ (Cloud Run)  │   │ (:8935)  │
│  MCP, CLI)│   └──────────────┘   └──┬───────┘
└──────────┘                          │    ┌─────────────────┐
                                      ├───>│ Proxy (Replicate)│──> Replicate API
                                      │    └─────────────────┘
                                      │    ┌─────────────────┐
                                      └───>│ Proxy (RunPod)   │──> RunPod API
                                           └─────────────────┘
```

**Key properties:**
- **Offchain mode** — no Ethereum, no staking, no gas fees. Self-signed TLS on orchestrator.
- **Capability-based routing** — each adapter registers capabilities (e.g., "recraft-v4", "ltx-i2v") with the orchestrator. Clients request by capability name.
- **Provider-agnostic** — the same orchestrator routes to fal.ai, Replicate, RunPod, or custom backends. Clients don't know or care which provider serves their request.

---

## 2. Request Flow <a name="request-flow"></a>

### Inference (e.g., generate an image)

```
1. Client POST /inference {capability: "recraft-v4", prompt: "a dragon"}
      │
2. SDK Service builds ByocJobRequest, adds Livepeer header (base64-encoded JSON)
      │
3. POST https://orch:8935/process/request/recraft-v4
      │ Header: Livepeer: base64({request: '{"prompt":"..."}', capability: "recraft-v4"})
      │ Body: {"prompt": "a dragon", "num_images": 1}
      │
4. Orchestrator looks up "recraft-v4" → registered at http://adapter:9090
      │
5. Orchestrator forwards body AS-IS to http://adapter:9090/inference/recraft-v4
      │
6. Adapter resolves recraft-v4 → model_id "fal-ai/recraft/v4/pro/text-to-image"
      │
7. Adapter forwards to http://proxy:8080/inference/fal-ai/recraft/v4/pro/text-to-image
      │
8. Proxy calls https://fal.run/fal-ai/recraft/v4/pro/text-to-image
      │
9. fal.ai returns {"images": [{"url": "https://..."}]}
      │
10. Response bubbles back through the stack unchanged
```

### The Livepeer Header

The orchestrator requires a base64-encoded JSON header for job routing:

```python
header = base64.b64encode(json.dumps({
    "request": json.dumps({"prompt": "..."}),  # nested JSON string
    "capability": "recraft-v4",
    "timeout_seconds": 300,
}).encode()).decode()
# Sent as: Livepeer: eyJyZXF1ZXN0Ijoi...
```

**Why nested JSON string?** The `request` field is a JSON string, not an object. This is a Livepeer protocol convention — the orchestrator treats it opaquely and passes it through.

### Registration (Adapter → Orchestrator)

Adapters self-register every 15 seconds:

```
POST https://orch:8935/capability/register
Authorization: {orch_secret}
{
    "name": "recraft-v4",
    "model_id": "fal-ai/recraft/v4/pro/text-to-image",
    "capacity": 5,
    "url": "http://adapter:9090/inference"
}
```

The orchestrator stores capabilities in memory. If the adapter stops heartbeating, the capability eventually becomes unavailable.

---

## 3. Component Architecture <a name="component-architecture"></a>

### Inference Adapter (`livepeer-inference-adapter`)

**Purpose:** Registers capabilities with the orchestrator and proxies inference requests to the backend (serverless proxy).

```
Key files:
  src/livepeer_adapter/
    main.py          # Async entrypoint, startup sequence
    registrar.py     # Capability registration + heartbeat
    proxy.py         # HTTP server: /inference, /capabilities, /health
    config.py        # Multi-capability config from CAPABILITIES env var
    health.py        # Backend health monitoring, auto-unregister on failure
```

**Startup sequence:**
1. Start HTTP server on ADAPTER_PORT
2. Wait for backend proxy to become healthy (up to 300s)
3. Register all capabilities with orchestrator (retry 30x with 5s intervals)
4. Start health monitor (polls backend every 15s)
5. Start registration heartbeat (re-registers every 15s)

**Health-driven registration:** If the backend becomes unhealthy, the adapter automatically unregisters from the orchestrator. When it recovers, it re-registers. This prevents the orchestrator from routing to a dead backend.

### Serverless Proxy (`livepeer-serverless-proxy`)

**Purpose:** Translates generic inference requests into provider-specific API calls.

```
Key files:
  src/serverless_proxy/
    server.py              # HTTP server, multi-provider routing
    config.py              # Provider config from env vars
    providers/
      base.py              # Abstract InferenceProvider
      fal_ai.py            # fal.ai: sync + queue fallback
      gemini.py            # Gemini: generateContent, predict, predictLongRunning
      replicate.py         # Replicate: predictions API + polling
      runpod.py            # RunPod: run + status polling
      custom.py            # Generic HTTP passthrough
```

**Multi-provider routing** (see Section 5 for details):
```python
# Model ID prefix determines provider:
# "gemini/gemini-2.5-flash" → GeminiProvider, model="gemini-2.5-flash"
# "fal-ai/flux/schnell"     → FalAiProvider (default)
```

### SDK Service (`sdk-service/app.py`)

**Purpose:** FastAPI REST wrapper around the `livepeer-gateway` Python SDK. Runs on Cloud Run so browser clients can call it directly (with CORS).

**Key endpoints:**
| Endpoint | Purpose |
|----------|---------|
| `GET /capabilities` | Aggregates from all adapters (:9090, :9091, :9092) |
| `POST /inference` | Routes to orchestrator via SDK |
| `POST /enrich` | LLM-powered prompt enrichment via Gemini |
| `POST /train` | LoRA training submission |
| `POST /stream/start` | Live streaming control plane |

**Why a separate service?** Browsers can't call the orchestrator directly (self-signed cert rejection, no CORS). The SDK service handles SSL, CORS, error normalization, and multi-adapter aggregation.

### Stream Adapter (`livepeer-stream-adapter`)

**Purpose:** Handles live video streaming through Scope (fal.ai's real-time video pipeline). See Section 4 for the full design evolution.

```
Key files:
  src/livepeer_stream_adapter/
    stream_server.py    # Main server: /inference, /stream/start, /stream/stop
    scope_bridge.py     # WebRTC + WebSocket bridge (legacy)
    trickle_client.py   # Trickle protocol: HTTP-based segment streaming
    frame_codec.py      # MPEG-TS ↔ raw frames, JPEG utilities
```

---

## 4. Scope-Live: WebRTC vs HTTP (Design Evolution) <a name="scope-live-webrtc-vs-http"></a>

This section documents the most significant prototyping journey — building live video processing through Daydream Scope on fal.ai.

### The Goal

Allow users to send video frames (from camera, uploaded image, or storyboard) through Scope's real-time AI video pipeline, which applies style transfer, enhancement, or creative effects to each frame.

### Attempt 1: WebRTC (Failed)

**Architecture:**
```
Client → Stream Adapter → Scope (fal.ai)
                           ↕ WebSocket (signaling)
                           ↕ WebRTC (video tracks)
```

**Implementation (`scope_bridge.py`):**

```python
class ScopeBridge:
    async def connect(self):
        # 1. WebSocket to fal.ai for signaling
        ws_url = f"wss://daydream-scope-app.gateway.alpha.fal.ai/ws"
        self._ws = await websockets.connect(ws_url, additional_headers=headers)

        # 2. Wait for "ready" message with connection_id
        await asyncio.wait_for(self._ready_event.wait(), timeout=10.0)

        # 3. Create WebRTC peer connection
        self._pc = RTCPeerConnection()
        self._input_track = InputVideoTrack()  # our frames → Scope
        self._pc.addTrack(self._input_track)

        # 4. Listen for output video track (Scope → us)
        @self._pc.on("track")
        def on_track(track):
            asyncio.ensure_future(self._receive_output(track))

        # 5. SDP offer/answer handshake
        offer = await self._pc.createOffer()
        await self._pc.setLocalDescription(offer)
        await self._ws.send(json.dumps({
            "type": "offer",
            "sdp": self._pc.localDescription.sdp,
            "connection_id": self._connection_id,
            "params": self._config.initial_params,
        }))

        # 6. Wait for SDP answer from Scope
        for _ in range(100):  # 10s timeout
            await asyncio.sleep(0.1)
            if self._pc.remoteDescription:
                break
```

**What went wrong:**

1. **`websockets` API change:** v16.0 renamed `extra_headers` → `additional_headers`. Silent failure — connection opened but auth wasn't sent.

2. **WebSocket URL construction bug:** Used `self._config.fal_endpoint` (`daydream/scope-app` with `/`) instead of the `-`-separated version, producing `wss://daydream/scope-app.gateway...` (invalid hostname).

3. **Missing ready-wait:** Initially sent SDP offer immediately after WebSocket connect. Scope needs to send a `{type: "ready", connection_id: "..."}` message first. Without waiting, the offer was ignored.

4. **SDP answer never received:** Even after fixing all the above, Scope acknowledged the WebSocket connection and sent `ready`, but never responded to the SDP offer with an SDP answer. The WebRTC handshake timed out every time.

   **Root cause hypothesis:** aiortc (Python WebRTC) generates SDP offers that may be incompatible with fal.ai's Scope signaling server. Scope may expect browser-style WebRTC (with specific codecs, ICE candidates, or DTLS parameters that aiortc doesn't produce).

5. **Frame queue complexity:** Even if WebRTC worked, the `InputVideoTrack` used a small queue (maxsize=2) with frame dropping — acceptable for real-time but complex to debug:

```python
async def send_frame(self, rgb_array):
    try:
        self._queue.put_nowait(rgb_array)
    except asyncio.QueueFull:
        self._queue.get_nowait()  # drop oldest
        self._queue.put_nowait(rgb_array)
```

### Attempt 2: HTTP API (Succeeded)

**Decision:** Abandon WebRTC for single-frame inference. Use fal.ai's synchronous HTTP API (`fal-ai/fast-lcm-diffusion`) for frame-by-frame processing.

**Architecture:**
```
Client → Stream Adapter → fal.ai HTTP API (stateless)
         POST /inference    POST https://fal.run/fal-ai/fast-lcm-diffusion
         {image, prompt}    {image_url: "data:...", prompt, strength, sync_mode: true}
```

**Implementation (`stream_server.py:_handle_inference`):**

```python
async def _handle_inference(self, request):
    body = await request.json()
    prompt = body.get("prompt", "enhance the image")
    strength = float(body.get("strength", 0.5))

    # Accept base64 image or URL
    image_url = body.get("image_url", "")
    if image_url.startswith("data:"):
        _, encoded = image_url.split(",", 1)
        jpeg_bytes = base64.b64decode(encoded)
    else:
        # Fetch image from URL
        async with session.get(image_url) as resp:
            jpeg_bytes = await resp.read()

    # Convert to base64 for fal.ai
    img_b64 = base64.b64encode(jpeg_bytes).decode()

    # Call fal.ai HTTP API (stateless, synchronous)
    fal_payload = {
        "prompt": prompt,
        "image_url": f"data:image/jpeg;base64,{img_b64}",
        "strength": strength,
        "num_inference_steps": 4,    # LCM = fast with few steps
        "guidance_scale": 1.0,
        "sync_mode": True,           # wait for result
    }

    async with session.post(fal_url, json=fal_payload, headers=auth) as resp:
        result = await resp.json()
        return web.json_response(result)
```

### Why HTTP Won

| Factor | WebRTC | HTTP |
|--------|--------|------|
| **Complexity** | WebSocket signaling + SDP + ICE + tracks | Simple POST/response |
| **State** | Persistent connection, session management | Stateless per request |
| **Debugging** | Opaque SDP negotiation failures | Clear HTTP status codes |
| **Latency** | ~30ms per frame (if it worked) | ~200-500ms per frame |
| **Reliability** | Single point of failure (connection drop) | Each request independent |
| **Server-side WebRTC** | aiortc has limited codec support | Any HTTP client works |

**The trade-off:** HTTP is slower per frame (~200-500ms vs ~30ms for WebRTC) but far simpler and more reliable. For the prototyping use case, reliability and debuggability won over raw latency.

### Trickle Protocol (Live Streaming Path)

For actual live streaming (not single-frame inference), the stream adapter uses the **Trickle protocol** — HTTP-based segment streaming:

```python
# Subscriber: receive video segments
async with TrickleSubscriber(subscribe_url) as sub:
    async for segment in sub:
        frames = decode_mpeg_ts(segment.data)   # MPEG-TS → raw frames
        for frame in frames:
            processed = await scope.process(frame)
        out_data = encode_mpeg_ts(output_frames)  # frames → MPEG-TS
        await publisher.write(out_data)

# Publisher: send processed segments
# Uses POST {url}/{seq} with Content-Type: video/MP2T
```

**Frame codec details:**
- Decode: `av.open(BytesIO(segment), format="mpegts")` → RGB numpy arrays
- Encode: `av.open(buf, mode="w", format="mpegts")` with `preset=ultrafast, tune=zerolatency`

### What Would I Do Differently

1. **Start with HTTP, not WebRTC.** WebRTC is the right choice for browser-to-browser real-time, but for server-to-server processing, HTTP is almost always simpler.
2. **Test WebSocket signaling in isolation** before adding WebRTC on top. The URL bug and auth issue were masked by the SDP failure.
3. **Use the fal.ai queue API** for longer operations instead of sync mode. The queue API (`queue.fal.run`) handles timeouts more gracefully.

---

## 5. Multi-Provider Routing <a name="multi-provider-routing"></a>

The serverless proxy supports multiple AI providers through prefix-based routing:

```python
# In docker-compose, the primary proxy is fal-ai with Gemini as extra:
PROVIDER: fal-ai
API_KEY: ${FAL_KEY}
EXTRA_PROVIDERS: '{"gemini": {"api_key": "${GEMINI_KEY}"}}'
```

**How it works:**

```python
# server.py — route by model_id prefix
for prefix, prov in self._extra_providers.items():
    if model_id.startswith(f"{prefix}/"):
        provider = prov
        model_id = model_id[len(prefix) + 1:]  # strip prefix
        break
```

**Example routing:**
| Capability | model_id | Provider | Stripped model_id |
|------------|----------|----------|-------------------|
| gemini-image | `gemini/gemini-2.5-flash-image` | GeminiProvider | `gemini-2.5-flash-image` |
| gemini-text | `gemini/gemini-2.0-flash` | GeminiProvider | `gemini-2.0-flash` |
| recraft-v4 | `fal-ai/recraft/v4/pro/text-to-image` | FalAiProvider (default) | same |
| flux-schnell | `fal-ai/flux/schnell` | FalAiProvider | same |

### Provider-Specific Behaviors

**fal.ai:** Two-stage strategy — try sync endpoint (`fal.run`) first, fall back to queue (`queue.fal.run`) with polling every 2s, timeout 300s.

**Gemini:** Routes by model name:
- `"image" in model_id` → `generateContent` with `responseModalities: ["TEXT", "IMAGE"]`
- `"veo" in model_id` → `predictLongRunning` with polling
- Default → `generateContent` for text responses

**Replicate/RunPod:** Always use async queue with polling.

### Base64 vs URL: A Critical Distinction

Different models return results in different formats:

| Model | Returns | Can chain to video? |
|-------|---------|-------------------|
| recraft-v4, flux-*, nano-banana, qwen-image | URL (`https://...`) | Yes |
| gemini-image | Base64 (inline bytes) | No — video models need URLs |
| fast-lcm | Base64 | No |
| lucy-i2v | Base64 video | Needs special handling |

**This caused real bugs:** When the LLM enrichment system chose gemini-image for an image→video chain, the video model received a base64 data URI and failed. Fixed by:
1. Adding explicit rules to the enrichment prompt
2. Storing `_resolvedImageUrl` on results for dependency resolution

---

## 6. Dependency Chaining (Image-to-Video) <a name="dependency-chaining"></a>

The storyboard supports multi-step workflows where one output feeds the next:

```
[recraft-v4: "dragon"] ──image_url──> [ltx-i2v: "animate the dragon"]
       ↓                                        ↓
   Image card                              Video card
```

### How It Works

1. **LLM enrichment** returns steps with `depends_on` field:
```json
[
  {"id": "dragon_01", "capability": "recraft-v4", "prompt": "..."},
  {"id": "dragon_vid", "capability": "ltx-i2v", "depends_on": "dragon_01", "prompt": "..."}
]
```

2. **ID resolution:** Build `idToIdx` map from step IDs to array indices:
```javascript
const idToIdx = {};
steps.forEach((step, i) => { if (step.id) idToIdx[step.id] = i; });
```

3. **Execution order:** Independent steps run concurrently (`Promise.allSettled`), dependent steps run sequentially after their dependencies complete.

4. **URL injection:** When a dependent step runs, extract the resolved URL from the dependency's result and inject as `params.image_url`:
```javascript
const prev = results[depIdx];
const prevImageUrl = prev._resolvedImageUrl || prev.image_url;
if (prevImageUrl) params.image_url = prevImageUrl;
```

### Bug That Caused Pain

The LLM returned `depends_on` as string IDs (e.g., `"dragon_01"`), but the original code treated them as numeric indices. A string like `"dragon_01"` was used directly as an array index, which returned `undefined`.

**Fix:** Added `idToIdx` map for string-to-index resolution. Also added support for both string IDs and numeric indices for backwards compatibility.

---

## 7. Storyboard Architecture <a name="storyboard-architecture"></a>

A single-page HTML application that combines natural language input with a visual canvas for AI media generation.

### Core Components

```
┌────────────────────────────────────────────────┐
│ Top Bar (zoom controls, card count)            │
├────────────────────────────────────────────────┤
│                                                │
│  ┌──────┐  ──arrow──>  ┌──────┐               │
│  │Image │              │Video │               │
│  │Card  │              │Card  │               │
│  └──────┘              └──────┘               │
│                Canvas (pan/zoom)               │
│                                                │
│                         ┌──────────────────┐   │
│                         │  Chat Panel      │   │
│                         │  (input + msgs)  │   │
│                         └──────────────────┘   │
│  ┌──────────────────┐                          │
│  │ Context Menu     │ (right-click on card)    │
│  │ - Animate        │                          │
│  │ - Restyle        │                          │
│  │ - Upscale        │                          │
│  │ - Custom prompt  │                          │
│  └──────────────────┘                          │
└────────────────────────────────────────────────┘
```

### Enrichment Pipeline

```
User types: "3 pictures of dragons, different styles"
  │
  ├──> SDK Service /enrich (primary)
  │      Uses gemini-text capability
  │      System prompt: creative director with model catalog
  │      Returns: [{id, type, prompt, capability, depends_on, title}]
  │
  ├──> Direct orchestrator gemini-text (fallback)
  │
  └──> Local parser (last resort)
         Regex-based: detects count, type, model hints
         Applies style templates for variety
```

### Card System

Each card is an absolute-positioned div on the canvas:
- **Drag** via header (pointer events)
- **Resize** via bottom-right handle
- **Minimize/Close** buttons
- **Media rendering:** `<img>`, `<video>`, `<audio>` based on type
- **Error state:** Red border, error message
- **Registry:** Global `registry[refId]` stores card metadata and results

### Arrow System

Cubic Bezier curves connect dependent cards:

```javascript
// From right-center of source to left-center of target
const pull = Math.max(60, Math.abs(x2 - x1) * 0.4);
const path = `M ${x1} ${y1} C ${x1+pull} ${y1}, ${x2-pull} ${y2}, ${x2} ${y2}`;
```

Arrows redraw on card move, resize, or delete.

### Context Menu

Right-click on any image card offers:
- **Animate to video** → picks ltx-i2v/lucy-i2v/kling-i2v
- **Restyle** → picks recraft-v4/kontext-edit
- **Upscale** → picks topaz-upscale
- **Custom prompt** → auto-detects image vs video from keywords

Creates a new card, draws an arrow, and runs inference.

---

## 8. MCP Integration <a name="mcp-integration"></a>

Two MCP server variants expose Livepeer capabilities to AI agents (Claude Code, Cursor):

### Direct Orchestrator (`server.py`)
```
Claude Code → MCP → server.py → Orchestrator (direct HTTP)
```
- Constructs Livepeer headers manually
- Handles self-signed SSL
- Has training/LoRA tools
- Best for: debugging, direct access

### SDK Service (`server-sdk.py`)
```
Claude Code → MCP → server-sdk.py → SDK Service (Cloud Run) → Orchestrator
```
- Simple HTTP to Cloud Run (no SSL issues)
- No Livepeer header construction needed
- SDK handles protocol details
- Best for: production use, any-directory access

### MCP Tools Available

| Tool | Description |
|------|-------------|
| `list_capabilities()` | List all registered AI models |
| `generate_image(prompt, model, image_url)` | Text-to-image or image editing |
| `generate_video(prompt, model, image_url)` | Text-to-video or image-to-video |
| `generate_audio(prompt, model)` | TTS or music generation |
| `view_media(file_path)` | View images inline or play video/audio |

### Configuration

**Project-level** (`.mcp.json` in repo root):
```json
{
  "mcpServers": {
    "livepeer-byoc": {
      "command": "python3",
      "args": ["e2e-byoc-test/mcp-server/server-sdk.py"],
      "env": {"SDK_URL": "https://livepeer-sdk-service-....run.app"}
    }
  }
}
```

**User-level** (via `/mcp add` command in Claude Code):
```bash
/mcp add livepeer python3 /path/to/server-sdk.py \
  -e SDK_URL=https://livepeer-sdk-service-....run.app -s user
```

---

## 9. Deployment Architecture <a name="deployment-architecture"></a>

### GCE VM (Docker Compose)

```yaml
services:
  orchestrator:          # go-livepeer, ports 8935 (HTTPS), 7935 (CLI)
  serverless-proxy:      # fal-ai + gemini extra provider, port 8080 internal
  inference-adapter:     # 21 capabilities, port 9090 external
  serverless-proxy-replicate:  # Replicate provider, port 8081 internal
  inference-adapter-replicate: # 10 capabilities, port 9091 external
  serverless-proxy-runpod:     # RunPod provider, port 8082 internal
  inference-adapter-runpod:    # 1 capability, port 9092 external
  stream-adapter:        # Scope live streaming, port 9093 external
```

### SDK Service (Cloud Run)

- **Image:** Built from `sdk-service/Dockerfile` (Python 3.11 + FastAPI + livepeer-gateway SDK)
- **URL:** `https://livepeer-sdk-service-90265565772.us-central1.run.app`
- **Env vars:** `ORCH_URL`, `ADAPTER_URLS` (comma-separated for multi-adapter aggregation)

### 31 Registered Capabilities (across all adapters)

**Adapter :9090 (fal-ai + Gemini):** nano-banana, recraft-v4, gemini-image, gemini-text, flux-schnell, flux-dev, flux-pro, qwen-image, kontext-edit, reve-edit, topaz-upscale, bg-remove, fast-lcm, ltx-t2v, ltx-t2v-23, ltx-i2v, lucy-i2v, wan-v2v, kling-i2v, chatterbox-tts, lux-tts

**Adapter :9091 (Replicate):** seedream-4, z-image-turbo, imagen-4, hunyuan-image-3, gen-4.5, pixverse-v5.6, dreamactor-m2, gpt-5-nano, dall-e-3, claude-opus

**Adapter :9092 (RunPod):** wan-animate

**Adapter :9093 (Scope):** scope-live

---

## 10. Lessons Learned <a name="lessons-learned"></a>

### 1. Start Simple, Add Complexity Later
WebRTC for Scope was the "right" architecture for real-time, but HTTP got us to a working prototype in hours vs days of debugging signaling issues. Ship the simple version first.

### 2. Browser Constraints Drive Architecture
We needed the SDK Service (Cloud Run) because browsers reject self-signed certs and lack CORS on the orchestrator. This intermediate layer also turned out to be useful for enrichment, multi-adapter aggregation, and error normalization.

### 3. Base64 vs URL Is a Semantic Distinction
Not all image results are interchangeable. Models that return base64 (gemini-image, fast-lcm) can't feed into models that expect URLs (ltx-i2v). This required both code fixes and LLM prompt engineering to avoid bad chains.

### 4. TTS Models Are Not Image Models
chatterbox-tts needs `{"text": "..."}` — sending `{"prompt": "...", "text": "..."}` fails because fal.ai validates against unexpected fields. lux-tts needs `{"prompt": "...", "audio_url": "..."}` (reference voice). Each model has its own input schema.

### 5. LLM Enrichment Needs Guardrails
The Gemini-powered enrichment works well but needs explicit rules:
- Exact count matching ("5 pictures" = 5 steps)
- Model catalog with clear descriptions
- Chain-safety rules (which models produce URLs vs base64)
- Fallback to local regex parser when LLM is unavailable

### 6. Self-Signed Certs Are Painful
Every component needed `verify=False` / `CERT_NONE`. The orchestrator generates a self-signed cert on startup. In production, use Let's Encrypt or a proper CA.

### 7. Provider APIs Aren't Uniform
- fal.ai: sync or queue, returns `images[0].url`
- Gemini: generateContent vs predict vs predictLongRunning, returns base64 in `candidates[0].content.parts`
- Replicate: always async, returns `output` (format varies by model)
- RunPod: always async, returns `output` (varies)

The serverless proxy abstracts this, but each provider implementation has ~100 lines of format-specific logic.

### 8. `gcloud --set-env-vars` and Commas
Commas in env var values (like `ADAPTER_URLS=url1,url2`) confuse gcloud's parser. Fix: use `^||^` delimiter syntax:
```bash
gcloud run deploy --set-env-vars "^||^ORCH_URL=...||ADAPTER_URLS=url1,url2"
```

### 9. Docker Compose Networking
Containers reference each other by service name (`byoc_orch`, `byoc_proxy`). External clients use the VM's public IP. The orchestrator's self-signed cert is valid for `0.0.0.0` and `byoc_orch` but not for the public IP — hence `verify=False` everywhere.

### 10. MCP Is the Right Abstraction for AI Tools
MCP (Model Context Protocol) lets any AI agent use Livepeer capabilities without knowing the API details. "Generate an image of a dragon" just works — the agent picks the right tool, model, and parameters.
