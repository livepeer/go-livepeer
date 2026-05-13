# E2E BYOC Local Test Design

## Goal

Deploy a BYOC orchestrator locally that integrates with NaaP containers
(livepeer-inference-adapter + livepeer-serverless-proxy) to bring remote
serverless AI inference (fal.ai, Scope, or mock) onto the Livepeer network.
Test end-to-end without real LPT staking or ticket payments.

## Architecture

```
+----------------------------------------------------+
|  Test Client (curl / Python script / livepeer-sdk)  |
+----------------------------------------------------+
                        |
                        v  HTTP POST /process/request/{capability}
+----------------------------------------------------+
|  go-livepeer Gateway  (port 9935)                   |
|  - Offchain mode (no ETH, no staking)               |
|  - Discovers orchestrator via -orchAddr flag         |
|  - Handles Livepeer header, payment bypass           |
+----------------------------------------------------+
                        |
                        v  HTTP (internal)
+----------------------------------------------------+
|  go-livepeer Orchestrator  (port 8935)              |
|  - Offchain mode                                    |
|  - BYOC routes: /capability/register,               |
|    /process/request/*, /process/token                |
|  - ExternalCapabilities registry (in-memory)        |
|  - orchSecret: test-secret-123                      |
|  - Price=0, no tickets required                     |
+----------------------------------------------------+
                        |
                        v  HTTP (proxied to registered URL)
+----------------------------------------------------+
|  livepeer-inference-adapter  (port 9090)            |
|  - Registers "test-inference" capability            |
|  - Heartbeat every 10s                              |
|  - Health monitors backend                          |
|  - Proxies /inference requests to backend           |
+----------------------------------------------------+
                        |
                        v  HTTP POST /inference
+----------------------------------------------------+
|  livepeer-serverless-proxy  (port 8080)             |
|  - Provider: custom (mock) or fal-ai                |
|  - Translates to provider API format                |
+----------------------------------------------------+
                        |
                        v
+----------------------------------------------------+
|  Backend: mock-backend / fal.ai / Scope             |
+----------------------------------------------------+
```

## Why This Works Without Real LPT

go-livepeer supports `-network offchain` mode (the default):
- No Ethereum connection (`n.Eth == nil`)
- No block watching, no smart contracts
- `orch.node.Recipient == nil` -> `JobPriceInfo()` returns price=0
- When price=0, `createPayment()` sets `createTickets = false`
- Balance tracking is in-memory only
- No deposit or reserve needed

## Components

### 1. go-livepeer Orchestrator (offchain)
- Flags: `-orchestrator -network offchain -serviceAddr :8935 -orchSecret test-secret-123`
- BYOC server routes auto-registered on the service port
- Accepts capability registration from the adapter
- Proxies `/process/request/*` to the registered capability URL

### 2. go-livepeer Gateway (offchain)
- Flags: `-gateway -network offchain -orchAddr orchestrator:8935 -httpAddr :9935`
- Receives client requests with `Livepeer` header
- Discovers orchestrator via `-orchAddr` (static list in offchain mode)
- Gets job token from orchestrator, creates payment (zero-cost)
- Forwards request to orchestrator's `/process/request/*`

### 3. livepeer-inference-adapter (NaaP)
- On startup: waits for backend health, then registers with orchestrator
- Registration: `POST /capability/register` with orchSecret auth
- Heartbeat: re-registers every 10s
- Proxy: forwards orchestrator requests to serverless-proxy
- Health: monitors backend, unregisters on failure

### 4. livepeer-serverless-proxy (NaaP)
- Default: `PROVIDER=custom` pointing to mock-backend
- fal.ai: `PROVIDER=fal-ai`, `API_KEY`, `MODEL_ID`
- Exposes `/health` and `/inference` endpoints
- Translates to provider-specific API calls

### 5. Mock Backend (for testing)
- Simple Python HTTP server
- `GET /health` -> 200
- `POST /*` -> mock inference response with images/text

## Identified Gaps

### Gap 1: Gateway BYOC Routing in Offchain Mode (MEDIUM)
The gateway's `setupGatewayJob()` calls `getJobOrchestrators()` which probes
orchestrators via `OrchestratorPool.GetInfos()` and then requests job tokens
via HTTP. In offchain mode with `-orchAddr`, the pool is populated from the
static address list. However, the job token request requires the orchestrator
to have the capability registered AND the gateway needs a valid sender identity.

**Mitigation:** In offchain mode, `getJobSender()` may return a dummy sender
since `node.Eth == nil`. If this fails, the direct-to-orchestrator path
(bypassing gateway) still works for testing the NaaP container integration.

### Gap 2: Livepeer Header Format (LOW)
The `Livepeer` header must be a base64-encoded JSON with specific fields:
`request`, `capability`, `timeout_seconds`. The `request` field is itself a
JSON string. Test scripts include correct header construction.

### Gap 3: livepeer-python-gateway SDK Is LV2V-Focused (HIGH)
The `livepeer-python-gateway` SDK (`livepeer-sdk`) is designed for live
video-to-video streaming (WebRTC, trickle channels, H.264), NOT for BYOC
job requests. It uses gRPC for orchestrator discovery and trickle protocol
for data transfer.

**For BYOC job testing, use:**
- `curl` with proper headers (see test-byoc.sh)
- The `test-byoc-client.py` Python script (uses stdlib only)
- Direct HTTP to the adapter (bypasses livepeer payment entirely)

**For future SDK support:** A `BYOCClient` class is planned in Phase 4 of
the NaaP plan but is currently deferred.

### Gap 4: Docker Build Time (LOW)
Building go-livepeer Docker image from source takes significant time (CUDA
dependencies, ffmpeg compilation). Consider using a pre-built image from
Docker Hub (`livepeer/go-livepeer:latest`) instead.

### Gap 5: Scope Integration Requires WebRTC (MEDIUM)
Scope uses WebRTC for real-time video streaming. The BYOC system supports
HTTP POST/response (batch) and trickle streams (live video). For Scope
integration:
- Scope's REST API endpoints (`/api/v1/pipeline/load`, etc.) could be
  wrapped by the serverless-proxy as a `custom` provider
- But Scope's primary value (real-time interactive streaming) requires the
  LV2V pipeline, not BYOC batch jobs
- For testing: use Scope's REST endpoints for pipeline management, or use
  fal.ai as the remote inference provider

### Gap 6: Adapter URL Registration (LOW)
The adapter registers `http://localhost:9090/inference` as its URL. But the
orchestrator is in a Docker container, so it needs the Docker network address
`http://byoc_inference_adapter:9090/inference`. The NaaP adapter handles this
correctly via `ORCH_URL` env var - it registers its own reachable address.

## How to Run

### Quick Start (Mock Backend)
```bash
cd e2e-byoc-test/
./setup.sh           # builds and starts all containers
./test-byoc.sh       # runs E2E tests
./setup.sh down      # tears down
```

### With fal.ai
```bash
FAL_KEY=your-key ./setup.sh fal
CAPABILITY=flux-dev ./test-byoc.sh
./setup.sh down
```

### With Python Client
```bash
python test-byoc-client.py --capability test-inference --test all
python test-byoc-client.py --test adapter   # direct adapter test (always works)
```

## Test Matrix

| Test | What It Validates | Expected Result |
|------|-------------------|-----------------|
| Health checks | All components running | All healthy |
| Direct adapter | adapter -> proxy -> backend | Mock response |
| Job token | Orchestrator BYOC token endpoint | Token or auth error |
| Orch direct | Orchestrator /process/request | Response or payment error |
| Full E2E | Gateway -> Orch -> Adapter -> Backend | Response (may need payment fix) |

## Files

```
e2e-byoc-test/
  docker-compose.yaml    # Full stack: orchestrator, gateway, adapter, proxy, mock
  setup.sh               # Build, start, stop, logs
  test-byoc.sh           # Bash E2E test script
  test-byoc-client.py    # Python E2E test client
  DESIGN.md              # This file
```
