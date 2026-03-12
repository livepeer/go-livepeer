# Livepeer BYOC SDK Guide

## Setup

```bash
# Install the SDK
pip3 install livepeer-gateway

# Or install from source (editable)
cd livepeer-python-gateway
pip3 install -e .
```

Set environment variables for your BYOC deployment:

```bash
export ORCH_URL=https://34.134.195.88:8935    # Orchestrator
export ADAPTER_URL=http://34.134.195.88:9090   # Adapter (direct access)
```

---

## 1. Inference (Image / Video / Music)

### Using `agent-client-sdk.py`

```bash
# Single task
python3 agent-client-sdk.py --task "a dragon as image using recraft"

# Multiple tasks in parallel (separated by ;)
python3 agent-client-sdk.py --task "a cat as image using recraft ; epic music"

# Image -> Video pipeline (chained with 'then')
python3 agent-client-sdk.py --task "a dragon as image using recraft then animate it using lucy"

# Interactive mode
python3 agent-client-sdk.py -i
```

**Available models:**
| Type    | Keywords           | Capability       |
|---------|--------------------|------------------|
| Image   | recraft, nano, gemini | recraft-v4, nano-banana, gemini-image |
| Video   | ltx, lucy, kling, veo | ltx-i2v, lucy-i2v, kling-i2v, veo31-fast |
| Text2Vid| ltx                | ltx-t2v-23       |
| V2V     | wan, sora          | wan-v2v, sora-v2v |
| Music   | beatoven           | beatoven-music   |

### Using SDK directly in Python

```python
from livepeer_gateway import submit_byoc_job, ByocJobRequest

result = submit_byoc_job(
    ByocJobRequest(capability="recraft-v4", payload={"prompt": "a sunset"}),
    orch_url="https://34.134.195.88:8935",
)
print(result.image_url)   # image URL
print(result.balance)     # remaining balance
```

---

## 2. LoRA Training

### Quick Start (CLI)

```bash
# Train with 10 steps (fastest, ~3-4 min) -- direct to adapter
python3 agent-client-sdk.py train submit \
  --dataset-url "https://v3b.fal.media/files/your-dataset.zip" \
  --trigger-word "mystyle" \
  --steps 10 \
  --direct \
  --wait

# Train via orchestrator (full BYOC pipeline)
python3 agent-client-sdk.py train submit \
  --dataset-url "https://v3b.fal.media/files/your-dataset.zip" \
  --trigger-word "mystyle" \
  --steps 100 \
  --wait

# Submit async (returns immediately with job_id)
python3 agent-client-sdk.py train submit \
  --dataset-url "https://v3b.fal.media/files/your-dataset.zip" \
  --trigger-word "mystyle" \
  --steps 100

# Check job status
python3 agent-client-sdk.py train status --job-id YOUR_JOB_ID
```

### CLI Options

```
python3 agent-client-sdk.py train submit [OPTIONS]

Required:
  -d, --dataset-url URL     URL to ZIP file of training images

Optional:
  -t, --trigger-word WORD   Trigger word for the LoRA (default: "lptest")
  -s, --steps N             Training steps (default: 100, use 10 for fast test)
  -m, --model-id ID         fal.ai model (default: fal-ai/flux-lora-fast-training)
  --capability NAME         Capability name (default: flux-lora-training)
  -w, --wait                Wait for completion (blocking, polls until done)
  --direct                  Submit to adapter directly (bypass orchestrator)
  --poll-interval SECS      Poll interval (default: 5.0)
  --timeout SECS            Max wait time (default: 3600)
```

### Using SDK directly in Python

```python
from livepeer_gateway import (
    ByocTrainingRequest,
    submit_training_job,
    get_training_status,
    wait_for_training,
)

# 1. Submit training job
req = ByocTrainingRequest(
    capability="flux-lora-training",
    model_id="fal-ai/flux-lora-fast-training",
    params={
        "images_data_url": "https://v3b.fal.media/files/your-dataset.zip",
        "trigger_word": "mystyle",
        "steps": 100,
        "is_style": False,
        "create_masks": True,
    },
    timeout_seconds=30,
)

resp = submit_training_job(req, orch_url="https://34.134.195.88:8935")
print(f"Job ID: {resp.job_id}")
print(f"Status: {resp.status}")

# 2. Wait for completion (blocking)
final = wait_for_training(
    resp.job_id,
    resp.orchestrator_url,
    poll_interval=5.0,
    timeout=3600.0,
)

print(f"Status: {final.status}")
print(f"LoRA weights: {final.lora_url}")
print(f"Config: {final.config_url}")

# 3. Or poll manually
status = get_training_status(resp.job_id, resp.orchestrator_url)
print(f"Progress: {status.progress}%")
```

### Preparing Training Data

Training requires a ZIP file of images hosted at a publicly accessible URL.

**Requirements:**
- At least 4-5 images (more is better)
- `.jpg` or `.png` format
- Optionally include `.txt` caption files (same name as image)
- ZIP must be downloadable by fal.ai

**Option A: Upload via fal.ai client (recommended)**

```python
# pip install fal-client
import fal_client
import os

os.environ["FAL_KEY"] = "your-fal-api-key"
url = fal_client.upload_file("my-dataset.zip")
print(url)  # https://v3b.fal.media/files/b/xxxx/dataset.zip
```

**Option B: Use any public URL**

Any publicly accessible URL works (Google Cloud Storage, S3, etc.). fal.ai will download the ZIP from this URL.

**ZIP structure:**
```
dataset.zip
├── photo1.jpg
├── photo1.txt          # optional caption: "a photo of [trigger_word]"
├── photo2.jpg
├── photo2.txt
├── photo3.jpg
├── photo4.jpg
└── photo5.jpg
```

---

## 3. Managing Capabilities

```bash
# List all registered capabilities
python3 agent-client-sdk.py reg ls

# Register a new capability (e.g., LoRA training)
python3 agent-client-sdk.py reg add \
  --name flux-lora-training \
  --model-id fal-ai/flux-lora-fast-training \
  --capacity 3

# Remove a capability
python3 agent-client-sdk.py reg rm --name flux-lora-training
```

---

## 4. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     BYOC Pipeline                           │
│                                                             │
│  ┌────────┐    ┌──────────────┐    ┌─────────┐    ┌──────┐ │
│  │ Client │───>│ Orchestrator │───>│ Adapter │───>│Proxy │ │
│  │ (SDK)  │    │  (:8935)     │    │ (:9090) │    │(:8080)│ │
│  └────────┘    └──────────────┘    └─────────┘    └──────┘ │
│       │                                              │      │
│       │  (or --direct)                               │      │
│       └──────────────────────>─────────────────>     │      │
│                                                      ▼      │
│                                                  ┌───────┐  │
│                                                  │fal.ai │  │
│                                                  └───────┘  │
└─────────────────────────────────────────────────────────────┘

Inference:  Synchronous request-response (seconds)
Training:   Async submit -> poll for status (minutes to hours)
```

**Inference flow:** `POST /process/request/{capability}` -> synchronous response with result

**Training flow:**
1. `POST /train` -> returns `{job_id, status: "submitted"}` (202 Accepted)
2. `GET /train/{job_id}` -> returns `{status, progress}` (poll until done)
3. On completion: `{status: "completed", result: {diffusers_lora_file: {url: "..."}}}`

---

## 5. Environment Variables

| Variable      | Default                     | Description               |
|---------------|-----------------------------|---------------------------|
| `ORCH_URL`    | `https://localhost:8935`    | Orchestrator URL          |
| `ADAPTER_URL` | `http://localhost:9090`     | Adapter URL (direct mode) |

---

## 6. Troubleshooting

**"No orchestrator available"**: Check ORCH_URL is correct and orchestrator is running.

**Training timeout**: Increase `--timeout`. LoRA training with 100+ steps can take 5-10 min.

**Dataset download error from fal.ai**: The dataset URL must be publicly accessible. Use `fal_client.upload_file()` to host on fal.ai storage.

**SSL certificate error**: The orchestrator uses self-signed TLS. The SDK auto-skips verification for self-signed certs.

**"capability not found"**: Register the training capability first:
```bash
python3 agent-client-sdk.py reg add -n flux-lora-training -m fal-ai/flux-lora-fast-training
```
