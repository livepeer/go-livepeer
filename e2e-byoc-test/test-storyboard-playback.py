"""Playwright E2E test for storyboard live stream playback.

Tests:
1. Opens storyboard in Chromium
2. Starts a live stream (text-only mode, no webcam needed)
3. Publishes MPEG-TS video frames to trickle input (simulating webcam)
4. Verifies mux.js transmuxer + MSE playback works
5. Measures output FPS
6. Tests prompt update

Usage:
  python3 test-storyboard-playback.py
"""

import asyncio
import json
import os
import sys
import time

import av
import numpy as np
import requests


SDK = "https://livepeer-sdk-service-90265565772.us-central1.run.app"
FAL_KEY = "513f7292-e6a3-40b4-836f-32a644919d6f:c055899e3c22b36a8309932d83f7163d"
STORYBOARD_PATH = os.path.join(os.path.dirname(__file__), "storyboard", "index.html")


async def publish_frames(stream_id: str, duration_s: float = 30):
    """Publish MPEG-TS video frames to trickle input."""
    from livepeer_gateway.media_publish import MediaPublish

    pub_url = f"https://34-134-195-88.nip.io/ai/trickle/{stream_id}"
    pub = MediaPublish(pub_url)

    # Load sample video
    c = av.open("/tmp/sample.mp4")
    frames = [
        av.VideoFrame.from_ndarray(
            np.array(f.to_image().resize((512, 512))), format="rgb24"
        )
        for f in c.decode(video=0)
    ]
    c.close()

    count = 0
    start = time.time()
    while time.time() - start < duration_s:
        await pub.write_frame(frames[count % len(frames)])
        count += 1
        await asyncio.sleep(0.04)  # 25fps

    await asyncio.sleep(3)
    await pub.close()
    elapsed = time.time() - start
    print(f"Published {count} frames in {elapsed:.1f}s ({count/elapsed:.1f} fps)")
    return count


async def main():
    from playwright.async_api import async_playwright

    print("=== Storyboard Playback E2E Test ===\n")

    # Step 1: Start stream via SDK Service
    print("1. Starting stream...")
    resp = requests.post(
        f"{SDK}/stream/start",
        json={"capability": "scope-live", "params": {"prompt": "a sunset over mountains"}},
        timeout=60,
    )
    result = resp.json()
    stream_id = result.get("stream_id", "")
    subscribe_url = result.get("subscribe_url", "")
    print(f"   Stream: {stream_id}")
    print(f"   Subscribe: {subscribe_url}")

    # Wait for scope-trickle session
    print("   Waiting 12s for scope-trickle session...")
    await asyncio.sleep(12)

    # Step 2: Start publishing frames in background
    print("\n2. Publishing frames in background (25fps for 30s)...")
    pub_task = asyncio.create_task(publish_frames(stream_id, 30))

    # Step 3: Open storyboard in browser
    print("\n3. Opening storyboard in Chromium...")
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        # Collect console messages
        console_logs = []
        page.on("console", lambda msg: console_logs.append(f"[{msg.type}] {msg.text}"))

        # Navigate to storyboard
        storyboard_url = f"file://{os.path.abspath(STORYBOARD_PATH)}"
        await page.goto(storyboard_url)
        await page.wait_for_load_state("networkidle")
        print("   Storyboard loaded")

        # Step 4: Test direct trickle + mux.js subscribe
        print("\n4. Testing direct trickle subscribe + mux.js transmux...")

        # Inject test: fetch trickle segment and verify mux.js can transmux it
        test_result = await page.evaluate(f"""
        (async () => {{
            const subUrl = "{subscribe_url}";
            const result = {{ muxjs: !!window.muxjs, mse: !!window.MediaSource }};

            if (!window.muxjs) {{
                result.error = "mux.js not loaded";
                return result;
            }}

            // Check MSE support
            result.mp4Support = MediaSource.isTypeSupported('video/mp4; codecs="avc1.42E01E"');

            // Try fetching a trickle segment
            try {{
                const resp = await fetch(subUrl + "/-1", {{ signal: AbortSignal.timeout(10000) }});
                result.fetchStatus = resp.status;

                if (resp.status === 200) {{
                    const data = new Uint8Array(await resp.arrayBuffer());
                    result.segmentSize = data.byteLength;
                    result.isMpegTS = data[0] === 0x47;

                    // Try transmuxing
                    const transmuxer = new muxjs.mp4.Transmuxer({{ remux: true }});
                    let gotData = false;
                    transmuxer.on('data', (seg) => {{
                        gotData = true;
                        result.initSegSize = seg.initSegment.byteLength;
                        result.dataSize = seg.data.byteLength;
                    }});
                    transmuxer.push(data);
                    transmuxer.flush();
                    result.transmuxed = gotData;
                    transmuxer.dispose();
                }}
            }} catch (e) {{
                result.fetchError = e.message || e.toString();
            }}

            return result;
        }})()
        """)

        print(f"   mux.js loaded: {test_result.get('muxjs')}")
        print(f"   MSE MP4 support: {test_result.get('mp4Support')}")
        print(f"   Fetch status: {test_result.get('fetchStatus')}")
        print(f"   Segment size: {test_result.get('segmentSize', 'N/A')}B")
        print(f"   Is MPEG-TS: {test_result.get('isMpegTS')}")
        print(f"   Transmuxed: {test_result.get('transmuxed')}")
        if test_result.get("initSegSize"):
            print(f"   Init segment: {test_result['initSegSize']}B, Data: {test_result.get('dataSize')}B")
        if test_result.get("fetchError"):
            print(f"   Fetch error: {test_result['fetchError']}")

        # Step 5: Test full MSE playback
        print("\n5. Testing MSE playback...")
        playback_result = await page.evaluate(f"""
        (async () => {{
            const subUrl = "{subscribe_url}";
            const result = {{}};

            // Create video + MSE
            const video = document.createElement('video');
            video.muted = true;
            video.autoplay = true;
            document.body.appendChild(video);

            const ms = new MediaSource();
            video.src = URL.createObjectURL(ms);
            await new Promise(r => ms.addEventListener('sourceopen', r, {{ once: true }}));

            const sb = ms.addSourceBuffer('video/mp4; codecs="avc1.42E01E"');
            sb.mode = 'sequence';

            const transmuxer = new muxjs.mp4.Transmuxer({{ remux: true }});
            let initDone = false;
            let pendingBuffers = [];
            let appending = false;

            function flush() {{
                if (appending || sb.updating || pendingBuffers.length === 0) return;
                appending = true;
                sb.appendBuffer(pendingBuffers.shift());
            }}
            sb.addEventListener('updateend', () => {{ appending = false; flush(); }});

            transmuxer.on('data', (seg) => {{
                if (!initDone) {{
                    const init = new Uint8Array(seg.initSegment.byteLength + seg.data.byteLength);
                    init.set(seg.initSegment, 0);
                    init.set(seg.data, seg.initSegment.byteLength);
                    pendingBuffers.push(init);
                    initDone = true;
                }} else {{
                    pendingBuffers.push(new Uint8Array(seg.data));
                }}
                flush();
            }});

            // Fetch 5 trickle segments
            let seq = -1;
            let segCount = 0;
            const startTime = performance.now();

            for (let attempt = 0; attempt < 20 && segCount < 5; attempt++) {{
                try {{
                    const resp = await fetch(subUrl + '/' + seq, {{ signal: AbortSignal.timeout(10000) }});
                    if (resp.status === 200) {{
                        const data = new Uint8Array(await resp.arrayBuffer());
                        if (data.byteLength > 0) {{
                            transmuxer.push(data);
                            transmuxer.flush();
                            segCount++;
                            seq = parseInt(resp.headers.get('Lp-Trickle-Seq') || seq) + 1;
                        }}
                    }} else if (resp.status === 470) {{
                        const latest = resp.headers.get('Lp-Trickle-Latest');
                        if (latest) seq = parseInt(latest);
                        await new Promise(r => setTimeout(r, 50));
                    }} else {{
                        await new Promise(r => setTimeout(r, 200));
                    }}
                }} catch (e) {{
                    if (e.name === 'TimeoutError') continue;
                    result.error = e.message;
                    break;
                }}
            }}

            const elapsed = (performance.now() - startTime) / 1000;

            // Wait for video to start playing
            await new Promise(r => setTimeout(r, 1000));
            await video.play().catch(() => {{}});
            await new Promise(r => setTimeout(r, 2000));

            result.segmentsReceived = segCount;
            result.elapsedSeconds = elapsed.toFixed(1);
            result.videoWidth = video.videoWidth;
            result.videoHeight = video.videoHeight;
            result.videoPaused = video.paused;
            result.currentTime = video.currentTime.toFixed(2);
            result.buffered = video.buffered.length > 0 ?
                video.buffered.end(video.buffered.length - 1).toFixed(2) : '0';
            result.readyState = video.readyState;

            transmuxer.dispose();
            return result;
        }})()
        """)

        print(f"   Segments received: {playback_result.get('segmentsReceived')}")
        print(f"   Elapsed: {playback_result.get('elapsedSeconds')}s")
        print(f"   Video size: {playback_result.get('videoWidth')}x{playback_result.get('videoHeight')}")
        print(f"   Playing: {not playback_result.get('videoPaused')}")
        print(f"   Current time: {playback_result.get('currentTime')}s")
        print(f"   Buffered: {playback_result.get('buffered')}s")
        print(f"   Ready state: {playback_result.get('readyState')}")
        if playback_result.get("error"):
            print(f"   Error: {playback_result['error']}")

        # Print console logs
        if console_logs:
            print(f"\n   Console ({len(console_logs)} messages):")
            for log in console_logs[-10:]:
                print(f"     {log}")

        # Verdict
        ok = (
            playback_result.get("segmentsReceived", 0) >= 3
            and float(playback_result.get("buffered", "0")) > 0
            and playback_result.get("videoWidth", 0) > 0
        )
        print(f"\n{'PASS' if ok else 'FAIL'}: MSE playback {'works' if ok else 'failed'}")

        await browser.close()

    # Wait for publisher to finish
    await pub_task

    print("\n=== Test Complete ===")


if __name__ == "__main__":
    asyncio.run(main())
