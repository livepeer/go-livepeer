#!/usr/bin/env python3
"""E2E test: push continuous video frames through SDK → orch → adapter → scope-trickle.

Simulates a webcam by generating colored frames and pushing them at ~2fps.
Reads output frames from the SDK trickle proxy.

Usage:
    python3 test-scope-trickle.py [--frames 30] [--fps 2]
"""

import argparse
import base64
import io
import json
import os
import sys
import time
import threading
import urllib.request
import urllib.error

SDK = os.environ.get("SDK_SERVICE", "https://livepeer-sdk-service-90265565772.us-central1.run.app")


def sdk_post(path, body=None, timeout=120):
    url = f"{SDK}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, json.loads(r.read())
    except urllib.error.HTTPError as e:
        return e.code, {"error": e.read().decode()[:300]}
    except Exception as e:
        return 0, {"error": str(e)}


def sdk_post_binary(path, data, content_type="image/jpeg", timeout=30):
    url = f"{SDK}{path}"
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Content-Type", content_type)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return 0, str(e).encode()


def sdk_get_binary(path, timeout=30):
    url = f"{SDK}{path}"
    req = urllib.request.Request(url)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            headers = dict(r.headers)
            return r.status, r.read(), headers
    except urllib.error.HTTPError as e:
        return e.code, e.read(), {}
    except Exception as e:
        return 0, str(e).encode(), {}


def make_frame(i, w=512, h=512):
    """Generate a colored JPEG frame with frame number."""
    from PIL import Image, ImageDraw
    r = (50 + i * 7) % 256
    g = (100 + i * 13) % 256
    b = (200 - i * 5) % 256
    img = Image.new("RGB", (w, h), (r, g, b))
    draw = ImageDraw.Draw(img)
    # Grid lines
    for x in range(0, w, 32):
        draw.line([(x, 0), (x, h)], fill=(255, 255, 255), width=1)
    for y in range(0, h, 32):
        draw.line([(0, y), (w, y)], fill=(255, 255, 255), width=1)
    # Frame number
    draw.text((10, 10), f"Frame {i}", fill=(255, 255, 0))
    buf = io.BytesIO()
    img.save(buf, "JPEG", quality=80)
    return buf.getvalue()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--frames", type=int, default=30, help="Number of frames to send")
    parser.add_argument("--fps", type=float, default=2, help="Frames per second")
    parser.add_argument("--prompt", default="cyberpunk neon city", help="Prompt for scope")
    parser.add_argument("--save-output", action="store_true", help="Save output frames to disk")
    args = parser.parse_args()

    print(f"SDK: {SDK}")
    print(f"Frames: {args.frames}, FPS: {args.fps}, Prompt: {args.prompt}")
    print()

    # Step 1: Start stream
    print("=== STEP 1: Start stream ===")
    status, result = sdk_post("/stream/start", {
        "capability": "scope-live",
        "params": {"prompt": args.prompt},
        "enable_video_ingress": True,
        "enable_video_egress": True,
    })
    if status != 200:
        print(f"  FAIL: {status} {result}")
        sys.exit(1)

    stream_id = result.get("stream_id", "")
    print(f"  stream_id: {stream_id}")
    print(f"  subscribe_url: {result.get('subscribe_url', 'MISSING')}")
    print(f"  publish_url: {result.get('publish_url', 'MISSING')}")
    print()

    if not stream_id:
        print("FAIL: No stream_id")
        sys.exit(1)

    # Step 2: Push frames
    print(f"=== STEP 2: Push {args.frames} frames at {args.fps} fps ===")
    pub_ok = 0
    pub_fail = 0
    interval = 1.0 / args.fps

    for i in range(args.frames):
        jpeg = make_frame(i)
        t0 = time.time()
        status, _ = sdk_post_binary(f"/stream/{stream_id}/publish?seq={i}", jpeg, timeout=10)
        elapsed = time.time() - t0

        if status == 200:
            pub_ok += 1
            marker = "OK"
        else:
            pub_fail += 1
            marker = f"FAIL({status})"

        print(f"  Frame {i:3d}: {len(jpeg):6d}B → {marker} ({elapsed:.1f}s)")

        # Rate limit
        sleep_time = interval - elapsed
        if sleep_time > 0:
            time.sleep(sleep_time)

    print(f"  Published: {pub_ok} OK, {pub_fail} failed")
    print()

    # Step 3: Read output frames
    print("=== STEP 3: Read output frames ===")
    seq = -1
    out_count = 0
    os.makedirs("/tmp/scope-output", exist_ok=True)

    for attempt in range(120):  # try for 120s
        status, data, headers = sdk_get_binary(f"/stream/{stream_id}/frame?seq={seq}", timeout=15)

        if status == 204:
            # No data yet, retry
            time.sleep(1)
            continue

        if status == 200 and len(data) > 1000:
            actual_seq = headers.get("X-Trickle-Seq", str(seq))
            out_count += 1
            print(f"  Output {out_count}: seq={actual_seq}, {len(data)}B")

            if args.save_output:
                with open(f"/tmp/scope-output/frame_{out_count:04d}.jpg", "wb") as f:
                    f.write(data)

            seq = int(actual_seq) + 1

            if out_count >= 5:
                print(f"  Got {out_count} output frames, stopping.")
                break
        elif status == 404 or status == 470:
            time.sleep(1)
            continue
        else:
            time.sleep(2)

    print(f"  Total output frames: {out_count}")
    print()

    # Step 4: Stop stream
    print("=== STEP 4: Stop stream ===")
    status, result = sdk_post(f"/stream/{stream_id}/stop", {}, timeout=10)
    print(f"  {status}: {result}")
    print()

    # Summary
    print("=== SUMMARY ===")
    print(f"  Input frames sent: {pub_ok}/{args.frames}")
    print(f"  Output frames received: {out_count}")
    if out_count > 0:
        print(f"  PASS: V2V pipeline working!")
        if args.save_output:
            print(f"  Output saved to /tmp/scope-output/")
    else:
        print(f"  FAIL: No output frames received")


if __name__ == "__main__":
    main()
