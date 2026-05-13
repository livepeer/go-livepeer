#!/usr/bin/env python3
"""
Thin Agent Client -- uses the Livepeer SDK Service (REST API) instead of the SDK directly.

Architecture: Agent -> SDK Service (Cloud Run) -> Orchestrator -> Adapter -> Proxy -> Provider

The SDK Service handles all Livepeer protocol details (headers, discovery, SSL,
polling). This client only needs `urllib` (stdlib) -- zero dependencies.

Compared to agent-client-sdk.py:
  - No SDK install needed (no pip, no grpcio, no protobuf, no av)
  - No orchestrator URL, no Livepeer headers, no SSL context
  - Just HTTP POST to a single REST endpoint
  - ~3x less code for the same functionality

Usage:
    python3 agent-client-thin.py --task "create a dragon as image using recraft"
    python3 agent-client-thin.py -i
    python3 agent-client-thin.py reg ls
    python3 agent-client-thin.py train submit -d DATASET_URL --steps 10
    python3 agent-client-thin.py train status --job-id JOB_ID
"""

import argparse
import json
import os
import re
import sys
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from urllib.request import Request, urlopen
from urllib.error import HTTPError

# ---- Config ----
SDK_SERVICE = os.environ.get(
    "SDK_SERVICE_URL",
    "https://livepeer-sdk-service-90265565772.us-central1.run.app",
)

_log_lock = threading.Lock()


def log(msg, level="AGENT"):
    ts = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    colors = {"AGENT": "\033[35m", "TOOL": "\033[36m", "RESULT": "\033[33m",
              "INFO": "\033[34m", "ERROR": "\033[31m", "OUTPUT": "\033[32m",
              "PARALLEL": "\033[95m"}
    tid = threading.current_thread().name
    prefix = f" [{tid}]" if tid != "MainThread" else ""
    with _log_lock:
        print(f"{colors.get(level, '')}{ts} [{level}]{prefix}\033[0m {msg}")


# ============================================================
# Tools -- all go through the SDK Service REST API
# ============================================================

def _sdk_get(path, timeout=30):
    """GET from SDK Service."""
    url = f"{SDK_SERVICE}{path}"
    req = Request(url, headers={"Accept": "application/json"})
    with urlopen(req, timeout=timeout) as r:
        return json.loads(r.read())


def _sdk_post(path, body, timeout=300):
    """POST to SDK Service."""
    url = f"{SDK_SERVICE}{path}"
    req = Request(url, data=json.dumps(body).encode(), headers={
        "Content-Type": "application/json",
    })
    try:
        with urlopen(req, timeout=timeout) as r:
            return json.loads(r.read())
    except HTTPError as e:
        err = e.read().decode()[:500]
        return {"error": f"HTTP {e.code}: {err}"}


def tool_list_capabilities():
    """List all capabilities from the Livepeer network."""
    log("Querying capabilities via SDK Service...", "TOOL")
    caps = _sdk_get("/capabilities")
    log(f"{'Name':<25s}{'Model ID':<50s}{'Cap':>4s}", "INFO")
    log("─" * 80, "INFO")
    for c in caps:
        log(f"{c['name']:<25s}{c['model_id']:<50s}{c['capacity']:>4d}", "OUTPUT")
    return caps


def tool_generate_image(prompt, capability="nano-banana", **extra):
    """Generate an image using a Livepeer network capability."""
    log(f'Generating image: "{prompt}" via [{capability}]', "TOOL")
    body = {"capability": capability, "prompt": prompt, "params": extra}
    result = _sdk_post("/inference", body)
    if "error" in result:
        log(f"Error: {result['error']}", "ERROR")
        return None
    url = result.get("image_url")
    if url:
        log(f"Image URL: {url}", "RESULT")
        log(f"Elapsed: {result.get('elapsed_ms', 0)}ms", "INFO")
    return result


def tool_generate_video(prompt, capability="ltx-t2v-23", image_url=None, video_url=None, **extra):
    """Generate a video using a Livepeer network capability."""
    params = {**extra}
    if image_url:
        params["image_url"] = image_url
    if video_url:
        params["video_url"] = video_url
    action = "image-to-video" if image_url else "video-to-video" if video_url else "text-to-video"
    log(f'Generating video ({action}): "{prompt}" via [{capability}]', "TOOL")
    body = {"capability": capability, "prompt": prompt, "params": params}
    result = _sdk_post("/inference", body, timeout=300)
    if "error" in result:
        log(f"Error: {result['error']}", "ERROR")
        return None
    url = result.get("video_url") or result.get("image_url")
    if url:
        log(f"Video URL: {url}", "RESULT")
        log(f"Elapsed: {result.get('elapsed_ms', 0)}ms", "INFO")
    return result


def tool_generate_music(prompt, capability="beatoven-music", duration=15, **extra):
    """Generate music using a Livepeer network capability."""
    log(f'Generating music: "{prompt}" via [{capability}]', "TOOL")
    params = {"duration": duration, **extra}
    body = {"capability": capability, "prompt": prompt, "params": params}
    result = _sdk_post("/inference", body, timeout=120)
    if "error" in result:
        log(f"Error: {result['error']}", "ERROR")
        return None
    url = result.get("audio_url")
    if url:
        log(f"Audio URL: {url}", "RESULT")
    return result


def tool_train_lora(dataset_url, model_id="fal-ai/flux-lora-fast-training",
                    capability="flux-lora-training", wait=False, **params):
    """Submit a LoRA training job."""
    log(f"Submitting training: model={model_id} cap={capability}", "TOOL")
    body = {
        "capability": capability,
        "model_id": model_id,
        "params": {"images_data_url": dataset_url, **params},
        "wait": wait,
    }
    result = _sdk_post("/train", body, timeout=600 if wait else 60)
    if "error" in result:
        log(f"Error: {result['error']}", "ERROR")
        return None
    log(f"Job ID: {result.get('job_id')}", "RESULT")
    log(f"Status: {result.get('status')}", "INFO")
    return result


def tool_training_status(job_id):
    """Check training job status."""
    log(f"Checking training status: {job_id}", "TOOL")
    result = _sdk_get(f"/train/{job_id}")
    log(f"Status: {result.get('status')}  Progress: {result.get('progress', 0)}%", "RESULT")
    if result.get("error"):
        log(f"Error: {result['error']}", "ERROR")
    if result.get("result"):
        log(f"Result: {json.dumps(result['result'])[:200]}", "INFO")
    return result


# ============================================================
# Task planner -- same NLP parsing as agent-client-sdk.py
# ============================================================

CAPABILITY_MAP = {
    "nano": "nano-banana", "banana": "nano-banana",
    "recraft": "recraft-v4",
    "gemini": "gemini-image",
    "ltx": "ltx-t2v-23",
    "lucy": "lucy-i2v",
    "wan": "wan-v2v",
    "seedream": "seedream-4",
    "imagen": "imagen-4",
    "beatoven": "beatoven-music", "music": "beatoven-music",
}


def resolve_capability(hint, media_type="image"):
    """Resolve a human-friendly name to a capability."""
    hint_lower = hint.lower().strip()
    for key, cap in CAPABILITY_MAP.items():
        if key in hint_lower:
            return cap
    if media_type == "video":
        return "ltx-t2v-23"
    if media_type == "music":
        return "beatoven-music"
    return "nano-banana"


def plan_and_execute(task):
    """Parse a natural language task into tool calls and execute them."""
    log(f'Task: "{task}"', "AGENT")

    steps = [s.strip() for s in re.split(r',\s*then\s+|,\s*and\s+then\s+|\.\s+then\s+', task, flags=re.IGNORECASE)]
    results = []
    prev_image_url = None
    prev_video_url = None
    prev_audio_url = None

    for i, step in enumerate(steps):
        step_lower = step.lower()
        log(f"Step {i+1}/{len(steps)}: {step}", "AGENT")

        cap_match = re.search(r'using\s+(\S+)', step_lower)
        cap_hint = cap_match.group(1) if cap_match else ""
        prompt = re.sub(r'\s*using\s+\S+.*$', '', step, flags=re.IGNORECASE).strip()
        prompt = re.sub(r'^(create|generate|make)\s+(an?\s+)?', '', prompt, flags=re.IGNORECASE).strip()

        if any(w in step_lower for w in ("video", "animate", "i2v", "t2v", "v2v")):
            cap = resolve_capability(cap_hint, "video")
            if "i2v" in cap or "lucy" in cap_lower if "lucy" in step_lower else False:
                result = tool_generate_video(prompt, cap, image_url=prev_image_url)
            elif "v2v" in cap:
                result = tool_generate_video(prompt, cap, video_url=prev_video_url)
            else:
                result = tool_generate_video(prompt, cap, image_url=prev_image_url if prev_image_url else None)
            if result:
                prev_video_url = result.get("video_url")

        elif any(w in step_lower for w in ("music", "audio", "song", "soundtrack")):
            cap = resolve_capability(cap_hint, "music")
            result = tool_generate_music(prompt, cap)
            if result:
                prev_audio_url = result.get("audio_url")

        else:
            cap = resolve_capability(cap_hint, "image")
            result = tool_generate_image(prompt, cap)
            if result:
                prev_image_url = result.get("image_url")

        results.append(result)

    # Summary
    log("", "INFO")
    log("=" * 70, "OUTPUT")
    log("RESULT URLs", "OUTPUT")
    log("=" * 70, "OUTPUT")
    for i, r in enumerate(results):
        if not r:
            continue
        if r.get("image_url"):
            log(f"  [{i+1}] IMAGE: {r['image_url']}", "OUTPUT")
        if r.get("video_url"):
            log(f"  [{i+1}] VIDEO: {r['video_url']}", "OUTPUT")
        if r.get("audio_url"):
            log(f"  [{i+1}] AUDIO: {r['audio_url']}", "OUTPUT")
    log("=" * 70, "OUTPUT")
    return results


# ============================================================
# CLI
# ============================================================

def cmd_register(args):
    if args.reg_action == "ls":
        tool_list_capabilities()
    else:
        log(f"Unknown action: {args.reg_action}", "ERROR")


def cmd_train(args):
    if args.train_action == "submit":
        params = {}
        if args.steps:
            params["steps"] = args.steps
        if args.trigger:
            params["trigger_word"] = args.trigger
        if args.extra_params:
            params.update(json.loads(args.extra_params))
        tool_train_lora(
            dataset_url=args.dataset_url,
            model_id=args.model_id,
            capability=args.capability,
            wait=args.wait,
            **params,
        )
    elif args.train_action == "status":
        tool_training_status(args.job_id)
    else:
        log(f"Unknown train action: {args.train_action}", "ERROR")


def main():
    parser = argparse.ArgumentParser(description="Thin Agent Client (SDK Service)")
    parser.add_argument("--task", "-t", help="Natural language task")
    parser.add_argument("--interactive", "-i", action="store_true", help="Interactive mode")
    sub = parser.add_subparsers(dest="command")

    reg_parser = sub.add_parser("reg", aliases=["register"])
    reg_parser.add_argument("reg_action", choices=["ls", "list"])
    reg_parser.set_defaults(func=cmd_register)

    train_parser = sub.add_parser("train")
    train_sub = train_parser.add_subparsers(dest="train_action")
    submit_p = train_sub.add_parser("submit")
    submit_p.add_argument("-d", "--dataset-url", required=True)
    submit_p.add_argument("-m", "--model-id", default="fal-ai/flux-lora-fast-training")
    submit_p.add_argument("-c", "--capability", default="flux-lora-training")
    submit_p.add_argument("--steps", type=int, default=None)
    submit_p.add_argument("--trigger", default=None)
    submit_p.add_argument("--wait", action="store_true")
    submit_p.add_argument("--params", dest="extra_params", default=None)
    status_p = train_sub.add_parser("status")
    status_p.add_argument("--job-id", required=True)
    train_parser.set_defaults(func=cmd_train)

    args = parser.parse_args()

    if args.command in ("reg", "register"):
        cmd_register(args)
    elif args.command == "train":
        cmd_train(args)
    elif args.task:
        plan_and_execute(args.task)
    elif args.interactive:
        log("Interactive mode. Type a task or 'quit' to exit.", "AGENT")
        while True:
            try:
                task = input("\n> ").strip()
            except (EOFError, KeyboardInterrupt):
                break
            if task.lower() in ("quit", "exit", "q"):
                break
            if task:
                plan_and_execute(task)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
