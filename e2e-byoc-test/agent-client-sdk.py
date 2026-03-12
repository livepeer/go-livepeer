#!/usr/bin/env python3
"""
Agent Test Client (SDK version) -- uses livepeer-gateway SDK for inference & LoRA training.

Supports both synchronous inference (image/video/music) and async LoRA training jobs.

Architecture: Agent -> SDK -> Orchestrator -> Adapter -> Proxy -> Provider (fal.ai)

Usage:
    # Inference (image/video/music):
    python3 agent-client-sdk.py --task "create a dragon as image using recraft"
    python3 agent-client-sdk.py -i                 # interactive mode

    # LoRA Training (via orchestrator):
    python3 agent-client-sdk.py train submit -d DATASET_URL --wait --steps 10
    python3 agent-client-sdk.py train status --job-id JOB_ID

    # LoRA Training (direct to adapter, bypasses orchestrator):
    python3 agent-client-sdk.py train submit -d DATASET_URL --direct --steps 10

    # Capabilities:
    python3 agent-client-sdk.py reg ls
    python3 agent-client-sdk.py reg add -n flux-lora-training -m fal-ai/flux-lora-fast-training

Prerequisites:
    pip3 install livepeer-gateway
"""

import argparse
import base64
import json
import os
import re
import sys
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

from livepeer_gateway import (
    submit_job,
    LivepeerJob,
    list_capabilities as sdk_list_capabilities,
    LivepeerGatewayError,
    ByocTrainingRequest,
    submit_training_job,
    get_training_status,
    wait_for_training,
)

# ---- Config ----
ORCH = os.environ.get("ORCH_URL", "https://localhost:8935")
ADAPTER = os.environ.get("ADAPTER_URL", "http://localhost:9090")

_log_lock = threading.Lock()


def log(msg, level="AGENT"):
    ts = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    colors = {"AGENT": "\033[35m", "TOOL": "\033[36m", "RESULT": "\033[33m",
              "INFO": "\033[34m", "ERROR": "\033[31m", "OUTPUT": "\033[32m",
              "PARALLEL": "\033[95m", "SDK": "\033[94m"}
    tid = threading.current_thread().name
    prefix = f" [{tid}]" if tid != "MainThread" else ""
    with _log_lock:
        print(f"{colors.get(level, '')}{ts} [{level}]{prefix}\033[0m {msg}")


# ============================================================
# Tools -- Livepeer network capabilities via SDK
# ============================================================

def tool_list_capabilities():
    """List all capabilities registered on the Livepeer network via SDK."""
    log("Querying capabilities via SDK...", "SDK")
    try:
        caps_list = sdk_list_capabilities(ADAPTER)
        capabilities = {}
        for c in caps_list:
            capabilities[c["name"]] = {
                "capacity": c.get("capacity", 0),
                "model_id": c.get("model_id"),
            }
        return capabilities
    except Exception as e:
        log(f"SDK list_capabilities failed: {e}", "ERROR")
        return {}


def _save_base64_media(data_uri, media_type="video", ext="mp4"):
    """Save a base64 data URI to a local file and return the file path."""
    if data_uri.startswith("data:"):
        data_uri = data_uri.split(",", 1)[1] if "," in data_uri else data_uri.split(";base64,", 1)[-1]
    raw = base64.b64decode(data_uri)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S_%f")
    filename = f"output_{media_type}_{ts}.{ext}"
    filepath = os.path.join(os.getcwd(), filename)
    with open(filepath, "wb") as f:
        f.write(raw)
    return filepath


def tool_generate_image(prompt, capability="nano-banana"):
    """Generate an image using the SDK's unified submit_job."""
    log(f"Generating image: \"{prompt}\" via [{capability}]", "TOOL")
    log(f"Using SDK submit_job -> {ORCH}", "SDK")

    start = time.time()
    try:
        job = submit_job(
            capability,
            {"prompt": prompt, "num_images": 1},
            orch_url=ORCH,
        )
        elapsed = time.time() - start
        balance = job.balance or "N/A"

        image_url = job.image_url
        # Handle base64 images
        if not image_url and job.images:
            img = job.images[0]
            if "base64" in img:
                mime = img.get("mime_type", "")
                ext = "png"
                if "webp" in mime: ext = "webp"
                elif "jpeg" in mime or "jpg" in mime: ext = "jpg"
                filepath = _save_base64_media(img["base64"], "image", ext)
                log(f"Image saved locally: {filepath}", "RESULT")
                image_url = f"file://{filepath}"

        if not image_url and isinstance(job.data, dict):
            log(f"Image response keys: {list(job.data.keys())}", "INFO")
            log(f"Image response (truncated): {json.dumps(job.data)[:300]}", "INFO")

        log(f"Image generated in {elapsed:.1f}s | Livepeer-Balance: {balance} | type={job.job_type}", "RESULT")
        return {"ok": True, "image_url": image_url, "result": job.data,
                "elapsed": elapsed, "balance": balance}

    except LivepeerGatewayError as e:
        log(f"Image generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}
    except Exception as e:
        log(f"Image generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


def tool_generate_video(prompt, capability="ltx-t2v-23", image_url=None, video_url=None):
    """Generate a video using the SDK's unified submit_job."""
    cap = capability
    if image_url and cap == "ltx-t2v-23":
        cap = "ltx-i2v"

    if video_url:
        action = "video-to-video"
    elif image_url:
        action = "image-to-video"
    else:
        action = "text-to-video"
    log(f"Generating video ({action}): \"{prompt}\" via [{cap}]", "TOOL")
    log(f"Using SDK submit_job -> {ORCH}", "SDK")

    payload = {"prompt": prompt}
    if image_url:
        payload["image_url"] = image_url
    if video_url:
        payload["video_url"] = video_url

    start = time.time()
    try:
        job = submit_job(cap, payload, orch_url=ORCH)
        elapsed = time.time() - start
        balance = job.balance or "N/A"

        vid_url = job.video_url

        if vid_url and vid_url.startswith("data:"):
            filepath = _save_base64_media(vid_url, "video", "mp4")
            log(f"Video saved locally: {filepath}", "RESULT")
            vid_url = f"file://{filepath}"

        if not vid_url and isinstance(job.data, dict):
            log(f"Video response keys: {list(job.data.keys())}", "INFO")
            log(f"Video response (truncated): {json.dumps(job.data)[:500]}", "INFO")

        log(f"Video generated in {elapsed:.1f}s | Livepeer-Balance: {balance} | type={job.job_type}", "RESULT")
        return {"ok": True, "video_url": vid_url, "result": job.data,
                "elapsed": elapsed, "balance": balance}

    except LivepeerGatewayError as e:
        log(f"Video generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}
    except Exception as e:
        log(f"Video generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


def tool_generate_music(prompt, capability="beatoven-music", duration=15):
    """Generate music using the SDK's unified submit_job."""
    log(f"Generating music: \"{prompt}\" via [{capability}]", "TOOL")
    log(f"Using SDK submit_job -> {ORCH}", "SDK")

    start = time.time()
    try:
        job = submit_job(
            capability,
            {"prompt": prompt, "duration": duration},
            orch_url=ORCH,
        )
        elapsed = time.time() - start
        balance = job.balance or "N/A"

        audio_url = job.audio_url

        log(f"Music generated in {elapsed:.1f}s | Livepeer-Balance: {balance} | type={job.job_type}", "RESULT")
        return {"ok": True, "audio_url": audio_url, "result": job.data,
                "elapsed": elapsed, "balance": balance}

    except LivepeerGatewayError as e:
        log(f"Music generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}
    except Exception as e:
        log(f"Music generation failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


# ============================================================
# Tools -- LoRA Training via SDK
# ============================================================

# Model-specific parameter mapping for different trainers
TRAINER_PARAM_MAP = {
    "fal-ai/flux-lora-fast-training": {
        "dataset_field": "images_data_url",
        "steps_field": "steps",
        "trigger_field": "trigger_word",
        "extra": {"is_style": False, "create_masks": True},
    },
    "fal-ai/flux-lora-portrait-trainer": {
        "dataset_field": "images_data_url",
        "steps_field": "steps",
        "trigger_field": "trigger_phrase",
        "extra": {"create_masks": False, "subject_crop": True},
    },
    "fal-ai/wan-trainer/t2v-14b": {
        "dataset_field": "training_data_url",
        "steps_field": "number_of_steps",
        "trigger_field": "trigger_phrase",
        "extra": {},
    },
    "fal-ai/qwen-image-trainer": {
        "dataset_field": "image_data_url",
        "steps_field": "steps",
        "trigger_field": "trigger_phrase",
        "extra": {},
    },
}


def _build_train_params(model_id, dataset_url, trigger_word, steps, extra_params=None):
    """Build training params with correct field names for the model."""
    mapping = TRAINER_PARAM_MAP.get(model_id, {
        "dataset_field": "images_data_url",
        "steps_field": "steps",
        "trigger_field": "trigger_word",
        "extra": {},
    })
    params = {
        mapping["dataset_field"]: dataset_url,
        mapping["steps_field"]: steps,
        mapping["trigger_field"]: trigger_word,
        **mapping["extra"],
    }
    if extra_params:
        params.update(extra_params)
    return params


def tool_train_lora(images_data_url, trigger_word="lptest", steps=100,
                    model_id="fal-ai/flux-lora-fast-training",
                    capability="flux-lora-training", extra_params=None):
    """Submit a LoRA training job via the SDK (async, returns immediately)."""
    log(f"Submitting training: trigger='{trigger_word}' steps={steps}", "TOOL")
    log(f"  Dataset: {images_data_url}", "INFO")
    log(f"  Model: {model_id} via [{capability}]", "SDK")

    params = _build_train_params(model_id, images_data_url, trigger_word, steps, extra_params)
    log(f"  Params: {json.dumps(params)}", "INFO")

    start = time.time()
    try:
        req = ByocTrainingRequest(
            capability=capability,
            model_id=model_id,
            params=params,
            timeout_seconds=30,
        )
        resp = submit_training_job(req, orch_url=ORCH)
        elapsed = time.time() - start
        log(f"Job submitted in {elapsed:.1f}s: job_id={resp.job_id} status={resp.status}", "RESULT")
        if resp.status_url:
            log(f"  Status URL: {resp.status_url}", "INFO")
        return {"ok": True, "job_id": resp.job_id, "status": resp.status,
                "orchestrator_url": resp.orchestrator_url, "elapsed": elapsed}

    except Exception as e:
        log(f"Training submit failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


def tool_training_status(job_id, orch_url=None):
    """Check training job status."""
    orch = orch_url or ORCH
    log(f"Checking training status: {job_id}", "TOOL")
    try:
        status = get_training_status(job_id, orch)
        cost_info = f" cost={status.cost} balance={status.balance}" if status.cost else ""
        log(f"  Status: {status.status} progress={status.progress}%{cost_info}", "RESULT")
        result = {"ok": True, "job_id": status.job_id, "status": status.status,
                  "progress": status.progress}
        if status.cost:
            result["cost"] = status.cost
        if status.balance:
            result["balance"] = status.balance
        if status.lora_url:
            result["lora_url"] = status.lora_url
            log(f"  LoRA URL: {status.lora_url}", "OUTPUT")
        if status.config_url:
            result["config_url"] = status.config_url
        if status.error:
            result["error"] = status.error
            log(f"  Error: {status.error}", "ERROR")
        return result
    except Exception as e:
        log(f"Status check failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


def tool_train_lora_and_wait(images_data_url, trigger_word="lptest", steps=100,
                              model_id="fal-ai/flux-lora-fast-training",
                              capability="flux-lora-training",
                              poll_interval=5.0, timeout=3600.0, extra_params=None):
    """Submit a LoRA training job and wait for completion (blocking)."""
    log(f"Training (blocking): trigger='{trigger_word}' steps={steps}", "TOOL")
    log(f"  Dataset: {images_data_url}", "INFO")
    log(f"  Model: {model_id} via [{capability}]", "SDK")
    log(f"  Timeout: {timeout}s poll_interval: {poll_interval}s", "INFO")

    params = _build_train_params(model_id, images_data_url, trigger_word, steps, extra_params)
    log(f"  Params: {json.dumps(params)}", "INFO")

    start = time.time()
    try:
        req = ByocTrainingRequest(
            capability=capability,
            model_id=model_id,
            params=params,
            timeout_seconds=30,
        )
        resp = submit_training_job(req, orch_url=ORCH)
        log(f"  Submitted: job_id={resp.job_id}", "RESULT")

        # Poll until done
        orch = resp.orchestrator_url or ORCH
        log("  Waiting for training to complete...", "INFO")
        final = wait_for_training(
            resp.job_id, orch,
            poll_interval=poll_interval,
            timeout=timeout,
        )

        elapsed = time.time() - start
        log(f"  Final: status={final.status} ({elapsed:.1f}s)", "RESULT")

        result = {"ok": final.status == "completed", "job_id": final.job_id,
                  "status": final.status, "elapsed": elapsed}
        if final.lora_url:
            result["lora_url"] = final.lora_url
            log(f"  LoRA weights: {final.lora_url}", "OUTPUT")
        if final.config_url:
            result["config_url"] = final.config_url
            log(f"  Config: {final.config_url}", "OUTPUT")
        if final.error:
            result["error"] = final.error
            log(f"  Error: {final.error}", "ERROR")
        return result

    except Exception as e:
        log(f"Training failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}


def tool_train_lora_direct(images_data_url, trigger_word="lptest", steps=100,
                            model_id="fal-ai/flux-lora-fast-training",
                            poll_interval=5.0, timeout=3600.0, extra_params=None):
    """Submit a LoRA training job directly to adapter (bypasses orchestrator)."""
    import urllib.request, urllib.error

    log(f"Training (direct to adapter): trigger='{trigger_word}' steps={steps}", "TOOL")
    log(f"  Dataset: {images_data_url}", "INFO")
    log(f"  Model: {model_id}", "INFO")
    log(f"  Adapter: {ADAPTER}", "SDK")

    params = _build_train_params(model_id, images_data_url, trigger_word, steps, extra_params)
    start = time.time()
    body = {"model_id": model_id, **params}

    try:
        req = urllib.request.Request(
            f"{ADAPTER}/train",
            data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read())
    except Exception as e:
        log(f"Submit failed: {e}", "ERROR")
        return {"ok": False, "error": str(e)}

    job_id = data.get("job_id")
    if not job_id:
        log(f"No job_id in response: {data}", "ERROR")
        return {"ok": False, "error": "No job_id"}

    log(f"  Submitted: job_id={job_id}", "RESULT")
    log("  Waiting for training to complete...", "INFO")

    elapsed_poll = 0
    while elapsed_poll < timeout:
        time.sleep(poll_interval)
        elapsed_poll += poll_interval

        try:
            with urllib.request.urlopen(f"{ADAPTER}/train/{job_id}", timeout=10) as resp:
                status_data = json.loads(resp.read())
        except Exception as e:
            log(f"  Poll error: {e}", "ERROR")
            continue

        status = status_data.get("status", "unknown")
        progress = status_data.get("progress", 0)
        log(f"  [{elapsed_poll:.0f}s] status={status} progress={progress}%", "INFO")

        if status == "completed":
            result = status_data.get("result", {})
            lora_file = result.get("diffusers_lora_file", {})
            lora_url = lora_file.get("url") if isinstance(lora_file, dict) else None
            config_file = result.get("config_file", {})
            config_url = config_file.get("url") if isinstance(config_file, dict) else None
            elapsed = time.time() - start
            log(f"  Training completed in {elapsed:.1f}s!", "OUTPUT")
            if lora_url:
                log(f"  LoRA weights: {lora_url}", "OUTPUT")
            return {"ok": True, "job_id": job_id, "status": "completed",
                    "lora_url": lora_url, "config_url": config_url, "elapsed": elapsed}

        if status in ("failed", "cancelled"):
            log(f"  Training {status}: {status_data.get('error', '?')}", "ERROR")
            return {"ok": False, "job_id": job_id, "status": status,
                    "error": status_data.get("error")}

    log(f"  Timed out after {timeout}s", "ERROR")
    return {"ok": False, "job_id": job_id, "status": "timeout"}


# ============================================================
# Workflow DAG Engine (identical logic to agent-client.py)
# ============================================================

IMAGE_MODELS = {
    "banana": "nano-banana", "nano": "nano-banana",
    "gemini": "gemini-image", "flash": "gemini-image",
    "recraft": "recraft-v4",
}
VIDEO_MODELS = {
    "ltx": "ltx-i2v", "lucy": "lucy-i2v", "decart": "lucy-i2v",
    "veo": "veo31-fast", "veo31": "veo31-fast", "google": "veo31-fast",
    "kling": "kling-i2v", "sora": "sora-v2v",
}
V2V_MODELS = {
    "wan": "wan-v2v", "sora": "sora-v2v", "remix": "sora-v2v",
}
T2V_MODELS = {
    "ltx": "ltx-t2v-23",
}
MUSIC_MODELS = {
    "beatoven": "beatoven-music", "music": "beatoven-music",
}
IMAGE_QUALITY = {"recraft-v4": 3, "gemini-image": 2, "nano-banana": 1}
VIDEO_QUALITY = {"veo31-fast": 4, "kling-i2v": 4, "veo-i2v": 3, "wan-v2v": 3, "sora-v2v": 3, "lucy-i2v": 2, "ltx-i2v": 1, "ltx-t2v": 1, "ltx-t2v-23": 1}
MUSIC_QUALITY = {"beatoven-music": 1}


def resolve_cap(hint, caps, ranks, fallback_fn):
    if hint == "__best__":
        best = max((n for n in caps if ranks.get(n, 0) > 0), key=lambda n: ranks.get(n, 0), default=None)
        return best or fallback_fn(caps)
    if hint and hint in caps:
        return hint
    return fallback_fn(caps)


def parse_workflow(task_text):
    """Parse a complex multi-task description into a workflow DAG."""
    raw_tasks = re.split(r'\s*[;|]\s*|\s*\d+\)\s*', task_text)
    raw_tasks = [t.strip() for t in raw_tasks if t.strip()]

    steps = []
    step_id = 0

    for task in raw_tasks:
        task_lower = task.lower()

        model_hints = re.findall(r'\busing\s+(\w+)', task_lower)
        task = re.sub(r'\busing\s+\w+', '', task, flags=re.IGNORECASE).strip()

        choose_best = False
        for pat in ["choose the best", "best quality", "best model"]:
            if pat in task_lower:
                choose_best = True
                task = re.sub(re.escape(pat), "", task, flags=re.IGNORECASE).strip()
                break

        task_lower = task.lower()

        is_image = any(w in task_lower for w in ["image", "picture", "photo", "draw"])
        is_video = any(w in task_lower for w in ["video", "animate", "animation", "movie"])
        is_music = any(w in task_lower for w in ["music", "song", "soundtrack", "audio", "melody"])
        is_pipeline = is_image and is_video

        prompt = task
        for rm in ["create me ", "create ", "make me ", "make ", "generate me ", "generate ",
                    "draw me ", "draw ", "as an image", "as image", "as a video", "as video",
                    "give me ", "output me ", "output "]:
            prompt = prompt.replace(rm, "")
        prompt = re.sub(r'\s+', ' ', prompt).strip().rstrip(',').strip()

        is_v2v = any(w in task_lower for w in ["remix", "restyle", "turn it into", "turn into",
                                                "convert to", "transform to", "change to",
                                                "cartoon style", "anime style", "v2v"])

        img_hint = vid_hint = v2v_hint = mus_hint = None
        for h in model_hints:
            if h in IMAGE_MODELS and not img_hint: img_hint = IMAGE_MODELS[h]
            if h in V2V_MODELS and not v2v_hint: v2v_hint = V2V_MODELS[h]
            elif h in VIDEO_MODELS and not vid_hint: vid_hint = VIDEO_MODELS[h]
            if h in T2V_MODELS:
                if not vid_hint or (not is_pipeline and is_video):
                    vid_hint = T2V_MODELS[h]
            if h in MUSIC_MODELS and not mus_hint: mus_hint = MUSIC_MODELS[h]
        if choose_best:
            img_hint = img_hint or "__best__"
            vid_hint = vid_hint or "__best__"
            mus_hint = mus_hint or "__best__"

        if is_pipeline:
            connectors = [" then ", " and then ", ", and animate", ", animate",
                         " and also ", ". then "]
            img_prompt = prompt
            vid_prompt = prompt
            for conn in connectors:
                if conn in task_lower:
                    idx = task_lower.index(conn)
                    img_prompt = task[:idx].strip()
                    vid_prompt = task[idx + len(conn):].strip()
                    for rm in ["create me ", "create ", "make me ", "make ", "generate ",
                               "as an image", "as image"]:
                        img_prompt = img_prompt.replace(rm, "").strip()
                    for rm in ["create me a video that ", "create a video of ",
                               "make a video of ", "it as a video", "it to video"]:
                        vid_prompt = vid_prompt.replace(rm, "").strip()
                    break

            img_id = f"step_{step_id}"
            steps.append({
                "id": img_id, "action": "image", "prompt": img_prompt,
                "model_hint": img_hint, "depends_on": [],
            })
            step_id += 1
            steps.append({
                "id": f"step_{step_id}", "action": "video", "prompt": vid_prompt,
                "model_hint": vid_hint, "depends_on": [img_id],
                "use_image_from": img_id,
            })
            step_id += 1

        elif is_music:
            steps.append({
                "id": f"step_{step_id}", "action": "music", "prompt": prompt,
                "model_hint": mus_hint, "depends_on": [],
            })
            step_id += 1

        elif is_v2v and steps:
            prev_vid = None
            for s in reversed(steps):
                if s["action"] == "video":
                    prev_vid = s["id"]
                    break
            if prev_vid:
                steps.append({
                    "id": f"step_{step_id}", "action": "v2v", "prompt": prompt,
                    "model_hint": v2v_hint or "wan-v2v", "depends_on": [prev_vid],
                    "use_video_from": prev_vid,
                })
                step_id += 1
            else:
                steps.append({
                    "id": f"step_{step_id}", "action": "video", "prompt": prompt,
                    "model_hint": vid_hint, "depends_on": [],
                })
                step_id += 1

        elif is_video:
            steps.append({
                "id": f"step_{step_id}", "action": "video", "prompt": prompt,
                "model_hint": vid_hint, "depends_on": [],
            })
            step_id += 1

        elif is_image:
            steps.append({
                "id": f"step_{step_id}", "action": "image", "prompt": prompt,
                "model_hint": img_hint, "depends_on": [],
            })
            step_id += 1

        else:
            steps.append({
                "id": f"step_{step_id}", "action": "image", "prompt": prompt,
                "model_hint": img_hint, "depends_on": [],
            })
            step_id += 1

    return steps


def execute_step(step, caps, results):
    """Execute a single workflow step."""
    action = step["action"]
    prompt = step["prompt"]
    hint = step.get("model_hint")

    if action == "image":
        def fb(c):
            if "nano-banana" in c: return "nano-banana"
            return next((n for n in c if "image" in n), None)
        cap = resolve_cap(hint, caps, IMAGE_QUALITY, fb)
        if not cap:
            log(f"[{step['id']}] No image capability!", "ERROR")
            return None
        log(f"[{step['id']}] Selected: {cap} ({caps.get(cap, {}).get('model_id', '?')})", "INFO")
        result = tool_generate_image(prompt, capability=cap)
        if result["ok"]:
            return {"type": "image", "url": result["image_url"], "prompt": prompt,
                    "capability": cap, "elapsed": result["elapsed"],
                    "balance": result["balance"], "step_id": step["id"]}

    elif action == "video":
        image_url = None
        img_from = step.get("use_image_from")
        if img_from and img_from in results:
            dep_art = results[img_from]
            if dep_art:
                image_url = dep_art.get("url")

        def fb(c):
            if image_url:
                if "ltx-i2v" in c: return "ltx-i2v"
                return next((n for n in c if "i2v" in n), "ltx-t2v")
            if "ltx-t2v" in c: return "ltx-t2v"
            return next((n for n in c if "video" in n or "t2v" in n), None)
        cap = resolve_cap(hint, caps, VIDEO_QUALITY, fb)
        if not cap:
            log(f"[{step['id']}] No video capability!", "ERROR")
            return None
        log(f"[{step['id']}] Selected: {cap} ({caps.get(cap, {}).get('model_id', '?')})", "INFO")
        result = tool_generate_video(prompt, capability=cap, image_url=image_url)
        if result["ok"]:
            return {"type": "video", "url": result["video_url"], "prompt": prompt,
                    "capability": cap, "elapsed": result["elapsed"],
                    "balance": result["balance"], "used_image": image_url is not None,
                    "step_id": step["id"]}

    elif action == "music":
        def fb(c):
            return next((n for n in c if "music" in n or "audio" in n or "beatoven" in n), None)
        cap = resolve_cap(hint, caps, MUSIC_QUALITY, fb)
        if not cap:
            log(f"[{step['id']}] No music capability!", "ERROR")
            return None
        log(f"[{step['id']}] Selected: {cap} ({caps.get(cap, {}).get('model_id', '?')})", "INFO")
        result = tool_generate_music(prompt, capability=cap)
        if result["ok"]:
            return {"type": "music", "url": result["audio_url"], "prompt": prompt,
                    "capability": cap, "elapsed": result["elapsed"],
                    "balance": result["balance"], "step_id": step["id"]}

    elif action == "v2v":
        video_url = None
        vid_from = step.get("use_video_from")
        if vid_from and vid_from in results:
            dep_art = results[vid_from]
            if dep_art:
                video_url = dep_art.get("url")
        if not video_url:
            log(f"[{step['id']}] No source video for v2v!", "ERROR")
            return None

        def fb(c):
            if "wan-v2v" in c: return "wan-v2v"
            if "sora-v2v" in c: return "sora-v2v"
            return next((n for n in c if "v2v" in n or "remix" in n), None)
        cap = resolve_cap(hint, caps, VIDEO_QUALITY, fb)
        if not cap:
            log(f"[{step['id']}] No v2v capability!", "ERROR")
            return None
        log(f"[{step['id']}] Selected: {cap} ({caps.get(cap, {}).get('model_id', '?')})", "INFO")
        log(f"[{step['id']}] Source video: {video_url}", "INFO")
        result = tool_generate_video(prompt, capability=cap, video_url=video_url)
        if result["ok"]:
            return {"type": "video", "url": result["video_url"], "prompt": prompt,
                    "capability": cap, "elapsed": result["elapsed"],
                    "balance": result["balance"], "used_video": True, "step_id": step["id"]}

    return None


def run_workflow(task_text):
    """Execute a complex multi-task workflow with concurrent execution."""
    log("=" * 70, "INFO")
    log(f"Workflow: \"{task_text}\"", "AGENT")
    log("Using livepeer-gateway SDK (submit_byoc_job)", "SDK")
    log("=" * 70, "INFO")

    # Step 1: Discover capabilities via SDK
    log("Phase 1: Discovering network capabilities via SDK...", "AGENT")
    caps = tool_list_capabilities()
    if not caps:
        log("No capabilities found!", "ERROR")
        return
    for name, info in caps.items():
        log(f"  [{name}] model={info.get('model_id', '?')}", "INFO")

    # Step 2: Parse workflow DAG
    log("Phase 2: Building execution DAG...", "AGENT")
    steps = parse_workflow(task_text)

    independent = [s for s in steps if not s["depends_on"]]
    dependent = [s for s in steps if s["depends_on"]]
    log(f"  Total steps: {len(steps)}", "INFO")
    log(f"  Independent (run concurrently): {len(independent)}", "PARALLEL")
    log(f"  Dependent (wait for input): {len(dependent)}", "INFO")
    for s in steps:
        deps = f" <- depends on {s['depends_on']}" if s["depends_on"] else " [CONCURRENT]"
        hint = f" (model: {s.get('model_hint', 'auto')})" if s.get("model_hint") else ""
        log(f"  {s['id']}: {s['action']}{hint} - \"{s['prompt']}\"{deps}", "INFO")

    # Step 3: Execute DAG with concurrency
    log("Phase 3: Executing workflow...", "AGENT")
    total_start = time.time()
    results = {}
    completed = set()

    def step_ready(s):
        return all(d in completed for d in s["depends_on"])

    artifact_order = []

    def _show_artifact(art, label=""):
        wall = time.time() - total_start
        idx = len(artifact_order)
        artifact_order.append(art)
        url = art.get("url") or "N/A"
        log("", "INFO")
        log(f"{'─' * 60}", "OUTPUT")
        log(f"ARTIFACT {idx+1} READY  [{art['step_id']}]  (wall +{wall:.1f}s){label}", "OUTPUT")
        log(f"{'─' * 60}", "OUTPUT")
        log(f"  Type:       {art['type'].upper()}", "OUTPUT")
        log(f"  Prompt:     \"{art['prompt']}\"", "OUTPUT")
        log(f"  Model:      {art['capability']}", "OUTPUT")
        log(f"  Inference:  {art['elapsed']:.1f}s", "OUTPUT")
        if art.get("used_image"):
            log(f"  Input:      image from pipeline dependency", "OUTPUT")
        if art.get("used_video"):
            log(f"  Input:      video from pipeline dependency (v2v)", "OUTPUT")
        log(f"  >>> URL:    {url}", "OUTPUT")
        log(f"{'─' * 60}", "OUTPUT")

    remaining = list(steps)
    while remaining:
        ready = [s for s in remaining if step_ready(s)]
        if not ready:
            log("Deadlock: no steps ready!", "ERROR")
            break

        remaining = [s for s in remaining if s not in ready]

        if len(ready) == 1:
            s = ready[0]
            log(f"--- Executing {s['id']} ({s['action']}) ---", "AGENT")
            art = execute_step(s, caps, results)
            results[s["id"]] = art
            completed.add(s["id"])
            if art:
                _show_artifact(art)
        else:
            log(f"--- Executing {len(ready)} steps CONCURRENTLY ---", "PARALLEL")
            for s in ready:
                log(f"  >> {s['id']}: {s['action']} - \"{s['prompt']}\"", "PARALLEL")

            with ThreadPoolExecutor(max_workers=len(ready),
                                    thread_name_prefix="worker") as pool:
                futures = {
                    pool.submit(execute_step, s, caps, results): s
                    for s in ready
                }
                for future in as_completed(futures):
                    s = futures[future]
                    try:
                        art = future.result()
                    except Exception as e:
                        log(f"[{s['id']}] failed: {e}", "ERROR")
                        art = None
                    results[s["id"]] = art
                    completed.add(s["id"])
                    if art:
                        _show_artifact(art, "  [concurrent]")

    total_elapsed = time.time() - total_start
    artifacts = [a for a in results.values() if a]

    # Final summary
    log("", "INFO")
    log("=" * 70, "INFO")
    log("WORKFLOW COMPLETE", "OUTPUT")
    log("=" * 70, "INFO")
    log(f"Task: \"{task_text}\"", "OUTPUT")
    log(f"Artifacts produced: {len(artifacts)}/{len(steps)}", "OUTPUT")
    log(f"Total wall time: {total_elapsed:.1f}s", "OUTPUT")

    sum_elapsed = sum(a["elapsed"] for a in artifacts)
    if sum_elapsed > total_elapsed and len([s for s in steps if not s["depends_on"]]) > 1:
        log(f"Sum of inference times: {sum_elapsed:.1f}s (parallelism saved {sum_elapsed - total_elapsed:.1f}s)", "OUTPUT")
    log("", "INFO")

    if artifacts:
        log("=" * 70, "OUTPUT")
        log("RESULT URLs (copy-paste ready)", "OUTPUT")
        log("=" * 70, "OUTPUT")
        for i, art in enumerate(artifacts, 1):
            art_type = art["type"].upper()
            cap = art["capability"]
            url = art.get("url") or "N/A"
            log(f"  [{i}] {art_type:<6} ({cap}): {url}", "OUTPUT")
        log("=" * 70, "OUTPUT")

    log("", "INFO")
    log("All artifacts produced through the Livepeer BYOC network via SDK.", "OUTPUT")
    log("Each request: SDK (submit_byoc_job) -> Orchestrator -> Adapter -> Proxy -> Provider", "OUTPUT")

    return artifacts


# ============================================================
# Interactive mode
# ============================================================

def interactive_mode():
    log("=" * 70, "INFO")
    log("Livepeer Network Agent (SDK version) - Interactive Mode", "AGENT")
    log("Using livepeer-gateway SDK for all BYOC requests", "SDK")
    log("", "INFO")
    log("Separate independent tasks with ; or |", "INFO")
    log("Chain dependent tasks with 'then' (image -> video pipeline)", "INFO")
    log("", "INFO")
    log("Examples:", "INFO")
    log('  "a dragon as image using recraft ; a castle as image using nano ; epic music"', "INFO")
    log('  "a cat as image then animate it using lucy ; a dog as image using recraft"', "INFO")
    log("", "INFO")
    log("Models: recraft, nano, gemini | ltx, lucy, veo, kling | beatoven", "INFO")
    log("=" * 70, "INFO")

    while True:
        try:
            task = input("\n\033[35m> \033[0m").strip()
        except (KeyboardInterrupt, EOFError):
            print()
            break
        if not task or task.lower() in ("quit", "exit", "q"):
            break
        run_workflow(task)


def cmd_register(args):
    """Register, unregister, or list capabilities on the adapter."""
    if args.register_action in ("list", "ls"):
        caps = tool_list_capabilities()
        if not caps:
            log("No capabilities found.", "INFO")
            return
        log(f"{'Name':<20} {'Model ID':<50} {'Cap':>4}", "INFO")
        log("─" * 76, "INFO")
        for name, info in caps.items():
            log(f"{name:<20} {info.get('model_id', '?'):<50} {info.get('capacity', '?'):>4}", "OUTPUT")

    elif args.register_action in ("add",):
        if not args.name or not args.model_id:
            log("--name and --model-id are required for add", "ERROR")
            return
        import urllib.request
        body = {"name": args.name, "model_id": args.model_id, "capacity": args.capacity}
        try:
            req = urllib.request.Request(
                f"{ADAPTER}/capabilities",
                data=json.dumps(body).encode(),
                headers={"Content-Type": "application/json"},
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                json.loads(resp.read())
                log(f"Registered: {args.name} -> {args.model_id} (capacity={args.capacity})", "OUTPUT")
        except Exception as e:
            log(f"Registration failed: {e}", "ERROR")

    elif args.register_action in ("remove", "rm"):
        if not args.name:
            log("--name is required for remove", "ERROR")
            return
        import urllib.request
        try:
            req = urllib.request.Request(
                f"{ADAPTER}/capabilities/{args.name}",
                method="DELETE",
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                json.loads(resp.read())
                log(f"Removed: {args.name}", "OUTPUT")
        except Exception as e:
            log(f"Removal failed: {e}", "ERROR")


def cmd_train(args):
    """Handle training subcommand."""
    if args.train_action in ("submit",):
        if not args.dataset_url:
            log("--dataset-url is required", "ERROR")
            return
        extra = json.loads(args.params) if args.params else None
        if args.direct:
            result = tool_train_lora_direct(
                images_data_url=args.dataset_url,
                trigger_word=args.trigger_word,
                steps=args.steps,
                model_id=args.model_id,
                poll_interval=args.poll_interval,
                timeout=args.timeout,
                extra_params=extra,
            )
        elif args.wait:
            result = tool_train_lora_and_wait(
                images_data_url=args.dataset_url,
                trigger_word=args.trigger_word,
                steps=args.steps,
                model_id=args.model_id,
                capability=args.capability,
                poll_interval=args.poll_interval,
                timeout=args.timeout,
                extra_params=extra,
            )
        else:
            result = tool_train_lora(
                images_data_url=args.dataset_url,
                trigger_word=args.trigger_word,
                steps=args.steps,
                model_id=args.model_id,
                capability=args.capability,
                extra_params=extra,
            )
        log(f"Result: {json.dumps(result, indent=2)}", "OUTPUT")

    elif args.train_action in ("status",):
        if not args.job_id:
            log("--job-id is required", "ERROR")
            return
        result = tool_training_status(args.job_id, args.orch_url)
        log(f"Result: {json.dumps(result, indent=2)}", "OUTPUT")


def main():
    parser = argparse.ArgumentParser(
        description="Livepeer Network Agent Client (SDK version)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Inference (image/video/music):
  python3 agent-client-sdk.py --task "a dragon as image using recraft"
  python3 agent-client-sdk.py -i

  # LoRA Training:
  python3 agent-client-sdk.py train submit --dataset-url URL --wait
  python3 agent-client-sdk.py train submit --dataset-url URL --direct --steps 10
  python3 agent-client-sdk.py train status --job-id JOB_ID

  # Capabilities:
  python3 agent-client-sdk.py reg ls
""",
    )
    sub = parser.add_subparsers(dest="command")

    parser.add_argument("--task", type=str, help="Task to execute")
    parser.add_argument("--interactive", "-i", action="store_true", help="Interactive mode")

    # Register subcommand
    reg = sub.add_parser("register", aliases=["reg"], help="Manage capabilities")
    reg_sub = reg.add_subparsers(dest="register_action")

    reg_sub.add_parser("list", aliases=["ls"], help="List capabilities")

    reg_add = reg_sub.add_parser("add", help="Register a capability")
    reg_add.add_argument("--name", "-n", required=True, help="Capability name")
    reg_add.add_argument("--model-id", "-m", required=True, help="Model ID")
    reg_add.add_argument("--capacity", "-c", type=int, default=5, help="Capacity")

    reg_rm = reg_sub.add_parser("remove", aliases=["rm"], help="Unregister a capability")
    reg_rm.add_argument("--name", "-n", required=True, help="Capability name to remove")

    # Train subcommand
    train = sub.add_parser("train", help="LoRA training jobs")
    train_sub = train.add_subparsers(dest="train_action")

    train_submit = train_sub.add_parser("submit", help="Submit a LoRA training job")
    train_submit.add_argument("--dataset-url", "-d", required=True, help="URL to ZIP of training images")
    train_submit.add_argument("--trigger-word", "-t", default="lptest", help="Trigger word (default: lptest)")
    train_submit.add_argument("--steps", "-s", type=int, default=100, help="Training steps (default: 100)")
    train_submit.add_argument("--model-id", "-m", default="fal-ai/flux-lora-fast-training", help="Training model ID")
    train_submit.add_argument("--capability", default="flux-lora-training", help="Capability name")
    train_submit.add_argument("--wait", "-w", action="store_true", help="Wait for completion (blocking)")
    train_submit.add_argument("--direct", action="store_true", help="Submit directly to adapter (bypass orchestrator)")
    train_submit.add_argument("--poll-interval", type=float, default=5.0, help="Poll interval in seconds")
    train_submit.add_argument("--timeout", type=float, default=3600.0, help="Max wait time in seconds")
    train_submit.add_argument("--params", "-p", type=str, help="Extra params as JSON (e.g. '{\"learning_rate\": 0.0001}')")

    train_status = train_sub.add_parser("status", help="Check training job status")
    train_status.add_argument("--job-id", "-j", required=True, help="Job ID to check")
    train_status.add_argument("--orch-url", help="Orchestrator URL (default: ORCH_URL env)")

    args = parser.parse_args()

    if args.command in ("register", "reg"):
        if not args.register_action:
            args.register_action = "list"
        cmd_register(args)
    elif args.command == "train":
        if not args.train_action:
            args.train_action = "submit"
        cmd_train(args)
    elif args.task:
        run_workflow(args.task)
    elif args.interactive:
        interactive_mode()
    else:
        run_workflow(
            "a fierce dragon on a volcano as image using recraft ; "
            "a crystal castle at sunset as image using nano ; "
            "a spaceship in nebula as image using gemini ; "
            "epic cinematic battle music"
        )


if __name__ == "__main__":
    main()
