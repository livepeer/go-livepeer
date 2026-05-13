#!/usr/bin/env python3
"""
BYOC E2E Test Client

Tests the full Livepeer BYOC pipeline:
  client -> gateway -> orchestrator -> inference-adapter -> serverless-proxy -> backend

Can also test directly against the orchestrator (bypassing gateway) or adapter.

Usage:
    # Full E2E via gateway:
    python test-byoc-client.py --gateway http://localhost:9935 --capability test-inference

    # Direct to orchestrator:
    python test-byoc-client.py --orch http://localhost:8935 --capability test-inference

    # Direct to adapter (bypass livepeer entirely):
    python test-byoc-client.py --adapter http://localhost:9090

    # With fal.ai capability:
    python test-byoc-client.py --gateway http://localhost:9935 --capability flux-dev
"""

import argparse
import base64
import json
import sys
import time
import urllib.request
import urllib.error


def make_livepeer_header(capability: str, timeout: int = 30) -> str:
    """Build the base64-encoded Livepeer job request header."""
    payload = {
        "request": json.dumps({"run": "echo"}),
        "capability": capability,
        "timeout_seconds": timeout,
    }
    return base64.b64encode(json.dumps(payload).encode()).decode()


def test_health(url: str, name: str) -> bool:
    """Test a health endpoint."""
    try:
        req = urllib.request.Request(f"{url}/health")
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = resp.read().decode()
            print(f"  [OK] {name} healthy: {data[:100]}")
            return True
    except Exception as e:
        print(f"  [FAIL] {name} not healthy: {e}")
        return False


def test_direct_adapter(adapter_url: str, payload: dict) -> dict | None:
    """Send inference request directly to adapter (bypasses livepeer)."""
    print("\n--- Direct Adapter Test ---")
    try:
        data = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{adapter_url}/inference",
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            result = json.loads(resp.read().decode())
            print(f"  [OK] Status: {resp.status}")
            print(f"  Response: {json.dumps(result, indent=2)[:500]}")
            return result
    except Exception as e:
        print(f"  [FAIL] {e}")
        return None


def test_via_orchestrator(orch_url: str, capability: str, payload: dict) -> dict | None:
    """Send request directly to orchestrator's BYOC process endpoint."""
    print("\n--- Orchestrator Direct Test ---")
    try:
        data = json.dumps(payload).encode()
        livepeer_hdr = make_livepeer_header(capability)
        req = urllib.request.Request(
            f"{orch_url}/process/request/{capability}",
            data=data,
            headers={
                "Content-Type": "application/json",
                "Livepeer": livepeer_hdr,
            },
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            result = json.loads(resp.read().decode())
            print(f"  [OK] Status: {resp.status}")
            print(f"  Response: {json.dumps(result, indent=2)[:500]}")
            return result
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        print(f"  [FAIL] HTTP {e.code}: {body[:200]}")
        return None
    except Exception as e:
        print(f"  [FAIL] {e}")
        return None


def test_via_gateway(gateway_url: str, capability: str, payload: dict) -> dict | None:
    """Send request via gateway (full E2E BYOC flow)."""
    print("\n--- Full E2E Gateway Test ---")
    try:
        data = json.dumps(payload).encode()
        livepeer_hdr = make_livepeer_header(capability)
        req = urllib.request.Request(
            f"{gateway_url}/process/request/{capability}",
            data=data,
            headers={
                "Content-Type": "application/json",
                "Livepeer": livepeer_hdr,
            },
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=60) as resp:
            result = json.loads(resp.read().decode())
            print(f"  [OK] Status: {resp.status}")
            print(f"  Response: {json.dumps(result, indent=2)[:500]}")
            return result
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        print(f"  [FAIL] HTTP {e.code}: {body[:200]}")
        return None
    except Exception as e:
        print(f"  [FAIL] {e}")
        return None


def test_job_token(orch_url: str, capability: str) -> dict | None:
    """Request a job token from the orchestrator."""
    print("\n--- Job Token Test ---")
    try:
        eth_addr = base64.b64encode(
            json.dumps({"address": "0x0000000000000000000000000000000000000001"}).encode()
        ).decode()
        req = urllib.request.Request(
            f"{orch_url}/process/token",
            headers={
                "Livepeer-Eth-Address": eth_addr,
                "Livepeer-Capability": capability,
            },
        )
        with urllib.request.urlopen(req, timeout=10) as resp:
            result = json.loads(resp.read().decode())
            print(f"  [OK] Token received")
            print(f"  Token: {json.dumps(result, indent=2)[:300]}")
            return result
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        print(f"  [FAIL] HTTP {e.code}: {body[:200]}")
        return None
    except Exception as e:
        print(f"  [FAIL] {e}")
        return None


def main():
    parser = argparse.ArgumentParser(description="BYOC E2E Test Client")
    parser.add_argument("--gateway", default="http://localhost:9935", help="Gateway URL")
    parser.add_argument("--orch", default="http://localhost:8935", help="Orchestrator URL")
    parser.add_argument("--adapter", default="http://localhost:9090", help="Adapter URL")
    parser.add_argument("--capability", default="test-inference", help="Capability name")
    parser.add_argument(
        "--prompt", default="a cat wearing a top hat", help="Test prompt"
    )
    parser.add_argument(
        "--test",
        choices=["all", "health", "adapter", "orch", "gateway", "token"],
        default="all",
        help="Which test to run",
    )
    args = parser.parse_args()

    payload = {"prompt": args.prompt, "num_inference_steps": 4}
    results = {}

    print("=" * 50)
    print(" BYOC E2E Test Client")
    print(f" Capability: {args.capability}")
    print("=" * 50)

    if args.test in ("all", "health"):
        print("\n--- Health Checks ---")
        results["orch_health"] = test_health(args.orch, "Orchestrator")
        results["adapter_health"] = test_health(args.adapter, "Adapter")

    if args.test in ("all", "adapter"):
        result = test_direct_adapter(args.adapter, payload)
        results["adapter"] = result is not None

    if args.test in ("all", "token"):
        result = test_job_token(args.orch, args.capability)
        results["token"] = result is not None

    if args.test in ("all", "orch"):
        result = test_via_orchestrator(args.orch, args.capability, payload)
        results["orch"] = result is not None

    if args.test in ("all", "gateway"):
        result = test_via_gateway(args.gateway, args.capability, payload)
        results["gateway"] = result is not None

    # Summary
    print("\n" + "=" * 50)
    print(" Summary")
    print("=" * 50)
    passed = sum(1 for v in results.values() if v)
    total = len(results)
    for name, ok in results.items():
        status = "PASS" if ok else "FAIL"
        print(f"  [{status}] {name}")
    print(f"\n  {passed}/{total} tests passed")

    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()
