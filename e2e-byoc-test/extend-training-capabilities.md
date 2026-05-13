# Extending Training Capabilities to New Providers

## Overview

The BYOC training pipeline is **provider-agnostic** by design. Only one layer — the serverless proxy — speaks provider-specific APIs. Everything else (orchestrator, adapter, SDK, CLI) is generic.

```
SDK/CLI  ->  Orchestrator  ->  Adapter  ->  Proxy  ->  Provider
(generic)    (generic)        (generic)    (provider-  (fal.ai,
                                            specific)  RunPod,
                                                       Replicate,
                                                       etc.)
```

Adding a new provider requires:
- **~80-120 lines** of Python in the proxy (one new provider file)
- **0 lines** changed in orchestrator, adapter, SDK, or CLI
- **1 curl command** to register the capability at runtime

---

## Architecture: What Each Layer Does

| Layer | Role | Provider-specific? |
|-------|------|--------------------|
| **Orchestrator** (Go) | Auth, routing, metering (charges per second), job tracking | No |
| **Adapter** (Python) | Submits to proxy `/train/submit`, polls `/train/status/{id}`, tracks jobs | No |
| **Proxy** (Python) | Translates generic `/train/submit` into provider-specific API calls | **Yes** |
| **SDK/CLI** (Python) | Builds request, polls orchestrator, displays results | No |

The proxy's `InferenceProvider` base class defines the contract:

```python
class InferenceProvider(ABC):
    async def train_submit(self, request_body, session) -> dict:
        """Submit async training job. Returns {request_id, model_id, ...}."""

    async def train_status(self, request_id, model_id, session) -> dict:
        """Poll training status. Returns {status, ...} or full result on completion."""
```

Every provider just implements these two methods with provider-specific HTTP calls.

---

## Step-by-Step: Adding RunPod as a Training Provider

### Step 1: Create the Provider File

Create `livepeer-serverless-proxy/src/serverless_proxy/providers/runpod.py`:

```python
"""RunPod serverless training provider."""

from __future__ import annotations

import logging
import aiohttp
from .base import InferenceProvider

logger = logging.getLogger(__name__)


class RunPodProvider(InferenceProvider):
    """Provider that forwards inference and training to RunPod serverless endpoints."""

    def __init__(self, api_key: str, model_id: str) -> None:
        self._api_key = api_key
        self._default_endpoint_id = model_id  # RunPod endpoint ID

    @property
    def _auth_headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self._api_key}",
            "Content-Type": "application/json",
        }

    async def health(self) -> bool:
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(
                    "https://api.runpod.ai/v2/health",
                    headers=self._auth_headers,
                    timeout=aiohttp.ClientTimeout(total=5),
                ) as resp:
                    return resp.status in (200, 401)  # 401 = reachable but needs auth
        except Exception:
            return False

    async def inference(self, request_body: dict, session: aiohttp.ClientSession) -> dict:
        """Synchronous inference via RunPod /runsync."""
        endpoint_id = request_body.pop("endpoint_id", None) or self._default_endpoint_id
        url = f"https://api.runpod.ai/v2/{endpoint_id}/runsync"

        payload = {"input": request_body}
        async with session.post(url, json=payload, headers=self._auth_headers) as resp:
            if resp.status != 200:
                body = await resp.text()
                return {"error": f"RunPod returned {resp.status}", "detail": body}
            return await resp.json()

    async def train_submit(self, request_body: dict, session: aiohttp.ClientSession) -> dict:
        """Submit async training job to RunPod /run endpoint."""
        # model_id here maps to RunPod endpoint_id
        endpoint_id = request_body.pop("model_id", None) or self._default_endpoint_id
        url = f"https://api.runpod.ai/v2/{endpoint_id}/run"

        # RunPod expects {"input": {...}} wrapper
        payload = {"input": request_body}

        logger.info("Submitting RunPod training to %s", url)
        async with session.post(url, json=payload, headers=self._auth_headers) as resp:
            if resp.status not in (200, 201, 202):
                body = await resp.text()
                return {"error": f"RunPod returned {resp.status}", "detail": body}
            result = await resp.json()

        # Normalize response to match adapter's expected format
        return {
            "request_id": result.get("id"),
            "status": result.get("status", "IN_QUEUE"),
            "model_id": endpoint_id,
        }

    async def train_status(self, request_id: str, model_id: str,
                           session: aiohttp.ClientSession) -> dict:
        """Check RunPod job status."""
        endpoint_id = model_id or self._default_endpoint_id
        url = f"https://api.runpod.ai/v2/{endpoint_id}/status/{request_id}"

        async with session.get(url, headers=self._auth_headers) as resp:
            data = await resp.json()

        runpod_status = data.get("status", "UNKNOWN")

        # Map RunPod statuses to our normalized statuses
        status_map = {
            "IN_QUEUE": "IN_QUEUE",
            "IN_PROGRESS": "IN_PROGRESS",
            "COMPLETED": "COMPLETED",
            "FAILED": "FAILED",
            "TIMED_OUT": "FAILED",
            "CANCELLED": "CANCELLED",
        }
        data["status"] = status_map.get(runpod_status, runpod_status)

        # On completion, promote "output" to top level as the result
        if data["status"] == "COMPLETED" and "output" in data:
            result = data["output"]
            if isinstance(result, dict):
                result["status"] = "COMPLETED"
                result["_training_meta"] = {
                    "request_id": request_id,
                    "model_id": model_id,
                    "execution_time": data.get("executionTime"),
                }
                return result

        return data
```

### Step 2: Register the Provider in the Proxy Factory

Edit `livepeer-serverless-proxy/src/serverless_proxy/providers/__init__.py` (or wherever providers are selected):

```python
from .runpod import RunPodProvider

# In the factory/selection logic:
if provider_name == "runpod":
    return RunPodProvider(api_key=api_key, model_id=model_id)
```

### Step 3: Deploy a Proxy Instance for RunPod

```yaml
# docker-compose addition or separate deployment
runpod-proxy:
  build:
    context: ./livepeer-serverless-proxy
  environment:
    PROVIDER: runpod
    API_KEY: ${RUNPOD_API_KEY}
    MODEL_ID: "your-runpod-endpoint-id"
    PORT: "8081"
```

Or add to existing proxy with multi-provider support.

### Step 4: Register the Capability

```bash
curl -X POST http://ADAPTER_URL:9090/capabilities \
  -H "Content-Type: application/json" \
  -d '{
    "name": "runpod-lora-training",
    "model_id": "your-runpod-endpoint-id",
    "capacity": 3
  }'
```

### Step 5: Use It via SDK

```python
from livepeer_gateway import ByocTrainingRequest, submit_training_job, wait_for_training

req = ByocTrainingRequest(
    capability="runpod-lora-training",
    model_id="your-runpod-endpoint-id",
    params={
        "images_data_url": "https://your-dataset.zip",
        "trigger_word": "mystyle",
        "steps": 100,
    },
)

resp = submit_training_job(req, orch_url="https://your-orch:8935")
final = wait_for_training(resp.job_id, resp.orchestrator_url)
print(f"Status: {final.status}, Cost: {final.cost}, Balance: {final.balance}")
```

Or via CLI:

```bash
python3 agent-client-sdk.py train submit \
  --dataset-url "https://your-dataset.zip" \
  --capability runpod-lora-training \
  --model-id your-runpod-endpoint-id \
  --steps 100 --wait
```

---

## Step-by-Step: Adding Replicate as a Training Provider

### Step 1: Create the Provider File

Create `livepeer-serverless-proxy/src/serverless_proxy/providers/replicate.py`:

```python
"""Replicate inference and training provider."""

from __future__ import annotations

import logging
import aiohttp
from .base import InferenceProvider

logger = logging.getLogger(__name__)

REPLICATE_API = "https://api.replicate.com/v1"


class ReplicateProvider(InferenceProvider):
    """Provider that forwards inference and training to Replicate predictions API."""

    def __init__(self, api_key: str, model_id: str) -> None:
        self._api_key = api_key
        self._default_model = model_id  # e.g. "owner/model-name" or "owner/model:version"

    @property
    def _auth_headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self._api_key}",
            "Content-Type": "application/json",
        }

    async def health(self) -> bool:
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(
                    f"{REPLICATE_API}/models",
                    headers=self._auth_headers,
                    timeout=aiohttp.ClientTimeout(total=5),
                ) as resp:
                    return resp.status in (200, 401)
        except Exception:
            return False

    async def inference(self, request_body: dict, session: aiohttp.ClientSession) -> dict:
        """Create a prediction on Replicate."""
        model = request_body.pop("model_id", None) or self._default_model
        url = f"{REPLICATE_API}/predictions"

        payload = {
            "version": model,
            "input": request_body,
        }

        async with session.post(url, json=payload, headers=self._auth_headers) as resp:
            if resp.status not in (200, 201):
                body = await resp.text()
                return {"error": f"Replicate returned {resp.status}", "detail": body}
            return await resp.json()

    async def train_submit(self, request_body: dict, session: aiohttp.ClientSession) -> dict:
        """Submit async training job to Replicate predictions API."""
        model = request_body.pop("model_id", None) or self._default_model
        url = f"{REPLICATE_API}/predictions"

        payload = {
            "version": model,
            "input": request_body,
        }

        logger.info("Submitting Replicate training: model=%s", model)
        async with session.post(url, json=payload, headers=self._auth_headers) as resp:
            if resp.status not in (200, 201):
                body = await resp.text()
                return {"error": f"Replicate returned {resp.status}", "detail": body}
            data = await resp.json()

        # Normalize to adapter's expected format
        return {
            "request_id": data.get("id"),
            "status": data.get("status", "starting"),
            "model_id": model,
            "urls": data.get("urls", {}),
        }

    async def train_status(self, request_id: str, model_id: str,
                           session: aiohttp.ClientSession) -> dict:
        """Check Replicate prediction status."""
        url = f"{REPLICATE_API}/predictions/{request_id}"

        async with session.get(url, headers=self._auth_headers) as resp:
            data = await resp.json()

        replicate_status = data.get("status", "unknown")

        # Map Replicate statuses to our normalized statuses
        status_map = {
            "starting": "IN_QUEUE",
            "processing": "IN_PROGRESS",
            "succeeded": "COMPLETED",
            "failed": "FAILED",
            "canceled": "CANCELLED",
        }
        data["status"] = status_map.get(replicate_status, replicate_status)

        # On completion, include output as result
        if data["status"] == "COMPLETED":
            output = data.get("output")
            if isinstance(output, dict):
                output["status"] = "COMPLETED"
                output["_training_meta"] = {
                    "request_id": request_id,
                    "model_id": model_id,
                    "predict_time": data.get("metrics", {}).get("predict_time"),
                }
                return output
            # If output is a string (URL) or list, wrap it
            return {
                "status": "COMPLETED",
                "output": output,
                "_training_meta": {
                    "request_id": request_id,
                    "model_id": model_id,
                },
            }

        return data
```

### Step 2: Register and Deploy

Same as RunPod — register provider in factory, deploy proxy instance, register capability:

```bash
# Deploy proxy with Replicate config
PROVIDER=replicate API_KEY=$REPLICATE_API_TOKEN MODEL_ID="owner/model:version"

# Register capability
curl -X POST http://ADAPTER_URL:9090/capabilities \
  -H "Content-Type: application/json" \
  -d '{
    "name": "replicate-sdxl-training",
    "model_id": "stability-ai/sdxl:version-hash",
    "capacity": 3
  }'
```

### Step 3: Use It

```bash
python3 agent-client-sdk.py train submit \
  --dataset-url "https://your-dataset.zip" \
  --capability replicate-sdxl-training \
  --model-id "stability-ai/sdxl:version-hash" \
  --steps 100 --wait
```

---

## Provider API Pattern Comparison

All three providers follow the same async pattern — submit, poll, get result:

| Step | fal.ai | RunPod | Replicate |
|------|--------|--------|-----------|
| **Submit** | `POST queue.fal.run/{model}` | `POST api.runpod.ai/v2/{endpoint}/run` | `POST api.replicate.com/v1/predictions` |
| **Auth** | `Authorization: Key {key}` | `Authorization: Bearer {key}` | `Authorization: Bearer {token}` |
| **Request body** | Flat JSON | `{"input": {...}}` | `{"version": "model", "input": {...}}` |
| **Submit response** | `{request_id, status_url}` | `{id, status: "IN_QUEUE"}` | `{id, status: "starting", urls: {...}}` |
| **Poll status** | `GET queue.fal.run/{model}/requests/{id}/status` | `GET api.runpod.ai/v2/{endpoint}/status/{id}` | `GET api.replicate.com/v1/predictions/{id}` |
| **Completed status** | `COMPLETED` | `COMPLETED` | `succeeded` |
| **Failed status** | `FAILED` | `FAILED` / `TIMED_OUT` | `failed` |
| **Get result** | Separate `GET .../requests/{id}` | Included in status response (`output`) | Included in status response (`output`) |
| **Cancel** | N/A | `POST .../cancel/{id}` | `POST .../predictions/{id}/cancel` |

---

## Supporting Other Training Types

### Pre-training / Full Fine-tuning

No code changes needed. Pre-training is just a longer-running job:

```python
req = ByocTrainingRequest(
    capability="runpod-llm-pretrain",
    model_id="your-pretrain-endpoint",
    params={
        "dataset_url": "https://huggingface.co/datasets/your-dataset",
        "epochs": 3,
        "batch_size": 8,
        "learning_rate": 2e-5,
    },
    timeout_seconds=60,  # just for the submit request
)
```

The orchestrator charges per second regardless of job type. A 4-hour pre-training job just gets charged for 4 hours (4 * 3600 = 14400 charge ticks at 30-second intervals).

### RLHF / DPO / Alignment Training

Same pattern — the params just differ:

```python
params={
    "model": "meta-llama/Llama-3-8B",
    "dataset": "https://your-preference-data.jsonl",
    "training_type": "dpo",
    "beta": 0.1,
    "epochs": 1,
}
```

### Distributed Training (Multi-GPU / Multi-Node)

Still the same from the BYOC perspective. The provider (RunPod, etc.) handles multi-GPU scheduling internally. The proxy just submits the job and polls for status.

---

## Checklist: Adding a New Provider

1. [ ] Create `providers/{provider_name}.py` implementing `InferenceProvider`
2. [ ] Implement `train_submit()` — submit job, return `{request_id, model_id}`
3. [ ] Implement `train_status()` — poll status, return normalized `{status, ...}`
4. [ ] Map provider-specific statuses to `COMPLETED` / `FAILED` / `CANCELLED` / `IN_QUEUE` / `IN_PROGRESS`
5. [ ] Register provider in factory/selection logic
6. [ ] Deploy proxy instance with `PROVIDER={name} API_KEY={key}`
7. [ ] Register capability: `POST /capabilities` with name + model_id
8. [ ] Test: `python3 agent-client-sdk.py train submit -d DATASET --capability NAME --wait`

**Total effort**: ~80-120 lines of Python per provider, 0 changes elsewhere.

---

## Multi-Provider Deployment

For running multiple providers simultaneously:

```yaml
# docker-compose example
services:
  proxy-fal:
    image: livepeer-serverless-proxy
    environment:
      PROVIDER: fal-ai
      API_KEY: ${FAL_KEY}
      PORT: "8080"

  proxy-runpod:
    image: livepeer-serverless-proxy
    environment:
      PROVIDER: runpod
      API_KEY: ${RUNPOD_KEY}
      PORT: "8081"

  proxy-replicate:
    image: livepeer-serverless-proxy
    environment:
      PROVIDER: replicate
      API_KEY: ${REPLICATE_TOKEN}
      PORT: "8082"

  adapter:
    image: livepeer-inference-adapter
    environment:
      CAPABILITIES: |
        [
          {"name": "flux-lora-training", "model_id": "fal-ai/flux-lora-fast-training", "capacity": 3},
          {"name": "runpod-lora", "model_id": "your-runpod-endpoint", "capacity": 3},
          {"name": "replicate-sdxl", "model_id": "stability-ai/sdxl:version", "capacity": 3}
        ]
```

Each capability routes to the correct proxy/provider based on the adapter's configuration.
