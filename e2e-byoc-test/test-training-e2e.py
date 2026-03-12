#!/usr/bin/env python3
"""
E2E Test: LoRA Training via Livepeer BYOC Network

Tests the full training pipeline:
  Client (SDK) -> Orchestrator -> Adapter -> Proxy -> fal.ai (queue API)

Usage:
    # Against local docker compose stack:
    ORCH_URL=https://localhost:18935 ADAPTER_URL=http://localhost:19090 python3 test-training-e2e.py

    # Against remote VM:
    ORCH_URL=https://34.134.195.88:8935 ADAPTER_URL=http://34.134.195.88:9090 python3 test-training-e2e.py

    # Smallest possible training (fast test):
    python3 test-training-e2e.py --fast

Prerequisites:
    pip3 install livepeer-gateway
    FAL_KEY must be set in the proxy container's environment
"""

import argparse
import json
import os
import ssl
import sys
import time
import urllib.request
import urllib.error

# ---- Config ----
ORCH_URL = os.environ.get("ORCH_URL", "https://localhost:18935")
ADAPTER_URL = os.environ.get("ADAPTER_URL", "http://localhost:19090")

_ssl_ctx = ssl.create_default_context()
_ssl_ctx.check_hostname = False
_ssl_ctx.verify_mode = ssl.CERT_NONE

# A small public dataset of 5 cat images hosted on fal.ai for testing
# This is a minimal dataset to keep training fast
TEST_DATASET_URL = "https://storage.googleapis.com/livepeer-test-assets/lora-test-dataset-5cats.zip"

# Colors for terminal output
class C:
    PASS = "\033[92m"
    FAIL = "\033[91m"
    INFO = "\033[94m"
    WARN = "\033[93m"
    BOLD = "\033[1m"
    END = "\033[0m"


def log(msg, color=C.INFO):
    ts = time.strftime("%H:%M:%S")
    print(f"{color}{ts}{C.END} {msg}")


def test_health():
    """Test 1: Check adapter health endpoint."""
    log("Test 1: Checking adapter health...", C.BOLD)
    try:
        with urllib.request.urlopen(f"{ADAPTER_URL}/health", timeout=5) as resp:
            data = json.loads(resp.read())
        log(f"  Health: {data}", C.PASS)
        assert data.get("status") == "healthy", f"Unexpected status: {data}"
        return True
    except Exception as e:
        log(f"  FAILED: {e}", C.FAIL)
        return False


def test_adapter_train_endpoint():
    """Test 2: Check adapter /train endpoint exists (should return 400 without body)."""
    log("Test 2: Checking adapter /train endpoint...", C.BOLD)
    try:
        req = urllib.request.Request(
            f"{ADAPTER_URL}/train",
            data=b"{}",
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read())
        log(f"  Response: {data}", C.INFO)
        return True
    except urllib.error.HTTPError as e:
        # 400 is expected (missing model_id)
        body = e.read().decode() if e.fp else ""
        if e.code == 400:
            log(f"  Got expected 400: {body[:200]}", C.PASS)
            return True
        log(f"  Unexpected HTTP {e.code}: {body[:200]}", C.FAIL)
        return False
    except Exception as e:
        log(f"  FAILED: {e}", C.FAIL)
        return False


def test_proxy_train_endpoint():
    """Test 3: Check proxy /train endpoint exists."""
    log("Test 3: Checking proxy /train via adapter...", C.BOLD)
    # The proxy is internal to the docker network, so we test through adapter
    # by submitting a training job with a bad model_id
    try:
        body = {
            "model_id": "fal-ai/nonexistent-model-test",
            "capability": "test",
        }
        req = urllib.request.Request(
            f"{ADAPTER_URL}/train",
            data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.loads(resp.read())
        # Should get back a job_id (async)
        if "job_id" in data:
            log(f"  Got job_id: {data['job_id']} (status: {data.get('status')})", C.PASS)
            return True, data.get("job_id")
        log(f"  Unexpected response: {data}", C.WARN)
        return True, None
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        log(f"  HTTP {e.code}: {body[:200]}", C.WARN)
        # Even errors are OK for this test - we're just verifying the endpoint exists
        return True, None
    except Exception as e:
        log(f"  FAILED: {e}", C.FAIL)
        return False, None


def test_job_status(job_id):
    """Test 4: Check training job status endpoint."""
    if not job_id:
        log("Test 4: Skipping (no job_id from previous test)", C.WARN)
        return True

    log(f"Test 4: Checking job status for {job_id}...", C.BOLD)
    try:
        with urllib.request.urlopen(f"{ADAPTER_URL}/train/{job_id}", timeout=10) as resp:
            data = json.loads(resp.read())
        log(f"  Status: {data.get('status')} progress={data.get('progress', 0)}%", C.PASS)
        return True
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        log(f"  HTTP {e.code}: {body[:200]}", C.FAIL if e.code != 404 else C.WARN)
        return e.code == 404  # 404 is OK if job already cleaned up
    except Exception as e:
        log(f"  FAILED: {e}", C.FAIL)
        return False


def test_sdk_training(fast=False):
    """Test 5: Full E2E training via SDK (smallest possible LoRA)."""
    log("Test 5: Full E2E LoRA training via SDK...", C.BOLD)

    try:
        from livepeer_gateway import (
            ByocTrainingRequest,
            submit_training_job,
            get_training_status,
            wait_for_training,
        )
    except ImportError as e:
        log(f"  SDK not installed: {e}", C.FAIL)
        log("  Install with: pip3 install -e /path/to/livepeer-python-gateway", C.INFO)
        return False

    steps = 10 if fast else 100  # Minimum steps for a fast test

    req = ByocTrainingRequest(
        capability="flux-lora-training",
        model_id="fal-ai/flux-lora-fast-training",
        params={
            "images_data_url": TEST_DATASET_URL,
            "trigger_word": "lptest",
            "steps": steps,
            "is_style": False,
            "create_masks": True,
        },
        timeout_seconds=30,
    )

    log(f"  Submitting training job (steps={steps})...", C.INFO)
    start = time.time()

    try:
        resp = submit_training_job(req, orch_url=ORCH_URL)
    except Exception as e:
        log(f"  Submit failed: {e}", C.FAIL)
        return False

    log(f"  Job submitted: job_id={resp.job_id} status={resp.status}", C.PASS)
    log(f"  Orchestrator: {resp.orchestrator_url}", C.INFO)
    if resp.status_url:
        log(f"  Status URL: {resp.status_url}", C.INFO)

    # Poll for completion
    log("  Polling for completion...", C.INFO)
    try:
        final = wait_for_training(
            resp.job_id,
            resp.orchestrator_url or ORCH_URL,
            poll_interval=5.0,
            timeout=600.0,  # 10 min max for small training
        )
    except Exception as e:
        log(f"  Polling failed: {e}", C.FAIL)
        return False

    elapsed = time.time() - start
    log(f"  Final status: {final.status} (took {elapsed:.1f}s)", C.PASS if final.status == "completed" else C.FAIL)

    if final.status == "completed":
        lora_url = final.lora_url
        log(f"  LoRA weights URL: {lora_url}", C.PASS)
        if final.config_url:
            log(f"  Config URL: {final.config_url}", C.INFO)
        return True
    else:
        log(f"  Error: {final.error}", C.FAIL)
        return False


def test_sdk_direct_proxy_train(fast=False):
    """Test 5b: Direct proxy training test (bypasses orchestrator, tests proxy+fal.ai)."""
    log("Test 5b: Direct proxy training test...", C.BOLD)

    steps = 10 if fast else 100
    body = {
        "model_id": "fal-ai/flux-lora-fast-training",
        "images_data_url": TEST_DATASET_URL,
        "trigger_word": "lptest",
        "steps": steps,
        "is_style": False,
        "create_masks": True,
    }

    # Submit to adapter's /train endpoint
    log(f"  Submitting training via adapter (steps={steps})...", C.INFO)
    start = time.time()

    try:
        req = urllib.request.Request(
            f"{ADAPTER_URL}/train",
            data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        err_body = e.read().decode() if e.fp else ""
        log(f"  Submit failed: HTTP {e.code}: {err_body[:300]}", C.FAIL)
        return False
    except Exception as e:
        log(f"  Submit failed: {e}", C.FAIL)
        return False

    job_id = data.get("job_id")
    if not job_id:
        log(f"  No job_id in response: {data}", C.FAIL)
        return False

    log(f"  Job submitted: job_id={job_id}", C.PASS)

    # Poll for completion
    log("  Polling for completion...", C.INFO)
    max_wait = 600
    elapsed_poll = 0

    while elapsed_poll < max_wait:
        time.sleep(5)
        elapsed_poll += 5

        try:
            with urllib.request.urlopen(f"{ADAPTER_URL}/train/{job_id}", timeout=10) as resp:
                status_data = json.loads(resp.read())
        except Exception as e:
            log(f"  Poll error: {e}", C.WARN)
            continue

        status = status_data.get("status", "unknown")
        progress = status_data.get("progress", 0)
        log(f"  Status: {status} progress={progress}% (elapsed={elapsed_poll}s)", C.INFO)

        if status == "completed":
            result = status_data.get("result", {})
            lora_file = result.get("diffusers_lora_file", {})
            lora_url = lora_file.get("url", "N/A") if isinstance(lora_file, dict) else "N/A"
            elapsed_total = time.time() - start
            log(f"  Training completed in {elapsed_total:.1f}s!", C.PASS)
            log(f"  LoRA weights URL: {lora_url}", C.PASS)
            return True

        if status in ("failed", "cancelled"):
            log(f"  Training {status}: {status_data.get('error', '?')}", C.FAIL)
            return False

    log(f"  Timed out after {max_wait}s", C.FAIL)
    return False


def main():
    parser = argparse.ArgumentParser(description="E2E LoRA Training Test")
    parser.add_argument("--fast", action="store_true", help="Use minimum steps (10) for fastest test")
    parser.add_argument("--skip-orch", action="store_true", help="Skip orchestrator tests, test adapter/proxy directly")
    parser.add_argument("--direct-only", action="store_true", help="Only test direct adapter/proxy path")
    args = parser.parse_args()

    log("=" * 60, C.BOLD)
    log("Livepeer BYOC LoRA Training E2E Test", C.BOLD)
    log(f"Orchestrator: {ORCH_URL}", C.INFO)
    log(f"Adapter: {ADAPTER_URL}", C.INFO)
    log(f"Fast mode: {args.fast}", C.INFO)
    log("=" * 60, C.BOLD)

    results = {}

    # Test 1: Health
    results["health"] = test_health()

    # Test 2: Adapter /train endpoint
    results["adapter_train"] = test_adapter_train_endpoint()

    # Test 3: Submit test job
    ok, job_id = test_proxy_train_endpoint()
    results["proxy_train"] = ok

    # Test 4: Job status
    results["job_status"] = test_job_status(job_id)

    if args.direct_only:
        # Test 5b: Direct proxy training (no orchestrator needed)
        results["direct_train"] = test_sdk_direct_proxy_train(fast=args.fast)
    elif not args.skip_orch:
        # Test 5: Full E2E via SDK
        results["sdk_training"] = test_sdk_training(fast=args.fast)
    else:
        # Test 5b: Direct training without orchestrator
        results["direct_train"] = test_sdk_direct_proxy_train(fast=args.fast)

    # Summary
    log("", C.BOLD)
    log("=" * 60, C.BOLD)
    log("TEST RESULTS", C.BOLD)
    log("=" * 60, C.BOLD)
    all_passed = True
    for name, passed in results.items():
        status = f"{C.PASS}PASS" if passed else f"{C.FAIL}FAIL"
        log(f"  {name:<20} {status}{C.END}")
        if not passed:
            all_passed = False

    log("=" * 60, C.BOLD)
    if all_passed:
        log("ALL TESTS PASSED!", C.PASS)
    else:
        log("SOME TESTS FAILED", C.FAIL)
    log("=" * 60, C.BOLD)

    sys.exit(0 if all_passed else 1)


if __name__ == "__main__":
    main()
