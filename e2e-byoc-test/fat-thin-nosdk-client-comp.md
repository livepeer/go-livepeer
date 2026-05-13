# Architecture Comparison: SDK Client vs Thin Client vs No-SDK Client

## Deployed Service

- **SDK Service URL**: `https://livepeer-sdk-service-90265565772.us-central1.run.app`
- **Cloud Run**: us-central1, 512Mi, max 3 instances, 300s timeout
- **Orchestrator**: `https://34.134.195.88:8935`

## Three Client Architectures

```
Fat Client (agent-client-sdk.py):
  Agent [SDK + protocol + SSL + headers] ──────────────────> Orchestrator

Thin Client (agent-client-thin.py):
  Agent [just HTTP POST] ──> SDK Service (Cloud Run) ──────> Orchestrator

No-SDK Client (agent-client.py):
  Agent [raw HTTP + base64 + SSL] ─────────────────────────> Orchestrator
```

## Code Comparison

| Metric | `agent-client-sdk.py` (Fat) | `agent-client-thin.py` (Thin) | `agent-client.py` (No-SDK) |
|---|---|---|---|
| **Lines of code** | 1070 | 348 (**3x less**) | ~600 |
| **External dependencies** | `livepeer-gateway` SDK (+ grpcio, protobuf, av, aiohttp) | **None** (stdlib only) | **None** (stdlib only) |
| **Install required** | `pip install livepeer-gateway` | Nothing | Nothing |
| **Livepeer protocol knowledge** | SDK abstracts it, but must construct `ByocJobRequest`, handle `ByocJobResponse` | **None** — just POST JSON to `/inference` | Full — must build base64 Livepeer headers, handle SSL, construct `/process/request/{cap}` URLs |
| **Orchestrator config** | Must know orchestrator URL, SDK handles certs | Just one env var: `SDK_SERVICE_URL` | Must know orchestrator URL, manually skip SSL verification |
| **Training support** | Builds `ByocTrainingRequest`, calls `submit_training_job()`, manages `TRAINER_PARAM_MAP` | `POST /train` with JSON body | Manual HTTP to `/process/train/{cap}`, manual polling |
| **Error handling** | SDK raises typed exceptions (`LivepeerGatewayError`, `NoOrchestratorAvailableError`) | HTTP status codes from service | Raw HTTP errors, manual parsing |

## What's Simpler in the Thin Client

### 1. Tool functions are 3–5 lines instead of 15–30

```python
# ── Fat client (agent-client-sdk.py) — tool_generate_image ──
def tool_generate_image(prompt, capability="nano-banana"):
    log(f'Generating image: "{prompt}" via [{capability}]', "TOOL")
    result = submit_byoc_job(
        ByocJobRequest(capability=capability, payload={"prompt": prompt, "num_images": 1}),
        orch_url=ORCH,
    )
    if isinstance(result.data, dict) and "error" in result.data:
        log(f"Error: {result.data}", "ERROR")
        return None
    url = result.image_url
    if url:
        log(f"Image URL: {url}", "RESULT")
    return result

# ── Thin client (agent-client-thin.py) — tool_generate_image ──
def tool_generate_image(prompt, capability="nano-banana", **extra):
    log(f'Generating image: "{prompt}" via [{capability}]', "TOOL")
    result = _sdk_post("/inference", {"capability": capability, "prompt": prompt, "params": extra})
    if "error" in result:
        log(f"Error: {result['error']}", "ERROR")
        return None
    url = result.get("image_url")
    if url:
        log(f"Image URL: {url}", "RESULT")
    return result

# ── No-SDK client (agent-client.py) — tool_generate_image ──
def tool_generate_image(prompt, capability="nano-banana"):
    log(f"Generating image: \"{prompt}\" via [{capability}]", "TOOL")
    url = f"{ORCH}/process/request/{capability}"
    body = {"prompt": prompt, "num_images": 1}
    req = urllib.request.Request(url, data=json.dumps(body).encode(), headers={
        "Content-Type": "application/json",
        "Livepeer": livepeer_header(capability),   # base64-encoded header
    })
    try:
        with urllib.request.urlopen(req, timeout=120, context=_ssl_ctx) as r:
            data = json.loads(r.read())
    except urllib.error.HTTPError as e:
        ...  # manual error handling
    # manually extract image URL from nested response structure
    ...
```

### 2. No protocol awareness

The **no-SDK client** must know:
- How to build the `Livepeer` base64-encoded header (`livepeer_header()`)
- The URL pattern: `/process/request/{capability}`
- SSL context setup for self-signed certs
- How to parse nested provider-specific response structures

The **fat client** abstracts some of this via the SDK but still requires:
- Constructing `ByocJobRequest` / `ByocTrainingRequest` dataclasses
- Understanding `ByocJobResponse` properties (`.image_url`, `.video_url`, `.data`)
- Managing SDK installation and version compatibility

The **thin client** needs none of this — just `POST /inference` with a JSON body.

### 3. No `TRAINER_PARAM_MAP`

The fat client has a ~30-line mapping of model-specific parameter names:

```python
# Fat client must know each trainer's field names
TRAINER_PARAM_MAP = {
    "fal-ai/flux-lora-fast-training": {
        "dataset_field": "images_data_url",
        "trigger_field": "trigger_word",
        "steps_field": "steps",
    },
    "fal-ai/flux-lora-portrait-trainer": {
        "dataset_field": "training_data_url",
        "trigger_field": "trigger_phrase",
        "steps_field": "number_of_steps",
    },
    ...
}
```

The thin client just passes params through — the SDK Service handles the mapping.

### 4. No orchestrator discovery

The fat client implements fallback logic across multiple orchestrators. The thin client delegates this entirely to the service.

## What's the Same Across All Three

- Same CLI interface (`--task`, `-i`, `reg ls`, `train submit/status`)
- Same natural language task planning and multi-step chaining
- Same capability resolution (human names → capability IDs)
- Same output format and logging
- Same concurrent execution support for independent steps

## Why the Thin Client Is Simpler

The thin client architecture **pushes all protocol complexity into the service layer**:

| Concern | Fat Client | No-SDK Client | Thin Client |
|---|---|---|---|
| Livepeer headers | SDK builds them | Manual base64 | Service handles |
| SSL / self-signed certs | SDK handles | Manual `ssl.create_default_context()` | Service handles |
| Orchestrator discovery | SDK `_resolve_orchestrators()` | Not supported | Service handles |
| Response parsing | SDK typed responses | Manual JSON parsing | Flat JSON from service |
| Training polling | SDK `wait_for_training()` | Manual loop | `"wait": true` in request |
| Provider param mapping | Client-side `TRAINER_PARAM_MAP` | Client-side mapping | Service handles |
| Error handling | SDK exceptions | Manual HTTP error parsing | HTTP status codes |
| Dependencies | grpcio, protobuf, av, aiohttp | None | None |

## Benefits for Agent Frameworks

### Any language can be a client

Since the thin client only needs HTTP, any agent framework can use Livepeer:

```bash
# curl (shell agent)
curl -X POST https://livepeer-sdk-service-....run.app/inference \
  -H "Content-Type: application/json" \
  -d '{"capability": "recraft-v4", "prompt": "a dragon"}'
```

```javascript
// JavaScript / Node.js agent
const res = await fetch(`${SDK_SERVICE}/inference`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ capability: "recraft-v4", prompt: "a dragon" }),
});
const { image_url } = await res.json();
```

```python
# LangChain tool definition
@tool
def generate_image(prompt: str, capability: str = "recraft-v4") -> str:
    """Generate an image using the Livepeer network."""
    r = requests.post(f"{SDK_SERVICE}/inference",
                      json={"capability": capability, "prompt": prompt})
    return r.json()["image_url"]
```

### SDK upgrades are server-side

When the SDK adds new features (new providers, new capabilities, protocol changes), only the Cloud Run service needs redeployment. No client changes required.

### Credentials stay server-side

The thin client never sees orchestrator URLs, API keys, or SSL certificates. The SDK Service manages all of this, making it safe to distribute thin clients to untrusted environments.

### OpenAPI spec for free

The FastAPI service auto-generates an OpenAPI spec at `/docs`, which can be directly imported into:
- LangChain / CrewAI tool definitions
- GPT Actions (Custom GPTs)
- Postman / Bruno collections
- Any OpenAPI-compatible agent framework

## Test Results

All three clients produce identical results for the same tasks:

```
# Fat client (SDK)
python3 agent-client-sdk.py --task "create a dragon using recraft"
  → IMAGE: https://v3b.fal.media/files/b/.../image.webp  (34s)

# Thin client (SDK Service)
python3 agent-client-thin.py --task "create a dragon using recraft"
  → IMAGE: https://v3b.fal.media/files/b/.../image.webp  (34s)

# No-SDK client (raw HTTP)
ORCH_URL=https://34.134.195.88:8935 python3 agent-client.py --task "create a dragon using recraft"
  → IMAGE: https://v3b.fal.media/files/b/.../image.webp  (33s)
```

The thin client adds ~100–200ms overhead (Cloud Run cold start / network hop) but provides dramatically simpler client code.

## Recommendation

| Use case | Recommended client |
|---|---|
| **Agent frameworks** (LangChain, CrewAI, GPTs) | **Thin client** — simplest integration, OpenAPI spec |
| **Production Python services** | **Fat client (SDK)** — typed responses, no extra network hop |
| **Debugging / low-level testing** | **No-SDK client** — full protocol visibility |
| **Non-Python environments** | **Thin client** — language-agnostic HTTP |
| **Edge / mobile / browser** | **Thin client** — zero dependencies, credentials stay server-side |
