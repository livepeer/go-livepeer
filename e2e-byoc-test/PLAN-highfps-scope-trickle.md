# Plan: 20fps V2V Streaming via scope-trickle

## Goal
25fps ingest → 20fps output via scope-trickle (longlive on fal H100), removing adapter and SDK from the media path.

## Current State (verified 2026-03-18)

### What Works
- **Batch inference**: SDK → Orch → Adapter → fal (flux, ltx, etc.) — all 21 capabilities working
- **Stream start/stop**: SDK `/stream/start` → Orch `/ai/stream/start` → Adapter `/stream/start` → returns trickle URLs
- **Trickle publish/subscribe**: 30 frames published at 2fps, all delivered to adapter
- **scope-trickle on fal (H100)**: `/process-frame` endpoint processes frames through longlive pipeline (when warm)
- **scope-trickle `/start`**: Creates sessions, returns instantly with max_multiplexing=10
- **E2E V2V via adapter**: Browser → SDK → Orch → Adapter → fal fast-lcm → trickle output — PASS (2fps, ~0.5s/frame warm)
- **test-scope-trickle.py**: CLI test script that sends continuous frames and reads output

### What's Blocking 20fps
1. **Self-signed TLS on orchestrator** — browser fetch() and fal's livepeer_gateway SDK reject the cert
2. **Adapter as media middleman** — HTTP round-trip per frame to fal (~500ms), max ~2fps
3. **SDK frame proxy** — Cloud Run timeout kills long-polling trickle reads
4. **scope-trickle `/process-frame` uses HTTP per frame** — not in-process GPU pipeline

### Key Files
| File | Location | Purpose |
|------|----------|---------|
| Storyboard | `e2e-byoc-test/storyboard/index.html` | Browser UI with live streaming panel |
| SDK Service | `e2e-byoc-test/sdk-service/app.py` | Cloud Run, handles /stream/start, /stream/{id}/frame proxy |
| Stream Adapter | `NaaP/containers/livepeer-stream-adapter/src/livepeer_stream_adapter/stream_server.py` | VM Docker, bridges trickle↔fal |
| scope-trickle fal app | `livepeer/scope-trickle/src/scope/cloud/fal_livepeer_app.py` | fal H100, scope pipeline |
| scope frame endpoint | `livepeer/scope-trickle/src/scope/server/app.py` | Added `/api/v1/frame/process` |
| Test script | `e2e-byoc-test/test-scope-trickle.py` | CLI E2E test, sends frames, reads output |
| Deploy script | `e2e-byoc-test/deploy-stream-adapter.sh` | VM deployment |

### Infrastructure
- **VM**: `34.134.195.88` (GCE `byoc-node`, `us-central1-a`)
- **SSH**: `gcloud compute ssh byoc-node --zone us-central1-a`
- **Docker containers**: byoc_orch (:8935/:7935), byoc_adapter (:9090), byoc_stream_adapter (:9093), byoc_proxy (:8080)
- **Orch config**: `-orchestrator -network offchain -orchSecret offchain-test-secret-2026 -liveAITrickleHostForRunner byoc_orch:8935`
- **scope-trickle fal**: `https://fal.run/Daydream/scope-trickle/` (health, start, process-frame, load-pipeline, debug endpoints)
- **SDK Service**: `https://livepeer-sdk-service-90265565772.us-central1.run.app`
- **fal admin key**: `513f7292-e6a3-40b4-836f-32a644919d6f:c055899e3c22b36a8309932d83f7163d`
- **fal model key** (used by adapter): `d26bca42-b78f-4933-aa85-7ca8196753e4:280a286603355c3a6a608085c652b754`

## Target Architecture

```
SETUP (once, via SDK):
  Browser → SDK /stream/start → Orch /ai/stream/start → creates trickle channels
  Orch → Adapter /stream/start → scope-trickle /start (passes public trickle URLs)
  scope-trickle runs livepeer_app.py media loops in-process (FrameProcessor on GPU)

MEDIA (continuous, direct trickle — no adapter, no SDK):
  Browser ──HTTPS──► Orch trickle pub ──relay──► scope-trickle (MediaOutput.frames())
                                                        │
                                                 FrameProcessor (longlive, H100 GPU, 20-30fps)
                                                        │
  Browser ◄──HTTPS── Orch trickle sub ◄──relay── scope-trickle (MediaPublish.write_frame())

CONTROL (via trickle control channel):
  Browser → Orch trickle control → scope-trickle (JSONLReader → update_parameters)
```

## Implementation Steps

### Phase 1: Valid TLS on Orchestrator

**Step 1.1**: Install nginx + Let's Encrypt on VM
- Install nginx, certbot
- Need a domain name pointing to 34.134.195.88 (or use Cloudflare tunnel)
- If no domain available: use `nip.io` wildcard DNS: `34-134-195-88.nip.io`
- nginx config: proxy_pass to localhost:8935 (orch), with SSL termination
- Also proxy WebSocket upgrade for trickle long-polling
- **Verify**: `curl https://34-134-195-88.nip.io/` returns orch response
- **Verify**: browser `fetch("https://34-134-195-88.nip.io/ai/trickle/test/-1")` works (no cert error)

**Step 1.2**: Update trickle URL rewriting
- SDK Service's `/stream/start` returns trickle URLs with public HTTPS hostname
- Update `_build_stream_livepeer_header` and URL rewriting to use new hostname
- Update orch `-liveAITrickleHostForRunner` to use the public hostname for scope-trickle
- **Verify**: SDK `/stream/start` returns URLs browser can access

**Step 1.3**: Browser trickle directly
- Update storyboard to pub/sub trickle using the public HTTPS URLs (not SDK proxy)
- Remove SDK `/stream/{id}/frame` proxy calls
- **Verify**: browser can publish JPEG and read output without cert errors

### Phase 2: scope-trickle Connects to Orch Trickle Directly

**Step 2.1**: Verify scope-trickle → orch connectivity
- From fal container, test HTTP GET to orch public URL
- **Verify**: `curl https://34-134-195-88.nip.io/ai/trickle/test/-1` from scope-trickle

**Step 2.2**: scope-trickle runs in-process media loops
- Update `/start` endpoint to launch media loops using livepeer_gateway SDK:
  - `MediaOutput(subscribe_url)` → reads input frames
  - `FrameProcessor.put(frame)` → GPU pipeline
  - `FrameProcessor.get()` → output tensor
  - `MediaPublish.write_frame(output)` → writes output frames
- Run in background thread (same pattern as current `/start`)
- Key: FrameProcessor is persistent per session, not created per frame
- **Verify**: push frames from CLI to orch trickle, read output from orch trickle (no adapter involved)

**Step 2.3**: Adapter lifecycle-only
- Adapter's `/stream/start` calls scope-trickle `/start` with public trickle URLs
- Adapter does NOT run `_fal_input_output_loop` when scope-trickle is used
- Adapter only manages capability registration and lifecycle (start/stop)
- **Verify**: stream start → adapter calls scope-trickle → scope-trickle handles media → output appears on trickle

### Phase 3: End-to-End at 25fps

**Step 3.1**: Storyboard publishes at 25fps
- Webcam capture at 25fps → JPEG → trickle publish (direct HTTPS to orch)
- Reduce JPEG quality for smaller frames (quality=60, ~15-20KB per frame)
- **Verify**: 25 frames/sec appear in adapter logs (or scope-trickle logs)

**Step 3.2**: scope-trickle outputs at 20fps
- FrameProcessor runs longlive pipeline on H100
- Output frames published to trickle at pipeline processing rate
- **Verify**: output trickle has 20+ frames per second

**Step 3.3**: Browser renders output
- Storyboard subscribes to trickle output (direct HTTPS)
- Renders JPEG frames to canvas
- Displays FPS counter
- **Verify**: live video playback in browser at 20fps

**Step 3.4**: Prompt updates
- Browser sends prompt updates to trickle control channel (direct HTTPS)
- scope-trickle reads control, updates FrameProcessor params
- **Verify**: changing prompt changes output in real-time

### Phase 4: Polish
- Cold start handling (loading indicator)
- Stream stop/cleanup
- Error recovery
- Multiple concurrent streams

## Verification Protocol

Each step has explicit pass/fail criteria. Use this log format at every boundary:

```
[BROWSER]  pub frame #{seq} ({size}B) → {url}
[ORCH]     trickle relay #{seq}
[SCOPE]    recv frame #{seq} ({size}B), processing...
[SCOPE]    output #{seq} ({size}B), publishing...
[ORCH]     trickle relay output #{seq}
[BROWSER]  recv output #{seq} ({size}B), {fps}fps
```

Test scripts:
- `test-scope-trickle.py` — CLI test for full pipeline
- Direct `curl` commands for each step in isolation
- Browser console for frontend verification

## Notes
- scope-trickle fal app needs `max_multiplexing = 10` (set in code, resets on deploy)
- longlive pipeline needs ~30s warm-up on cold start (first few frames may timeout)
- fal `keep_alive=300` keeps runner warm for 5 min between requests
- Orch uses `-liveAITrickleHostForRunner` to set trickle URLs sent to adapter/runner
- nip.io DNS: `34-134-195-88.nip.io` resolves to `34.134.195.88` (free, no setup)
