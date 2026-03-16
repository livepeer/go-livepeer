#!/usr/bin/env python3
"""
Livepeer BYOC MCP Server — SDK Service backend

Routes all requests through the hosted SDK Service (Cloud Run) instead of
calling the orchestrator directly. This tests the full SDK path:

    MCP Client → SDK Service (Cloud Run) → Orchestrator → Adapter → Proxy → Provider

Usage:
    # Stdio mode (for Claude Code):
    python server-sdk.py

    # With custom SDK service URL:
    SDK_URL=https://livepeer-sdk-service-90265565772.us-central1.run.app python server-sdk.py
"""

import base64
import json
import os
import subprocess
import time
import urllib.request
import urllib.error
from datetime import datetime

from mcp.server.fastmcp import FastMCP

# ---- Config ----
SDK_URL = os.environ.get("SDK_URL", "https://livepeer-sdk-service-90265565772.us-central1.run.app")
OUTPUT_DIR = os.environ.get("LIVEPEER_OUTPUT_DIR",
    os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "livepeer-output"))

os.makedirs(OUTPUT_DIR, exist_ok=True)

# ---- Helpers ----

def _sdk_request(path: str, body: dict = None, timeout: int = 300) -> dict:
    """Call the SDK Service REST API."""
    url = f"{SDK_URL}{path}"
    if body is not None:
        data = json.dumps(body).encode()
        req = urllib.request.Request(url, data=data, headers={
            "Content-Type": "application/json",
        })
    else:
        req = urllib.request.Request(url)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read())


def _download_file(url: str, ext: str) -> str:
    """Download a URL to a local file in OUTPUT_DIR."""
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"livepeer_{ts}.{ext}"
    filepath = os.path.join(OUTPUT_DIR, filename)
    urllib.request.urlretrieve(url, filepath)
    return filepath


def _save_base64(data: str, ext: str) -> str:
    """Save base64 data to a local file."""
    if data.startswith("data:"):
        data = data.split(",", 1)[1] if "," in data else data
    raw = base64.b64decode(data)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"livepeer_{ts}.{ext}"
    filepath = os.path.join(OUTPUT_DIR, filename)
    with open(filepath, "wb") as f:
        f.write(raw)
    return filepath


def _guess_image_ext(url: str) -> str:
    if ".webp" in url: return "webp"
    if ".jpg" in url or ".jpeg" in url: return "jpg"
    return "png"


# ---- MCP Server ----

mcp = FastMCP(
    "Livepeer BYOC (SDK)",
    instructions="AI media generation via the Livepeer network, routed through the hosted SDK Service. "
                 "Generate images, videos, and audio using 20+ models.",
)


@mcp.tool()
def list_capabilities() -> str:
    """List all AI capabilities available on the Livepeer network (via SDK Service).

    Returns the available models for image, video, and audio generation.
    """
    try:
        caps = _sdk_request("/capabilities")
        lines = ["Available Livepeer Network Capabilities (via SDK Service):", ""]
        for c in caps:
            name = c.get("name", "?")
            model = c.get("model_id", "?")
            cap = c.get("capacity", "?")
            source = c.get("source", "")
            lines.append(f"  {name:<20} model={model}  capacity={cap}")
        lines.append(f"\nSDK Service: {SDK_URL}")
        return "\n".join(lines)
    except Exception as e:
        return f"Error listing capabilities: {e}"


@mcp.tool()
def generate_image(prompt: str, model: str = "nano-banana", image_url: str = "") -> str:
    """Generate an image using AI models on the Livepeer network (via SDK Service).

    Args:
        prompt: Text description of the image to generate.
        model: Model to use. Text-to-image: nano-banana, recraft-v4, gemini-image,
               flux-schnell, flux-dev, flux-pro, qwen-image, fast-lcm.
               Image editing (requires image_url): kontext-edit, reve-edit,
               topaz-upscale, bg-remove. Default: nano-banana.
        image_url: URL of input image for editing models. Leave empty for text-to-image.

    Returns the image file path.
    """
    params = {"num_images": 1}
    if image_url:
        params["image_url"] = image_url

    body = {
        "capability": model,
        "prompt": prompt,
        "params": params,
    }

    start = time.time()
    try:
        result = _sdk_request("/inference", body)
    except urllib.error.HTTPError as e:
        err = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {err[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start

    # Check for image URL in response
    img_url = result.get("image_url")
    if img_url:
        if img_url.startswith("data:"):
            # base64 data URI
            _, encoded = img_url.split(",", 1) if "," in img_url else ("", img_url)
            filepath = _save_base64(encoded, "png")
            return (
                f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                f"Saved to: {filepath}\n"
                f"To view inline: use the Read tool on {filepath}"
            )
        ext = _guess_image_ext(img_url)
        try:
            filepath = _download_file(img_url, ext)
            return (
                f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                f"URL: {img_url}\n"
                f"Saved to: {filepath}\n"
                f"To view inline: use the Read tool on {filepath}"
            )
        except Exception as dl_err:
            return (
                f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                f"URL: {img_url}\n"
                f"Download failed: {dl_err}"
            )

    # Check data field for base64 images
    data = result.get("data")
    if isinstance(data, dict):
        images = data.get("images", [])
        if images and isinstance(images[0], dict):
            b64 = images[0].get("base64")
            if b64:
                mime = images[0].get("mime_type", "image/png")
                ext = "png"
                if "webp" in mime: ext = "webp"
                elif "jpeg" in mime or "jpg" in mime: ext = "jpg"
                filepath = _save_base64(b64, ext)
                return (
                    f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                    f"Saved to: {filepath}\n"
                    f"To view inline: use the Read tool on {filepath}"
                )
            url = images[0].get("url")
            if url:
                ext = _guess_image_ext(url)
                filepath = _download_file(url, ext)
                return (
                    f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                    f"URL: {url}\n"
                    f"Saved to: {filepath}\n"
                    f"To view inline: use the Read tool on {filepath}"
                )

    # Check data.image (singular) — topaz-upscale, bg-remove etc.
    if isinstance(data, dict):
        image = data.get("image")
        if isinstance(image, dict) and image.get("url"):
            url = image["url"]
            ext = _guess_image_ext(url)
            try:
                filepath = _download_file(url, ext)
                return (
                    f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                    f"URL: {url}\n"
                    f"Saved to: {filepath}\n"
                    f"To view inline: use the Read tool on {filepath}"
                )
            except Exception as dl_err:
                return (
                    f"Image generated in {elapsed:.1f}s using {model} (via SDK)\n"
                    f"URL: {url}\n"
                    f"Download failed: {dl_err}"
                )

    return f"Image generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"


@mcp.tool()
def generate_video(prompt: str, model: str = "ltx-t2v-23", image_url: str = "", video_url: str = "") -> str:
    """Generate a video using AI models on the Livepeer network (via SDK Service).

    Args:
        prompt: Text description of the video to generate.
        model: Model to use. Options: ltx-t2v-23 (text-to-video), ltx-i2v (image-to-video),
               lucy-i2v (high quality i2v), kling-i2v (highest quality i2v),
               wan-v2v (video-to-video, requires video_url). Default: ltx-t2v-23.
        image_url: Input image URL for image-to-video models.
        video_url: Input video URL for video-to-video models (wan-v2v).

    Returns the video URL and local file path.
    """
    if image_url and model == "ltx-t2v-23":
        model = "ltx-i2v"
    if video_url and model != "wan-v2v":
        model = "wan-v2v"

    params = {}
    if image_url:
        params["image_url"] = image_url
    if video_url:
        params["video_url"] = video_url

    body = {
        "capability": model,
        "prompt": prompt,
        "params": params,
    }

    start = time.time()
    try:
        result = _sdk_request("/inference", body, timeout=300)
    except urllib.error.HTTPError as e:
        err = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {err[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start
    video_url = result.get("video_url")

    if not video_url:
        # Try data field
        data = result.get("data", {})
        if isinstance(data, dict):
            video = data.get("video", {})
            if isinstance(video, dict):
                video_url = video.get("url")
            elif isinstance(video, str):
                video_url = video
            if not video_url:
                video_url = data.get("video_url") or data.get("url")

    if not video_url:
        return f"Video generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"

    filepath = None
    try:
        ext = "webm" if ".webm" in video_url else "mp4"
        filepath = _download_file(video_url, ext)
    except Exception as e:
        filepath = f"(download failed: {e})"

    return (
        f"Video generated in {elapsed:.1f}s using {model} (via SDK)\n"
        f"URL: {video_url}\n"
        f"Local file: {filepath}\n"
        f"To watch: open {filepath}"
    )


@mcp.tool()
def generate_audio(prompt: str, model: str = "chatterbox-tts") -> str:
    """Generate audio (speech or music) using AI models on the Livepeer network (via SDK Service).

    Args:
        prompt: Text to speak (for TTS models) or description of music to generate.
        model: Model to use. Options: chatterbox-tts (text-to-speech),
               lux-tts (expressive TTS), beatoven-music (background music).
               Default: chatterbox-tts.

    Returns the audio URL and local file path.
    """
    params = {}
    if model in ("chatterbox-tts", "lux-tts"):
        params["text"] = prompt
        body = {
            "capability": model,
            "prompt": "",
            "params": params,
        }
    else:
        params["duration"] = 15
        body = {
            "capability": model,
            "prompt": prompt,
            "params": params,
        }

    start = time.time()
    try:
        result = _sdk_request("/inference", body, timeout=60)
    except urllib.error.HTTPError as e:
        err = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {err[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start
    audio_url = result.get("audio_url")

    if not audio_url:
        data = result.get("data", {})
        if isinstance(data, dict):
            audio = data.get("audio", data.get("audio_file", {}))
            if isinstance(audio, dict):
                audio_url = audio.get("url")
            elif isinstance(audio, str):
                audio_url = audio
            if not audio_url:
                audio_url = data.get("url")

    if not audio_url:
        return f"Audio generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"

    filepath = None
    try:
        ext = "wav" if ".wav" in audio_url else "mp3"
        filepath = _download_file(audio_url, ext)
    except Exception as e:
        filepath = f"(download failed: {e})"

    return (
        f"Audio generated in {elapsed:.1f}s using {model} (via SDK)\n"
        f"URL: {audio_url}\n"
        f"Local file: {filepath}\n"
        f"To listen: open {filepath}"
    )


@mcp.tool()
def view_media(file_path: str) -> str:
    """View or play a previously generated media file.

    Args:
        file_path: Path to the media file to view/play.

    For images: tells the caller to use Read tool to view inline.
    For video/audio: opens with system player (macOS 'open' command).
    """
    if not os.path.exists(file_path):
        return f"File not found: {file_path}"

    ext = file_path.rsplit(".", 1)[-1].lower()

    if ext in ("png", "jpg", "jpeg", "webp", "gif"):
        return (
            f"Image file: {file_path}\n"
            f"To view inline: use the Read tool on {file_path}"
        )

    if ext in ("mp4", "webm", "mov", "mp3", "wav", "ogg"):
        subprocess.Popen(["open", file_path], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return f"Opened {file_path} in system player."

    return f"Unknown file type: .{ext}"


if __name__ == "__main__":
    mcp.run()
