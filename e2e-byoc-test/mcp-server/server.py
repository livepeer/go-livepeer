#!/usr/bin/env python3
"""
Livepeer BYOC MCP Server

Exposes Livepeer network AI capabilities (image, video, music generation)
as MCP tools that any MCP-compatible agent (Claude Code, Cursor, etc.) can use.

The server talks to a Livepeer BYOC orchestrator deployed on GCE.

Usage:
    # Stdio mode (for Claude Code):
    python server.py

    # With custom orchestrator URL:
    ORCH_URL=https://34.134.195.88:8935 python server.py
"""

import base64
import json
import os
import ssl
import subprocess
import time
import urllib.request
import urllib.error
from datetime import datetime

from mcp.server.fastmcp import FastMCP

# ---- Config ----
ORCH_URL = os.environ.get("ORCH_URL", "https://34.134.195.88:8935")
ADAPTER_URL = os.environ.get("ADAPTER_URL", "http://34.134.195.88:9090")
OUTPUT_DIR = os.environ.get("LIVEPEER_OUTPUT_DIR", os.path.join(os.getcwd(), "livepeer-output"))

os.makedirs(OUTPUT_DIR, exist_ok=True)

_ssl_ctx = ssl.create_default_context()
_ssl_ctx.check_hostname = False
_ssl_ctx.verify_mode = ssl.CERT_NONE

# ---- Helpers ----

def _livepeer_header(capability: str, timeout: int = 300) -> str:
    return base64.b64encode(json.dumps({
        "request": json.dumps({"run": "echo"}),
        "capability": capability,
        "timeout_seconds": timeout,
    }).encode()).decode()


def _call_orch(capability: str, body: dict, timeout: int = 300) -> dict:
    """Call the orchestrator's BYOC process endpoint."""
    url = f"{ORCH_URL}/process/request/{capability}"
    req = urllib.request.Request(url, data=json.dumps(body).encode(), headers={
        "Content-Type": "application/json",
        "Livepeer": _livepeer_header(capability, timeout),
    })
    with urllib.request.urlopen(req, timeout=timeout, context=_ssl_ctx) as resp:
        return json.loads(resp.read())


def _call_orch_train(capability: str, body: dict, timeout: int = 30) -> dict:
    """Submit a training job to the orchestrator's BYOC training endpoint."""
    url = f"{ORCH_URL}/process/train/{capability}"
    req = urllib.request.Request(url, data=json.dumps(body).encode(), headers={
        "Content-Type": "application/json",
        "Livepeer": _livepeer_header(capability, timeout),
    })
    with urllib.request.urlopen(req, timeout=timeout, context=_ssl_ctx) as resp:
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


def _extract_image_url(result: dict):
    if "images" in result and result["images"]:
        img = result["images"][0]
        return img.get("url")
    if "image_url" in result:
        return result["image_url"]
    if "url" in result:
        return result["url"]
    return None


def _extract_image_base64(result: dict):
    if "images" in result and result["images"]:
        img = result["images"][0]
        if "base64" in img:
            mime = img.get("mime_type", "image/png")
            ext = "png"
            if "webp" in mime: ext = "webp"
            elif "jpeg" in mime or "jpg" in mime: ext = "jpg"
            return img["base64"], ext
    return None


def _extract_video_url(result: dict):
    if "video" in result:
        vid = result["video"]
        return vid.get("url") if isinstance(vid, dict) else vid
    if "video_url" in result:
        return result["video_url"]
    if "url" in result:
        return result["url"]
    if "output" in result:
        out = result["output"]
        if isinstance(out, dict):
            return out.get("video", {}).get("url") if isinstance(out.get("video"), dict) else out.get("video") or out.get("url")
        if isinstance(out, str):
            return out
    return None


def _extract_audio_url(result: dict):
    if "audio" in result:
        return result["audio"].get("url") if isinstance(result["audio"], dict) else result["audio"]
    if "audio_file" in result:
        return result["audio_file"].get("url") if isinstance(result["audio_file"], dict) else result["audio_file"]
    if "url" in result:
        return result["url"]
    return None


def _guess_image_ext(url: str) -> str:
    if ".webp" in url: return "webp"
    if ".jpg" in url or ".jpeg" in url: return "jpg"
    return "png"


# ---- MCP Server ----

mcp = FastMCP(
    "Livepeer BYOC",
    instructions="AI media generation via the Livepeer decentralized network. "
                 "Generate images, videos, and music using models like Recraft, "
                 "Gemini, LTX, Lucy, Kling, and Beatoven.",
)


@mcp.tool()
def list_capabilities() -> str:
    """List all AI capabilities currently registered on the Livepeer network.

    Returns the available models for image generation, video generation,
    and music generation, along with their capacity.
    """
    try:
        with urllib.request.urlopen(f"{ADAPTER_URL}/capabilities", timeout=10) as r:
            data = json.loads(r.read())
            caps = data.get("capabilities", [])
            lines = ["Available Livepeer Network Capabilities:", ""]
            for c in caps:
                name = c.get("name", "?")
                model = c.get("model_id", "?")
                cap = c.get("capacity", "?")
                lines.append(f"  {name:<20} model={model}  capacity={cap}")
            lines.append(f"\nOrchestrator: {ORCH_URL}")
            return "\n".join(lines)
    except Exception as e:
        return f"Error listing capabilities: {e}"


@mcp.tool()
def generate_image(prompt: str, model: str = "nano-banana", image_url: str = "") -> str:
    """Generate an image using AI models on the Livepeer network.

    Args:
        prompt: Text description of the image to generate.
        model: Model to use. Text-to-image: nano-banana, recraft-v4, gemini-image,
               flux-schnell, flux-dev, flux-pro, qwen-image, fast-lcm.
               Image editing (requires image_url): kontext-edit, reve-edit,
               topaz-upscale, bg-remove. Default: nano-banana.
        image_url: URL of input image for editing models (kontext-edit, reve-edit,
                   topaz-upscale, bg-remove). Leave empty for text-to-image.

    Returns the image file path. The caller should use the Read tool on the
    returned file path to view the image inline.
    """
    body = {"prompt": prompt, "num_images": 1}
    if image_url:
        body["image_url"] = image_url

    start = time.time()
    try:
        result = _call_orch(model, body)
    except urllib.error.HTTPError as e:
        body = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {body[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start

    # Try base64 data first
    b64_data = _extract_image_base64(result)
    if b64_data:
        raw_b64, ext = b64_data
        filepath = _save_base64(raw_b64, ext)
        return (
            f"Image generated in {elapsed:.1f}s using {model}\n"
            f"Saved to: {filepath}\n"
            f"To view inline: use the Read tool on {filepath}"
        )

    # Fall back to URL — download it
    image_url = _extract_image_url(result)
    if image_url:
        ext = _guess_image_ext(image_url)
        try:
            filepath = _download_file(image_url, ext)
            return (
                f"Image generated in {elapsed:.1f}s using {model}\n"
                f"URL: {image_url}\n"
                f"Saved to: {filepath}\n"
                f"To view inline: use the Read tool on {filepath}"
            )
        except Exception as dl_err:
            return (
                f"Image generated in {elapsed:.1f}s using {model}\n"
                f"URL: {image_url}\n"
                f"Download failed: {dl_err}"
            )

    return f"Image generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"


@mcp.tool()
def generate_video(prompt: str, model: str = "ltx-t2v-23", image_url: str = "") -> str:
    """Generate a video using AI models on the Livepeer network.

    Args:
        prompt: Text description of the video to generate.
        model: Model to use. Options: ltx-t2v-23 (text-to-video), ltx-i2v (image-to-video),
               lucy-i2v (high quality i2v), kling-i2v (highest quality i2v),
               wan-v2v (video-to-video). Default: ltx-t2v-23.
        image_url: Optional image URL for image-to-video models. Leave empty for text-to-video.

    Returns the video URL and local file path. The caller can use 'open <path>' to play it.
    """
    if image_url and model == "ltx-t2v-23":
        model = "ltx-i2v"

    body = {"prompt": prompt}
    if image_url:
        body["image_url"] = image_url

    start = time.time()
    try:
        result = _call_orch(model, body)
    except urllib.error.HTTPError as e:
        body_text = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {body_text[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start
    video_url = _extract_video_url(result)

    if not video_url:
        return f"Video generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"

    filepath = None
    try:
        ext = "webm" if ".webm" in video_url else "mp4"
        filepath = _download_file(video_url, ext)
    except Exception as e:
        filepath = f"(download failed: {e})"

    return (
        f"Video generated in {elapsed:.1f}s using {model}\n"
        f"URL: {video_url}\n"
        f"Local file: {filepath}\n"
        f"To watch: open {filepath}"
    )


@mcp.tool()
def generate_music(prompt: str, model: str = "beatoven-music", duration: int = 15) -> str:
    """Generate music or speech audio using AI models on the Livepeer network.

    Args:
        prompt: Text description of music to generate, or text to speak for TTS models.
        model: Model to use. Options: beatoven-music (background music),
               chatterbox-tts (text-to-speech), lux-tts (expressive TTS).
               Default: beatoven-music.
        duration: Duration in seconds (for music models). Default: 15.

    Returns the audio URL and local file path.
    """
    if model in ("chatterbox-tts", "lux-tts"):
        body = {"text": prompt}
    else:
        body = {"prompt": prompt, "duration": duration}

    start = time.time()
    try:
        result = _call_orch(model, body)
    except urllib.error.HTTPError as e:
        body_text = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {body_text[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start
    audio_url = _extract_audio_url(result)

    if not audio_url:
        return f"Music generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"

    filepath = None
    try:
        filepath = _download_file(audio_url, "mp3")
    except Exception:
        filepath = "(download failed)"

    return (
        f"Music generated in {elapsed:.1f}s using {model}\n"
        f"URL: {audio_url}\n"
        f"Local file: {filepath}\n"
        f"To listen: open {filepath}"
    )


@mcp.tool()
def train_lora(
    images_data_url: str,
    model: str = "fal-ai/flux-lora-fast-training",
    capability: str = "flux-lora-training",
    trigger_word: str = "ohwx",
    steps: int = 1000,
    is_style: bool = False,
) -> str:
    """Submit a LoRA fine-tuning training job on the Livepeer network.

    Args:
        images_data_url: URL to a ZIP file containing training images (min 4, recommended 10-50).
        model: fal.ai training model ID. Default: fal-ai/flux-lora-fast-training.
        capability: Livepeer capability name. Default: flux-lora-training.
        trigger_word: Trigger word for the trained LoRA. Default: ohwx.
        steps: Number of training steps. Default: 1000.
        is_style: True for style training, False for subject training.

    Returns job_id and status_url for polling. Training runs async (minutes to hours).
    """
    body = {
        "model_id": model,
        "params": {
            "images_data_url": images_data_url,
            "trigger_word": trigger_word,
            "steps": steps,
            "is_style": is_style,
            "create_masks": not is_style,
        },
    }

    start = time.time()
    try:
        result = _call_orch_train(capability, body)
    except urllib.error.HTTPError as e:
        err_body = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {err_body[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start
    job_id = result.get("job_id", "unknown")
    status = result.get("status", "unknown")

    return (
        f"Training job submitted in {elapsed:.1f}s\n"
        f"Job ID: {job_id}\n"
        f"Status: {status}\n"
        f"Model: {model}\n"
        f"Trigger word: {trigger_word}\n"
        f"Steps: {steps}\n"
        f"Use check_training_status with job_id to monitor progress."
    )


@mcp.tool()
def check_training_status(job_id: str) -> str:
    """Check the status of a training job.

    Args:
        job_id: The job ID returned by train_lora.

    Returns current status, progress, and result if completed.
    """
    try:
        url = f"{ORCH_URL}/process/job/{job_id}"
        req = urllib.request.Request(url, headers={"Accept": "application/json"})
        with urllib.request.urlopen(req, timeout=10, context=_ssl_ctx) as resp:
            data = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return f"Job {job_id} not found"
        return f"Error: HTTP {e.code}"
    except Exception as e:
        return f"Error: {e}"

    status = data.get("status", "unknown")
    progress = data.get("progress", 0)
    lines = [
        f"Job ID: {job_id}",
        f"Status: {status}",
        f"Progress: {progress}%",
        f"Model: {data.get('model_id', '?')}",
    ]

    if status == "completed":
        result = data.get("result", {})
        lora_file = result.get("diffusers_lora_file", {})
        lora_url = lora_file.get("url", "N/A") if isinstance(lora_file, dict) else "N/A"
        lines.append(f"LoRA weights URL: {lora_url}")

        config_file = result.get("config_file", {})
        config_url = config_file.get("url", "N/A") if isinstance(config_file, dict) else "N/A"
        lines.append(f"Config URL: {config_url}")

    if status == "failed":
        lines.append(f"Error: {data.get('error', 'unknown')}")

    return "\n".join(lines)


@mcp.tool()
def generate_with_lora(
    prompt: str,
    lora_url: str,
    model: str = "recraft-v4",
    lora_scale: float = 1.0,
) -> str:
    """Generate an image using a trained LoRA model.

    Args:
        prompt: Text description (include the trigger word from training).
        lora_url: URL to the .safetensors LoRA weights file.
        model: Base model capability. Default: recraft-v4.
        lora_scale: LoRA application strength (0.0-2.0). Default: 1.0.

    Returns the generated image path.
    """
    body = {
        "prompt": prompt,
        "loras": [{"path": lora_url, "scale": lora_scale}],
        "num_images": 1,
    }

    start = time.time()
    try:
        result = _call_orch(model, body)
    except urllib.error.HTTPError as e:
        err_body = e.read().decode() if e.fp else ""
        return f"Error: HTTP {e.code} - {err_body[:300]}"
    except Exception as e:
        return f"Error: {e}"

    elapsed = time.time() - start

    image_url = _extract_image_url(result)
    if image_url:
        ext = _guess_image_ext(image_url)
        try:
            filepath = _download_file(image_url, ext)
            return (
                f"Image generated with LoRA in {elapsed:.1f}s using {model}\n"
                f"URL: {image_url}\n"
                f"Saved to: {filepath}\n"
                f"To view inline: use the Read tool on {filepath}"
            )
        except Exception as dl_err:
            return f"Image generated with LoRA in {elapsed:.1f}s\nURL: {image_url}\nDownload failed: {dl_err}"

    return f"Generated in {elapsed:.1f}s but could not extract URL.\nRaw: {json.dumps(result)[:500]}"


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
