"""
Livepeer SDK Service -- FastAPI wrapper around livepeer-gateway SDK.

Exposes the SDK's BYOC inference, training, and capability listing as a
REST API so thin clients (agents, scripts, mobile apps) don't need the
SDK installed locally.

Architecture:
    Thin Client -> SDK Service (Cloud Run) -> Orchestrator -> Adapter -> Proxy -> Provider

All Livepeer protocol details (header encoding, orchestrator discovery,
SSL handling, polling) are handled by this service.
"""

from __future__ import annotations

import base64
import hashlib
import json
import logging
import os
import ssl
import time
import urllib.request
import urllib.error
from typing import Any, Optional

from fastapi import FastAPI, HTTPException, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from livepeer_gateway import (
    ByocJobRequest,
    ByocTrainingRequest,
    LivepeerGatewayError,
    NoOrchestratorAvailableError,
    submit_byoc_job,
    submit_training_job,
    get_training_status,
    wait_for_training,
    list_capabilities,
)

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("sdk-service")

# ---------------------------------------------------------------------------
# Config from environment
# ---------------------------------------------------------------------------

ORCH_URL = os.environ.get("ORCH_URL", "https://34.134.195.88:8935")
ORCH_PUBLIC_URL = os.environ.get("ORCH_PUBLIC_URL", "https://34-134-195-88.nip.io")
ADAPTER_URLS = os.environ.get(
    "ADAPTER_URLS", "http://34.134.195.88:9090,http://34.134.195.88:9091,http://34.134.195.88:9092"
).split(",")
# SSL context for self-signed orchestrator certs
_ssl_ctx = ssl.create_default_context()
_ssl_ctx.check_hostname = False
_ssl_ctx.verify_mode = ssl.CERT_NONE

app = FastAPI(
    title="Livepeer SDK Service",
    description="REST API wrapping the Livepeer Python Gateway SDK for thin-client access.",
    version="0.1.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


# ---------------------------------------------------------------------------
# Request / response models
# ---------------------------------------------------------------------------

class InferenceRequest(BaseModel):
    capability: str = Field(..., description="Capability name (e.g. 'nano-banana', 'recraft-v4')")
    prompt: str = Field("", description="Text prompt")
    image_data: Optional[str] = Field(None, description="Base64-encoded image (JPEG/PNG) for image-to-X pipelines")
    params: dict[str, Any] = Field(default_factory=dict, description="Extra params merged into payload")
    timeout: int = Field(300, description="Request timeout in seconds")


class InferenceResponse(BaseModel):
    status: str = "ok"
    capability: str
    image_url: Optional[str] = None
    video_url: Optional[str] = None
    audio_url: Optional[str] = None
    data: Any = None
    balance: Optional[str] = None
    orchestrator: Optional[str] = None
    elapsed_ms: int = 0


class TrainingSubmitRequest(BaseModel):
    capability: str = Field("flux-lora-training", description="Training capability name")
    model_id: str = Field("fal-ai/flux-lora-fast-training", description="Provider model ID")
    params: dict[str, Any] = Field(default_factory=dict, description="Training params (images_data_url, etc.)")
    wait: bool = Field(False, description="If true, poll until done (synchronous)")
    callback_url: Optional[str] = Field(None, description="Webhook URL for completion")
    timeout: int = Field(300, description="Submit timeout in seconds")


class TrainingSubmitResponse(BaseModel):
    job_id: str
    status: str
    orchestrator: Optional[str] = None
    status_url: Optional[str] = None
    result: Optional[dict] = None
    error: Optional[str] = None
    elapsed_ms: int = 0


class TrainingStatusResponse(BaseModel):
    job_id: str
    status: str
    progress: int = 0
    result: Optional[dict] = None
    error: Optional[str] = None
    model_id: Optional[str] = None
    cost: Optional[str] = None
    balance: Optional[str] = None


class CapabilityItem(BaseModel):
    name: str
    model_id: str
    capacity: int = 0
    source: str = ""


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Temp file upload/serving (for base64→URL conversion)
# ---------------------------------------------------------------------------
UPLOAD_DIR = "/tmp/livepeer-uploads"
os.makedirs(UPLOAD_DIR, exist_ok=True)


class UploadRequest(BaseModel):
    data: str = Field(..., description="Base64-encoded file data (with or without data URI prefix)")
    content_type: str = Field("application/octet-stream", description="MIME type")
    filename: str = Field("upload", description="Original filename")


@app.post("/upload")
async def upload_file(req: UploadRequest, request: Request):
    """Store base64 data and return a publicly accessible URL."""
    raw = req.data
    if raw.startswith("data:"):
        raw = raw.split(",", 1)[1] if "," in raw else raw
    try:
        file_bytes = base64.b64decode(raw)
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid base64: {e}")

    # Determine extension
    ext_map = {"video/mp4": "mp4", "video/webm": "webm", "image/png": "png",
               "image/jpeg": "jpg", "image/webp": "webp", "application/zip": "zip"}
    ext = ext_map.get(req.content_type, req.filename.rsplit(".", 1)[-1] if "." in req.filename else "bin")

    file_hash = hashlib.sha256(file_bytes[:4096]).hexdigest()[:12]
    fname = f"{file_hash}_{int(time.time())}.{ext}"
    fpath = os.path.join(UPLOAD_DIR, fname)
    with open(fpath, "wb") as f:
        f.write(file_bytes)

    # Build URL based on request host (force HTTPS for Cloud Run)
    base_url = str(request.base_url).rstrip("/")
    if base_url.startswith("http://") and "run.app" in base_url:
        base_url = base_url.replace("http://", "https://", 1)
    url = f"{base_url}/files/{fname}"
    return {"url": url, "filename": fname, "size": len(file_bytes)}


@app.get("/files/{filename}")
async def serve_file(filename: str):
    """Serve an uploaded temp file."""
    import re
    if not re.match(r'^[a-zA-Z0-9_\-.]+$', filename):
        raise HTTPException(status_code=400, detail="Invalid filename")
    fpath = os.path.join(UPLOAD_DIR, filename)
    if not os.path.exists(fpath):
        raise HTTPException(status_code=404, detail="File not found")

    ext = filename.rsplit(".", 1)[-1].lower()
    ct_map = {"mp4": "video/mp4", "webm": "video/webm", "png": "image/png",
              "jpg": "image/jpeg", "jpeg": "image/jpeg", "webp": "image/webp", "zip": "application/zip"}
    content_type = ct_map.get(ext, "application/octet-stream")

    with open(fpath, "rb") as f:
        data = f.read()
    return Response(content=data, media_type=content_type)


@app.get("/health")
async def health():
    return {"status": "ok", "orchestrator": ORCH_URL}


@app.get("/capabilities", response_model=list[CapabilityItem])
async def get_capabilities():
    """List all capabilities from all configured adapters."""
    all_caps = []
    for adapter_url in ADAPTER_URLS:
        adapter_url = adapter_url.strip()
        if not adapter_url:
            continue
        try:
            caps = list_capabilities(adapter_url)
            for c in caps:
                all_caps.append(CapabilityItem(
                    name=c.get("name", ""),
                    model_id=c.get("model_id", ""),
                    capacity=c.get("capacity", 0),
                    source=adapter_url,
                ))
        except Exception as e:
            logger.warning("Failed to list capabilities from %s: %s", adapter_url, e)
    return all_caps


@app.post("/inference", response_model=InferenceResponse)
async def inference(req: InferenceRequest):
    """Run inference through the Livepeer network."""
    t0 = time.time()
    payload = {**req.params}
    if req.prompt and "text" not in payload:
        payload["prompt"] = req.prompt
    if req.image_data:
        # Pass base64 image as data URI so providers can consume it
        if not req.image_data.startswith("data:"):
            payload["image_url"] = f"data:image/jpeg;base64,{req.image_data}"
        else:
            payload["image_url"] = req.image_data

    byoc_req = ByocJobRequest(
        capability=req.capability,
        payload=payload,
        timeout_seconds=req.timeout,
    )

    try:
        result = submit_byoc_job(byoc_req, orch_url=ORCH_URL, timeout=req.timeout)
    except NoOrchestratorAvailableError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except LivepeerGatewayError as e:
        raise HTTPException(status_code=502, detail=str(e))

    elapsed_ms = int((time.time() - t0) * 1000)
    return InferenceResponse(
        capability=req.capability,
        image_url=result.image_url,
        video_url=result.video_url,
        audio_url=result.audio_url,
        data=result.data,
        balance=result.balance,
        orchestrator=result.orchestrator_url,
        elapsed_ms=elapsed_ms,
    )


@app.post("/train", response_model=TrainingSubmitResponse)
async def train(req: TrainingSubmitRequest):
    """Submit a training job to the Livepeer network."""
    t0 = time.time()

    train_req = ByocTrainingRequest(
        capability=req.capability,
        model_id=req.model_id,
        params=req.params,
        timeout_seconds=req.timeout,
        callback_url=req.callback_url,
    )

    try:
        result = submit_training_job(train_req, orch_url=ORCH_URL, timeout=req.timeout)
    except NoOrchestratorAvailableError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except LivepeerGatewayError as e:
        raise HTTPException(status_code=502, detail=str(e))

    # If wait=true, poll until done
    if req.wait and result.orchestrator_url:
        try:
            final = wait_for_training(result.job_id, result.orchestrator_url)
            elapsed_ms = int((time.time() - t0) * 1000)
            return TrainingSubmitResponse(
                job_id=final.job_id,
                status=final.status,
                orchestrator=result.orchestrator_url,
                result=final.result,
                error=final.error,
                elapsed_ms=elapsed_ms,
            )
        except LivepeerGatewayError as e:
            raise HTTPException(status_code=502, detail=str(e))

    elapsed_ms = int((time.time() - t0) * 1000)
    return TrainingSubmitResponse(
        job_id=result.job_id,
        status=result.status,
        orchestrator=result.orchestrator_url,
        status_url=result.status_url,
        elapsed_ms=elapsed_ms,
    )


@app.get("/train/{job_id}", response_model=TrainingStatusResponse)
async def train_status(job_id: str):
    """Get training job status."""
    try:
        status = get_training_status(job_id, ORCH_URL)
    except LivepeerGatewayError as e:
        raise HTTPException(status_code=404, detail=str(e))

    return TrainingStatusResponse(
        job_id=status.job_id,
        status=status.status,
        progress=status.progress,
        result=status.result,
        error=status.error,
        model_id=status.model_id,
        cost=status.cost,
        balance=status.balance,
    )


# ---------------------------------------------------------------------------
# Prompt enrichment via Gemini (routed through Livepeer network)
# ---------------------------------------------------------------------------

ENRICH_CAPABILITY = os.environ.get("ENRICH_CAPABILITY", "gemini-text")

ENRICH_SYSTEM_TEMPLATE = """You are a master creative director AI agent controlling a media generation pipeline.
Given a user's task, produce a PRECISE execution plan as a JSON array.

For EACH step, return a JSON object with:
- "id": unique short ID for referencing (e.g. "dragon_01", "ocean_02") — snake_case, descriptive
- "type": "image" | "video" | "music"
- "prompt": richly enriched, detailed prompt — see PROMPT ENGINEERING below
- "capability": MUST be one of the AVAILABLE CAPABILITIES listed below
- "depends_on": null (or step id for chaining, e.g. image→video, image→edit)
- "params": optional dict of extra params (e.g. {{"width": 1024, "height": 1024}} for size-capped images)
- "title": short human-readable title (e.g. "Dragon vs Anglerfish — Abyss")

IMPORTANT RULES:
1. OUTPUT COUNT vs SUBJECT COUNT — ABSOLUTELY CRITICAL:
   Numbers describe EITHER subjects in the scene OR number of outputs. You MUST distinguish:
   - "two girls skating" → 1 image containing two girls (number describes SUBJECTS)
   - "three cats playing" → 1 image containing three cats (number describes SUBJECTS)
   - "a dragon fighting five knights" → 1 image with one dragon and five knights (SUBJECTS)
   - "create 3 pictures of a cat" → 3 separate images (number modifies "pictures" = OUTPUT COUNT)
   - "5 images of landscapes" → 5 separate images (number modifies "images" = OUTPUT COUNT)
   A number is an OUTPUT COUNT ONLY when it directly modifies words like "pictures", "images", "photos", "videos", "drawings", "sketches", "outputs", "variations". Otherwise it describes subjects in the scene. When no output count is given, produce exactly 1 output.
2. VARY EACH OUTPUT. When creating multiple of the same subject, vary: camera angle, style, mood, lighting, composition.
3. DISTRIBUTE ACROSS MODELS for variety unless user specifies one.
4. Each "id" must be unique and descriptive.
5. ONLY use capabilities from the AVAILABLE list. Never invent capability names.
6. IMAGE→VIDEO CHAINS: Use URL-returning models (recraft-v4, nano-banana, flux-*) for the image step. Do NOT use gemini-image — it returns base64 which video models cannot consume.
6b. VIDEO→VIDEO CHAINS (wan-v2v): The source video MUST have an HTTP URL. Do NOT use lucy-i2v as source for wan-v2v chains — lucy-i2v returns base64 video which wan-v2v cannot consume. Use ltx-i2v or kling-i2v instead (they return URLs).
6c. IMAGE SIZE FOR VIDEO CHAINS: When an image step feeds into a video step (image→video), add "params": {{"width": 1024, "height": 1024}} to the image step. Video models have a 10MB input limit — high-res images can exceed this.

MULTI-PHASE WORKFLOW PLANNING — CRITICAL:
11. SEQUENTIAL PHASES: When the user says "create X, then do Y with some of them", you MUST create ALL generation steps FIRST, then create dependent steps with depends_on pointing to specific source steps. Example: "create 5 images, animate 2 of them" → 5 image steps (no depends_on) + 2 video steps (each with depends_on pointing to a specific image step).
12. SELECTION CONSTRAINTS: When the user says "pick N of them" or "select some", choose N specific source steps to chain from. If there's a constraint (e.g. "not from gemini"), ensure the selected source steps satisfy that constraint — use appropriate models for those specific sources.
13. INDEPENDENT GROUPS: Different subjects in the same prompt are independent. "Create 5 portraits AND 2 fish drawings" → 5 portrait steps + 2 fish steps, ALL independent (no depends_on between groups). They run concurrently.
14. NEVER create video steps without an image source unless the user explicitly asks for text-to-video. "Animate" or "bring to life" ALWAYS means image→video (i2v) with depends_on, not text-to-video.

EDITING vs GENERATION — CRITICAL:
7. RESTYLING / STYLE TRANSFER: When the user wants to change the style of an existing image (restyle, transform, convert to anime/watercolor/oil painting/etc.), you MUST use an IMAGE EDITING model (kontext-edit or reve-edit) with depends_on pointing to the source image. These models take image_url and preserve composition/subjects while changing style. NEVER use text-to-image models for restyling — they generate completely new images and discard the original.
8. EDITING MODELS require image_url (injected automatically via depends_on). They EDIT the input. Text-to-image models GENERATE from scratch.
9. UPSCALING: Use topaz-upscale with depends_on pointing to the source. It enhances resolution without changing content.
10. BACKGROUND REMOVAL: Use bg-remove with depends_on pointing to the source.

PROMPT ENGINEERING BY MODEL TYPE:
- Text-to-image (recraft, flux, nano-banana, gemini-image, qwen): Write a rich descriptive prompt with subject, artistic style, lighting, composition, mood, color palette, camera angle, atmosphere, texture.
- Image editing (kontext-edit, reve-edit): Write an INSTRUCTION prompt, e.g. "Restyle this image as [style]. Keep all subjects, poses, and composition exactly the same. Only change the artistic rendering." Be explicit about what to KEEP and what to CHANGE.
- Video (ltx, lucy, kling): Describe the MOTION and animation, not just the scene. Include camera movement, subject animation, atmosphere dynamics.
- Audio (chatterbox-tts, lux-tts): The prompt IS the text to speak. Keep it natural.

{capability_catalog}

MODEL SELECTION STRATEGY:
- "fast/quick/draft" → fast models (nano-banana, fast-lcm, flux-schnell)
- "best/stunning/high quality" → quality models (flux-pro, recraft-v4, kling-i2v)
- "artistic/creative/unique" → creative models (gemini-image, qwen-image, recraft-v4)
- "realistic/photo" → photorealistic models (flux-pro, gemini-image)
- "restyle/transform style/watercolor/anime/..." → EDITING models (kontext-edit, reve-edit) with depends_on
- "upscale/enhance/sharpen" → topaz-upscale with depends_on
- When creating multiple outputs, MIX models for stylistic variety
- If user specifies a model ("using recraft"), respect that

Return ONLY a valid JSON array. No markdown, no explanation, no code fences."""

# Default catalog when no capabilities are provided
DEFAULT_CAPABILITY_CATALOG = """AVAILABLE CAPABILITIES:
Image models (text-to-image):
  - nano-banana    | Fast (~3s)  | Low quality   | Good for drafts, testing. Returns URL.
  - recraft-v4     | Medium (8s) | High quality  | Illustrations, graphic design. Returns URL.
  - gemini-image   | Medium (10s)| High quality  | Photorealism, creative. Returns base64 — NOT for image→video chains.
  - flux-schnell   | Fast (~5s)  | Good quality  | FLUX fast, versatile. Returns URL.
  - flux-dev       | Medium (12s)| High quality  | FLUX dev, better detail. Returns URL.
  - flux-pro       | Slow (15s)  | Best quality  | FLUX pro, highest quality. Returns URL.
  - qwen-image     | Medium (10s)| High quality  | Qwen, good with Asian art. Returns URL.
  - fast-lcm       | Fastest (2s)| Draft quality | LCM distilled, fastest. Returns URL.

Image models (editing — requires image_url from prior step):
  - kontext-edit   | Medium (10s)| High quality  | Edit/transform images with text instructions.
  - reve-edit      | Medium (10s)| High quality  | Image editing and restyling.
  - topaz-upscale  | Medium (8s) | Enhancement   | Upscale and enhance resolution.
  - bg-remove      | Fast (3s)   | Utility       | Remove background from image.

Video models:
  - ltx-t2v-23     | Medium (20s)| Good quality  | Text-to-video, general purpose
  - ltx-i2v        | Medium (20s)| Good quality  | Image-to-video (needs image_url)
  - lucy-i2v       | Slow (30s)  | High quality  | Image-to-video, cinematic motion
  - wan-v2v        | Slow (30s)  | High quality  | Video-to-video transformation
  - kling-i2v      | Slow (40s)  | Best quality  | Image-to-video, professional cinematic

Audio models:
  - chatterbox-tts | Fast (5s)   | Good quality  | Text-to-speech, natural voice
  - lux-tts        | Fast (5s)   | Good quality  | Text-to-speech, expressive voice"""


def _build_capability_catalog(capabilities: list[dict]) -> str:
    """Build a model catalog string from a list of capability dicts."""
    if not capabilities:
        return DEFAULT_CAPABILITY_CATALOG

    lines = ["AVAILABLE CAPABILITIES (you MUST only use these):"]
    image_gen_caps = []
    image_edit_caps = []
    video_caps = []
    music_caps = []
    other_caps = []

    # Known editing models — these require image_url and preserve composition
    EDITING_MODELS = {"kontext-edit", "reve-edit", "topaz-upscale", "bg-remove"}

    for cap in capabilities:
        name = cap.get("name", "")
        desc = cap.get("description", "")
        model_id = cap.get("model_id", "")
        cap_type = cap.get("type", "")
        sub_type = cap.get("sub_type", "")

        # Infer type from name/model_id if not provided
        if not cap_type:
            if any(k in name.lower() for k in ["image", "banana", "recraft", "gemini-image", "seedream", "imagen", "dall",
                                                   "flux", "qwen", "kontext", "reve", "topaz", "upscale", "bg-remove", "lcm"]):
                cap_type = "image"
            elif any(k in name.lower() for k in ["video", "t2v", "i2v", "v2v", "ltx", "lucy", "wan", "kling", "veo", "gen-"]):
                cap_type = "video"
            elif any(k in name.lower() for k in ["music", "audio", "beatoven", "tts", "speech", "chatterbox", "lux-tts"]):
                cap_type = "music"

        entry = f"  - {name}"
        if sub_type:
            entry += f" | {sub_type}"
        if desc:
            entry += f" | {desc}"
        elif model_id:
            entry += f" | {model_id}"

        is_edit = name in EDITING_MODELS or "editing" in sub_type.lower() or "upscale" in sub_type.lower() or "utility" in sub_type.lower()
        if cap_type == "image" and is_edit:
            image_edit_caps.append(entry)
        elif cap_type == "image":
            image_gen_caps.append(entry)
        elif cap_type == "video":
            video_caps.append(entry)
        elif cap_type == "music":
            music_caps.append(entry)
        else:
            other_caps.append(entry)

    if image_gen_caps:
        lines.append("Image models (text-to-image — generate NEW images from text):")
        lines.extend(image_gen_caps)
    if image_edit_caps:
        lines.append("Image models (editing — EDIT existing images, requires image_url via depends_on):")
        lines.append("  ⚠ These models PRESERVE the original composition. Use for restyle/transform/upscale/bg-remove.")
        lines.extend(image_edit_caps)
    if video_caps:
        lines.append("Video models:")
        lines.extend(video_caps)
    if music_caps:
        lines.append("Music/Audio models:")
        lines.extend(music_caps)
    if other_caps:
        lines.append("Other models:")
        lines.extend(other_caps)

    return "\n".join(lines)


class EnrichRequest(BaseModel):
    task: str = Field(..., description="User's natural language task description")
    context: str = Field("", description="Optional context (previous results, references)")
    capabilities: list[dict[str, Any]] = Field(
        default_factory=list,
        description="Available capabilities [{name, model_id, type?, description?}]. "
                    "If empty, uses default catalog.",
    )


class EnrichResponse(BaseModel):
    steps: list[dict[str, Any]] = Field(default_factory=list)
    raw: str = ""


def _parse_enrichment_response(raw_text: str) -> list[dict]:
    """Parse Gemini text response into structured steps."""
    clean = raw_text.strip()
    if clean.startswith("```"):
        clean = clean.split("\n", 1)[1] if "\n" in clean else clean[3:]
    if clean.endswith("```"):
        clean = clean[:-3]
    clean = clean.strip()
    return json.loads(clean)


@app.post("/enrich", response_model=EnrichResponse)
async def enrich(req: EnrichRequest):
    """Use Gemini (via Livepeer network) to parse and enrich a task into structured generation steps.

    Pass `capabilities` to constrain model selection to only available models.
    """
    catalog = _build_capability_catalog(req.capabilities)
    system_prompt = ENRICH_SYSTEM_TEMPLATE.format(capability_catalog=catalog)

    full_prompt = f"{system_prompt}\n\nUser task: {req.task}"
    if req.context:
        full_prompt += f"\n\nContext: {req.context}"

    # Route through Livepeer network: SDK Service → Orchestrator → Adapter → Proxy → Gemini
    byoc_req = ByocJobRequest(
        capability=ENRICH_CAPABILITY,
        payload={"prompt": full_prompt},
        timeout_seconds=30,
    )

    try:
        result = submit_byoc_job(byoc_req, orch_url=ORCH_URL, timeout=30)
    except NoOrchestratorAvailableError as e:
        raise HTTPException(status_code=503, detail=f"No orchestrator for {ENRICH_CAPABILITY}: {e}")
    except LivepeerGatewayError as e:
        raise HTTPException(status_code=502, detail=f"Livepeer error: {e}")

    # The Gemini provider returns {"text": "..."} for text-only responses
    raw_text = ""
    if isinstance(result.data, dict):
        raw_text = result.data.get("text", "")
    if not raw_text:
        raise HTTPException(status_code=502, detail=f"Gemini returned no text: {result.data}")

    try:
        steps = _parse_enrichment_response(raw_text)
        return EnrichResponse(steps=steps, raw=raw_text)
    except Exception as e:
        logger.error("Failed to parse enrichment response: %s\nRaw: %s", e, raw_text[:500])
        raise HTTPException(status_code=502, detail=f"Failed to parse Gemini response: {e}")


# ---------------------------------------------------------------------------
# Agentic Enrichment v2 — Chain-of-thought, Decompose, Validate, Re-plan
# ---------------------------------------------------------------------------

def _llm_call(prompt: str, timeout: int = 30, max_output_tokens: int = 0) -> str:
    """Make a single LLM call via Livepeer network. Returns raw text."""
    payload = {"prompt": prompt}
    if max_output_tokens > 0:
        payload["max_output_tokens"] = max_output_tokens
    byoc_req = ByocJobRequest(
        capability=ENRICH_CAPABILITY,
        payload=payload,
        timeout_seconds=timeout,
    )
    result = submit_byoc_job(byoc_req, orch_url=ORCH_URL, timeout=timeout)
    if isinstance(result.data, dict):
        return result.data.get("text", "")
    return ""


def _parse_json_from_llm(raw: str, repair_array: bool = False) -> Any:
    """Extract JSON from LLM response, handling markdown fences and mixed text.

    If repair_array=True, attempts to repair truncated JSON arrays by
    extracting all complete objects from a partial array.
    """
    import re
    clean = raw.strip()
    # Strip all markdown code fences
    clean = re.sub(r'```(?:json)?\s*\n?', '', clean).strip()
    # Try to find a complete JSON array first
    arr_start = clean.find("[")
    if arr_start >= 0:
        depth = 0
        for i in range(arr_start, len(clean)):
            if clean[i] == "[":
                depth += 1
            elif clean[i] == "]":
                depth -= 1
                if depth == 0:
                    try:
                        return json.loads(clean[arr_start:i + 1])
                    except json.JSONDecodeError:
                        break

        # Array found but not closed — repair by extracting complete objects
        if repair_array:
            items = []
            obj_depth = 0
            obj_start = -1
            in_string = False
            escape_next = False
            for i in range(arr_start + 1, len(clean)):
                c = clean[i]
                if escape_next:
                    escape_next = False
                    continue
                if c == "\\":
                    escape_next = True
                    continue
                if c == '"':
                    in_string = not in_string
                    continue
                if in_string:
                    continue
                if c == "{" and obj_depth == 0:
                    obj_start = i
                    obj_depth = 1
                elif c == "{":
                    obj_depth += 1
                elif c == "}" and obj_depth > 0:
                    obj_depth -= 1
                    if obj_depth == 0 and obj_start >= 0:
                        try:
                            obj = json.loads(clean[obj_start:i + 1])
                            items.append(obj)
                        except json.JSONDecodeError:
                            pass
                        obj_start = -1
            if items:
                logger.info("Repaired truncated array: extracted %d complete objects", len(items))
                return items

    # Try to find a complete JSON object
    obj_start = clean.find("{")
    if obj_start >= 0:
        depth = 0
        for i in range(obj_start, len(clean)):
            if clean[i] == "{":
                depth += 1
            elif clean[i] == "}":
                depth -= 1
                if depth == 0:
                    try:
                        return json.loads(clean[obj_start:i + 1])
                    except json.JSONDecodeError:
                        break

    # Last resort: try the whole thing
    return json.loads(clean)


# ── Phase 1: Decompose + Reason (Chain-of-Thought) ──

DECOMPOSE_PROMPT = """You are a creative director. Analyze the user's request and plan media generation.

Think step by step:
1. INTENT: What to create? How many outputs?
   CRITICAL: "two girls" = subjects in scene (1 output). "5 pictures" = 5 separate outputs. Numbers are output counts ONLY when modifying words like "pictures/images/videos/drawings". Default: 1 output.
2. NARRATIVE: How do outputs relate? What story connects them?
3. TASKS: List each output with type (image/video/audio), dependencies, and role.
   Chain types: "i2v" (image to video), "edit" (restyle), "upscale", "v2v" (video to video), null.
   Narrative roles: establishing, hero, transition, climax, finale, ambient.

Return a JSON object:
{{"intent":"...","output_count_reasoning":"...","narrative":"...","sub_tasks":[{{"id":"snake_case","description":"...","type":"image","depends_on_task":null,"chain_type":null,"narrative_role":"hero","style_notes":"..."}}],"model_strategy":"diverse","style_coherence":"..."}}\n\nReturn ONLY valid JSON. No markdown fences."""


# ── Phase 2: Plan (Convert decomposition to executable steps) ──

PLAN_PROMPT_TEMPLATE = """You are a technical planner converting a creative brief into executable generation steps.

DECOMPOSITION:
{decomposition}

CAPABILITY CATALOG:
{capability_catalog}

CONSTRAINTS:
- gemini-image returns base64 — CANNOT chain to video models (use recraft-v4, flux-* instead)
- lucy-i2v returns base64 video — CANNOT chain to wan-v2v (use ltx-i2v or kling-i2v)
- Image steps feeding video: add "params": {{"width": 1024, "height": 1024}}
- Editing models (kontext-edit, reve-edit, topaz-upscale, bg-remove) REQUIRE depends_on
- i2v video models REQUIRE depends_on with image source
- ONLY use capabilities from the catalog above

PROMPT CRAFTING RULES:
- Text-to-image: 2-3 sentence scene description with subject, style, lighting, composition. Be specific but concise.
- Image editing: Instruction prompt — "Restyle as [style]. Keep subjects and composition."
- Video: Describe MOTION concisely — camera movement, subject animation.
- Audio/TTS: The text to speak.

NARRATIVE CONTEXT: {narrative}
STYLE COHERENCE: {style_coherence}
Ensure each prompt reflects its narrative role and maintains thematic consistency.

For EACH step return a JSON object with these keys:
"id", "type", "prompt" (2-3 sentences max), "capability", "depends_on" (id or null), "params" (dict), "title" (short), "narrative_role"

CRITICAL: Return ALL steps in a single JSON array. Do not stop early. You MUST include every step from the decomposition.
Return ONLY a valid JSON array. No markdown fences, no explanation."""


# ── Phase 3: Validate (Deterministic) ──

def validate_plan(steps: list[dict], available_caps: set[str]) -> tuple[list[dict], list[str]]:
    """Validate and fix plan against capability constraints. Returns (fixed_steps, fixes)."""
    fixes = []
    id_to_step = {s["id"]: s for s in steps}
    id_to_idx = {s["id"]: i for i, s in enumerate(steps)}

    # Model category mappings for fallback
    IMAGE_GEN = ["recraft-v4", "flux-schnell", "flux-dev", "flux-pro", "nano-banana", "qwen-image", "gemini-image", "fast-lcm"]
    IMAGE_EDIT = ["kontext-edit", "reve-edit"]
    VIDEO_I2V = ["ltx-i2v", "lucy-i2v", "kling-i2v"]
    VIDEO_T2V = ["ltx-t2v-23"]
    AUDIO = ["chatterbox-tts", "lux-tts"]

    def _find_fallback(step_type: str, exclude: set = None) -> Optional[str]:
        exclude = exclude or set()
        if step_type == "image":
            for m in IMAGE_GEN:
                if m in available_caps and m not in exclude:
                    return m
        elif step_type == "video":
            for m in VIDEO_I2V + VIDEO_T2V:
                if m in available_caps and m not in exclude:
                    return m
        elif step_type == "music":
            for m in AUDIO:
                if m in available_caps and m not in exclude:
                    return m
        return None

    for step in steps:
        cap = step.get("capability", "")

        # 1. Capability must exist
        if cap not in available_caps:
            fallback = _find_fallback(step.get("type", "image"))
            if fallback:
                fixes.append(f'{step["id"]}: unknown "{cap}" → {fallback}')
                step["capability"] = fallback
            else:
                fixes.append(f'{step["id"]}: unknown "{cap}" and no fallback available')

        # 2. gemini-image feeding video chain
        dep_of = [s for s in steps if s.get("depends_on") == step["id"] and s.get("type") == "video"]
        if step.get("capability") == "gemini-image" and dep_of:
            old = step["capability"]
            step["capability"] = "recraft-v4" if "recraft-v4" in available_caps else "flux-schnell"
            fixes.append(f'{step["id"]}: {old} → {step["capability"]} (feeds video chain, base64 incompatible)')

        # 3. lucy-i2v feeding wan-v2v
        if step.get("capability") == "wan-v2v" and step.get("depends_on"):
            parent = id_to_step.get(step["depends_on"])
            if parent and parent.get("capability") == "lucy-i2v":
                parent["capability"] = "ltx-i2v" if "ltx-i2v" in available_caps else "kling-i2v"
                fixes.append(f'{parent["id"]}: lucy-i2v → {parent["capability"]} (feeds v2v, base64 incompatible)')

        # 4. Image size cap for video chains
        if step.get("type") == "video" and step.get("depends_on"):
            parent = id_to_step.get(step["depends_on"])
            if parent and parent.get("type") == "image":
                params = parent.setdefault("params", {})
                if "width" not in params:
                    params["width"] = 1024
                    params["height"] = 1024
                    fixes.append(f'{parent["id"]}: added size cap 1024×1024 for video chain')

        # 5. Editing model without depends_on
        EDITING = {"kontext-edit", "reve-edit", "topaz-upscale", "bg-remove"}
        if cap in EDITING and not step.get("depends_on"):
            fixes.append(f'{step["id"]}: editing model {cap} has no source — removed')
            step["_remove"] = True

        # 6. depends_on references non-existent step
        dep = step.get("depends_on")
        if dep and dep not in id_to_step:
            fixes.append(f'{step["id"]}: depends_on "{dep}" not found — cleared')
            step["depends_on"] = None

    # 7. Circular dependency detection
    for step in steps:
        visited = set()
        current = step.get("depends_on")
        while current:
            if current in visited:
                fixes.append(f'{step["id"]}: circular dependency via {current} — cleared')
                step["depends_on"] = None
                break
            visited.add(current)
            parent = id_to_step.get(current)
            current = parent.get("depends_on") if parent else None

    # 8. Remove marked steps
    steps = [s for s in steps if not s.get("_remove")]
    # Clean internal markers
    for s in steps:
        s.pop("_remove", None)

    return steps, fixes


# ── Phase 4: Re-plan ──

REPLAN_PROMPT_TEMPLATE = """A media generation plan is partially complete. One step failed. Decide how to recover.

ORIGINAL PLAN:
{steps_json}

COMPLETED STEPS (with URLs):
{completed_json}

FAILED STEP:
  id: {failed_id}
  capability: {failed_cap}
  error: {error}

AVAILABLE CAPABILITIES:
{capability_catalog}

Choose ONE action:
1. "retry" — try with a different model. Return the replacement step.
2. "skip" — this step is non-critical, skip it and adjust dependents.
3. "restructure" — replace failed step and its dependents with alternative approach.

Return a JSON object (NO markdown fences):
{{
  "action": "retry|skip|restructure",
  "reason": "why",
  "patched_steps": [... only new/modified steps with full schema ...]
}}"""


# ── Endpoint: /enrich/v2 ──

class EnrichV2Request(BaseModel):
    task: str = Field(..., description="User's natural language task")
    context: str = Field("", description="Optional context (previous results)")
    capabilities: list[dict[str, Any]] = Field(default_factory=list)
    quality: str = Field("balanced", description="'fast' (1 LLM call) | 'balanced' (2 calls) | 'cinematic' (2 calls, richer)")


class EnrichV2Response(BaseModel):
    steps: list[dict[str, Any]] = Field(default_factory=list)
    reasoning: Optional[dict[str, Any]] = None
    validation_fixes: list[str] = Field(default_factory=list)
    phases: list[str] = Field(default_factory=list)


@app.post("/enrich/v2")
async def enrich_v2(req: EnrichV2Request):
    """Agentic enrichment with chain-of-thought, decomposition, validation, and narrative planning."""
    catalog = _build_capability_catalog(req.capabilities)
    available_caps = {c.get("name", "") for c in req.capabilities} if req.capabilities else set(DEFAULT_CAPABILITY_CATALOG.split("  - ")[1:])
    # Build available caps set from catalog text if no capabilities provided
    if not req.capabilities:
        import re
        available_caps = set(re.findall(r'^\s*-\s+(\S+)', DEFAULT_CAPABILITY_CATALOG, re.MULTILINE))

    phases = []
    reasoning = None
    raw_decomposition = ""

    # ── Phase 1: Decompose + Reason (skip in fast mode) ──
    if req.quality != "fast":
        phases.append("decompose")
        decompose_prompt = DECOMPOSE_PROMPT + f"\n\nUser request: {req.task}"
        if req.context:
            decompose_prompt += f"\n\nContext: {req.context}"

        try:
            raw_decomposition = _llm_call(decompose_prompt, timeout=30)
            logger.info("Phase 1 raw response: %s", raw_decomposition[:500])
            reasoning = _parse_json_from_llm(raw_decomposition)
            # If Gemini returned a list instead of dict, wrap it
            if isinstance(reasoning, list):
                reasoning = {"intent": req.task, "sub_tasks": reasoning, "narrative": "", "style_coherence": ""}
            logger.info("Phase 1 decomposition: %s", json.dumps(reasoning, indent=2)[:500])
        except Exception as e:
            logger.warning("Phase 1 decomposition failed: %s\nRaw: %s", e, raw_decomposition[:300] if raw_decomposition else "empty")
            # Fall back to old single-shot enrichment
            return await _fallback_enrich(req, catalog)

    # ── Phase 2: Plan ──
    phases.append("plan")
    if reasoning:
        # Use decomposition to guide planning
        plan_prompt = PLAN_PROMPT_TEMPLATE.format(
            decomposition=json.dumps(reasoning, indent=2),
            capability_catalog=catalog,
            narrative=reasoning.get("narrative", ""),
            style_coherence=reasoning.get("style_coherence", ""),
        )
    else:
        # Fast mode — single-shot (use old template but with chain-of-thought hint)
        system_prompt = ENRICH_SYSTEM_TEMPLATE.format(capability_catalog=catalog)
        plan_prompt = f"{system_prompt}\n\nUser task: {req.task}"
        if req.context:
            plan_prompt += f"\n\nContext: {req.context}"

    try:
        raw_plan = _llm_call(plan_prompt, timeout=45, max_output_tokens=8192)
        logger.info("Phase 2 raw response: %d chars", len(raw_plan))
        steps = _parse_json_from_llm(raw_plan, repair_array=True)
        if isinstance(steps, dict):
            if "steps" in steps:
                steps = steps["steps"]
            elif "id" in steps and "type" in steps:
                # Single step returned as dict — wrap in list
                steps = [steps]
            else:
                # Try extracting any list value from the dict
                for v in steps.values():
                    if isinstance(v, list):
                        steps = v
                        break
        if not isinstance(steps, list):
            raise ValueError(f"Expected list, got {type(steps)}")
        logger.info("Phase 2 parsed %d steps", len(steps))
    except Exception as e:
        logger.error("Phase 2 planning failed: %s", e)
        if reasoning:
            # Try to fall back to single-shot
            return await _fallback_enrich(req, catalog)
        raise HTTPException(status_code=502, detail=f"Planning failed: {e}")

    # ── Phase 3: Validate ──
    phases.append("validate")
    steps, validation_fixes = validate_plan(steps, available_caps)
    if validation_fixes:
        logger.info("Validation fixes: %s", validation_fixes)

    return {
        "steps": steps,
        "reasoning": reasoning,
        "validation_fixes": validation_fixes,
        "phases": phases,
    }


async def _fallback_enrich(req: EnrichV2Request, catalog: str) -> dict:
    """Fallback to single-shot enrichment if agentic pipeline fails."""
    system_prompt = ENRICH_SYSTEM_TEMPLATE.format(capability_catalog=catalog)
    full_prompt = f"{system_prompt}\n\nUser task: {req.task}"
    if req.context:
        full_prompt += f"\n\nContext: {req.context}"
    raw = _llm_call(full_prompt, timeout=30)
    steps = _parse_json_from_llm(raw)
    if isinstance(steps, dict) and "steps" in steps:
        steps = steps["steps"]
    if not isinstance(steps, list):
        steps = [steps] if isinstance(steps, dict) else []
    available_caps = {c.get("name", "") for c in req.capabilities} if req.capabilities else set()
    if available_caps:
        steps, fixes = validate_plan(steps, available_caps)
    else:
        fixes = []
    return {"steps": steps, "reasoning": None, "validation_fixes": fixes, "phases": ["fallback"]}


# ── Endpoint: /replan ──

class ReplanRequest(BaseModel):
    original_steps: list[dict[str, Any]]
    failed_step_id: str
    error: str
    completed_results: dict[str, Any] = Field(default_factory=dict)
    capabilities: list[dict[str, Any]] = Field(default_factory=list)


class ReplanResponse(BaseModel):
    action: str  # "retry" | "skip" | "restructure"
    reason: str = ""
    patched_steps: list[dict[str, Any]] = Field(default_factory=list)


@app.post("/replan", response_model=ReplanResponse)
async def replan(req: ReplanRequest):
    """Re-plan after a step failure. Returns patched steps."""
    catalog = _build_capability_catalog(req.capabilities)
    failed_step = next((s for s in req.original_steps if s["id"] == req.failed_step_id), None)
    if not failed_step:
        raise HTTPException(status_code=400, detail=f"Step {req.failed_step_id} not found")

    prompt = REPLAN_PROMPT_TEMPLATE.format(
        steps_json=json.dumps(req.original_steps, indent=2),
        completed_json=json.dumps(req.completed_results, indent=2),
        failed_id=req.failed_step_id,
        failed_cap=failed_step.get("capability", "?"),
        error=req.error[:500],
        capability_catalog=catalog,
    )

    try:
        raw = _llm_call(prompt, timeout=20)
        result = _parse_json_from_llm(raw)
        return ReplanResponse(
            action=result.get("action", "skip"),
            reason=result.get("reason", ""),
            patched_steps=result.get("patched_steps", []),
        )
    except Exception as e:
        logger.error("Re-plan failed: %s", e)
        return ReplanResponse(action="skip", reason=f"Re-plan LLM call failed: {e}")


# ---------------------------------------------------------------------------
# Stream proxy (control plane) — proxies to orchestrator
# ---------------------------------------------------------------------------

def _orch_request(method: str, path: str, body: Optional[bytes] = None,
                  headers: Optional[dict] = None, timeout: int = 30) -> tuple[int, bytes, dict]:
    """Make a request to the orchestrator, handling self-signed certs."""
    url = f"{ORCH_URL}{path}"
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Content-Type", "application/json")
    if headers:
        for k, v in headers.items():
            req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=_ssl_ctx) as r:
            resp_headers = dict(r.headers)
            return r.status, r.read(), resp_headers
    except urllib.error.HTTPError as e:
        return e.code, e.read(), dict(e.headers)


class StreamStartRequest(BaseModel):
    capability: str = Field(..., description="Stream capability name")
    model_id: str = Field("", description="Model ID for the stream")
    params: dict[str, Any] = Field(default_factory=dict)
    enable_video_ingress: bool = Field(True, description="Allow video input via WHIP")
    enable_video_egress: bool = Field(True, description="Enable video output via WHEP")
    enable_data_output: bool = Field(False, description="Enable JSON data output")


def _build_stream_livepeer_header(
    capability: str, stream_id: str, params: dict,
    enable_video_ingress: bool, enable_video_egress: bool,
    enable_data_output: bool,
) -> str:
    """Build base64-encoded Livepeer header for stream start."""
    import uuid
    job_request = {
        "id": stream_id,
        "request": json.dumps(params),
        "capability": capability,
        "timeout_seconds": 300,
        "parameters": json.dumps({
            "enable_video_ingress": enable_video_ingress,
            "enable_video_egress": enable_video_egress,
            "enable_data_output": enable_data_output,
        }),
    }
    return base64.b64encode(json.dumps(job_request).encode()).decode()


@app.post("/stream/start")
async def stream_start(req: StreamStartRequest):
    """Start a live stream through the orchestrator."""
    import uuid
    stream_id = str(uuid.uuid4())[:8]

    livepeer_hdr = _build_stream_livepeer_header(
        capability=req.capability,
        stream_id=stream_id,
        params=req.params,
        enable_video_ingress=req.enable_video_ingress,
        enable_video_egress=req.enable_video_egress,
        enable_data_output=req.enable_data_output,
    )

    body = json.dumps({
        "stream_id": stream_id,
        "params": json.dumps(req.params) if req.params else "{}",
    }).encode()

    status, data, resp_headers = _orch_request(
        "POST", "/ai/stream/start", body,
        headers={
            "Livepeer": livepeer_hdr,
            "Livepeer-Capability": req.capability,
        },
        timeout=300,
    )
    if status >= 400:
        raise HTTPException(status_code=status, detail=data.decode()[:500])
    try:
        result = json.loads(data)
    except Exception:
        result = {"raw": data.decode()[:1000]}
    # Rewrite trickle URLs: replace internal Docker hostnames with public HTTPS URL
    from urllib.parse import urlparse
    pub_parsed = urlparse(ORCH_PUBLIC_URL)
    pub_host = pub_parsed.netloc  # e.g. "34-134-195-88.nip.io"
    for key in ("X-Control-Url", "X-Events-Url", "X-Publish-Url",
                "X-Subscribe-Url", "X-Data-Url"):
        if key in resp_headers:
            url = resp_headers[key]
            # Replace any internal hostname patterns with public address
            for internal in ("0.0.0.0:8935", "byoc_orch:8935", "localhost:8935",
                             "34.134.195.88:8935", "34-134-195-88.nip.io:443"):
                url = url.replace(internal, pub_host)
            # Ensure HTTPS scheme
            if url.startswith("http://"):
                url = "https://" + url[7:]
            result[key.lower().replace("-", "_").replace("x_", "")] = url
    result["stream_id"] = stream_id

    # Initialize MPEG-TS media session for browser ↔ trickle encoding/decoding
    pub_url = result.get("publish_url", "")
    sub_url = result.get("subscribe_url", "")
    if pub_url or sub_url:
        try:
            await _init_stream_session(stream_id, pub_url, sub_url)
        except Exception as e:
            logger.warning("Failed to init MPEG-TS session %s: %s", stream_id, e)

    return result


@app.post("/stream/{stream_id}/stop")
async def stream_stop(stream_id: str):
    """Stop a live stream."""
    # Clean up media session
    session = _stream_sessions.pop(stream_id, None)
    if session:
        await _cleanup_stream_session(session)

    # Build Livepeer header with stream_id in the request field
    job_request = {
        "id": stream_id,
        "request": json.dumps({"stream_id": stream_id}),
        "capability": "scope-live",
    }
    livepeer_hdr = base64.b64encode(json.dumps(job_request).encode()).decode()
    status, data, _ = _orch_request(
        "POST", "/ai/stream/stop", timeout=10,
        body=json.dumps({"stream_id": stream_id}).encode(),
        headers={"Livepeer": livepeer_hdr},
    )
    if status >= 400:
        raise HTTPException(status_code=status, detail=data.decode()[:500])
    return {"status": "stopped", "stream_id": stream_id}


# ---------------------------------------------------------------------------
# Stream media sessions — MPEG-TS encode/decode for browser ↔ trickle
# ---------------------------------------------------------------------------
import asyncio
import io
import threading
from typing import Any

_stream_sessions: dict[str, dict[str, Any]] = {}


async def _init_stream_session(stream_id: str, publish_url: str, subscribe_url: str):
    """Create persistent MediaPublish/MediaOutput for MPEG-TS encoding/decoding."""
    from livepeer_gateway.media_publish import MediaPublish
    from livepeer_gateway.media_output import MediaOutput

    session: dict[str, Any] = {
        "stream_id": stream_id,
        "publish_url": publish_url,
        "subscribe_url": subscribe_url,
        "publisher": None,
        "output": None,
        "latest_frame_jpeg": None,
        "frame_seq": 0,
        "output_task": None,
    }

    # Input: JPEG → MPEG-TS → trickle (MediaPublish encodes to MPEG-TS)
    if publish_url:
        try:
            from livepeer_gateway.media_publish import MediaPublishConfig
            publisher = MediaPublish(publish_url, config=MediaPublishConfig(fps=25.0, keyframe_interval_s=0.5))
        except Exception:
            publisher = MediaPublish(publish_url)
        session["publisher"] = publisher

    # Output: trickle → MPEG-TS → decoded frames (background loop)
    if subscribe_url:
        output = MediaOutput(subscribe_url)
        session["output"] = output
        # Start background task to decode MPEG-TS frames
        session["output_task"] = asyncio.create_task(
            _output_decode_loop(session, output)
        )

    _stream_sessions[stream_id] = session
    logger.info("Stream session %s: MPEG-TS media proxy initialized", stream_id)


async def _output_decode_loop(session: dict, output):
    """Background: read MPEG-TS frames from trickle, keep latest as JPEG."""
    import numpy as np
    from PIL import Image

    try:
        async for decoded in output.frames():
            if session.get("stopped"):
                break
            if getattr(decoded, "kind", None) != "video":
                continue
            frame = getattr(decoded, "frame", None)
            if frame is None:
                continue
            # Convert VideoFrame → JPEG
            arr = frame.to_ndarray(format="rgb24")
            img = Image.fromarray(arr)
            buf = io.BytesIO()
            img.save(buf, format="JPEG", quality=85)
            session["latest_frame_jpeg"] = buf.getvalue()
            session["frame_seq"] += 1
    except Exception as e:
        logger.warning("Output decode loop %s error: %s", session.get("stream_id"), e)


async def _cleanup_stream_session(session: dict):
    """Clean up a stream session."""
    session["stopped"] = True
    if session.get("output_task"):
        session["output_task"].cancel()
    if session.get("publisher"):
        try:
            await session["publisher"].close()
        except Exception:
            pass
    if session.get("output"):
        try:
            await session["output"].close()
        except Exception:
            pass


@app.get("/stream/{stream_id}/frame")
async def stream_get_frame(stream_id: str, seq: int = -1):
    """Get the latest output frame from a stream.

    Reads MPEG-TS from trickle via background MediaOutput, returns decoded JPEG.
    """
    session = _stream_sessions.get(stream_id)
    if not session:
        # Fallback: lazy-init session from stored URLs
        from starlette.responses import Response as StarletteResponse
        return StarletteResponse(status_code=204)

    # Wait for a new frame (up to 5s, poll every 10ms for low latency)
    target_seq = session.get("frame_seq", 0) + (1 if seq >= 0 else 0)
    for _ in range(500):
        jpeg = session.get("latest_frame_jpeg")
        current_seq = session.get("frame_seq", 0)
        if jpeg and (seq < 0 or current_seq > seq):
            from starlette.responses import Response as StarletteResponse
            resp = StarletteResponse(content=jpeg, media_type="image/jpeg")
            resp.headers["X-Trickle-Seq"] = str(current_seq)
            resp.headers["X-Trickle-Latest"] = str(current_seq)
            resp.headers["Access-Control-Expose-Headers"] = "X-Trickle-Seq, X-Trickle-Latest"
            return resp
        await asyncio.sleep(0.01)

    from starlette.responses import Response as StarletteResponse
    return StarletteResponse(status_code=204)


@app.post("/stream/{stream_id}/publish")
async def stream_publish_frame(stream_id: str, request: Request, seq: int = 0):
    """Publish a frame to a stream's input.

    Browser sends JPEG, SDK Service encodes to MPEG-TS via MediaPublish → trickle.
    """
    body = await request.body()
    session = _stream_sessions.get(stream_id)

    if session and session.get("publisher"):
        # Encode JPEG → VideoFrame → MPEG-TS → trickle
        try:
            import numpy as np
            from PIL import Image
            from av import VideoFrame as AVVideoFrame

            img = Image.open(io.BytesIO(body)).convert("RGB")
            arr = np.array(img)
            frame = AVVideoFrame.from_ndarray(arr, format="rgb24")
            await session["publisher"].write_frame(frame)
            return {"status": "ok", "seq": seq, "encoding": "mpegts"}
        except Exception as e:
            logger.warning("MPEG-TS encode error for %s: %s", stream_id, e)
            # Fall through to raw publish

    # Fallback: publish raw bytes to trickle
    trickle_url = f"{ORCH_URL}/ai/trickle/{stream_id}/{seq}"
    try:
        req = urllib.request.Request(trickle_url, data=body, method="POST")
        req.add_header("Content-Type", request.headers.get("content-type", "image/jpeg"))
        with urllib.request.urlopen(req, timeout=10, context=_ssl_ctx) as r:
            return {"status": "ok", "seq": seq}
    except urllib.error.HTTPError as e:
        raise HTTPException(status_code=e.code, detail=e.read().decode()[:200])
    except Exception as e:
        raise HTTPException(status_code=502, detail=str(e))


@app.post("/stream/{stream_id}/whip")
async def stream_whip_proxy(stream_id: str, request: Request):
    """Proxy WHIP signaling to the orchestrator."""
    body = await request.body()
    content_type = request.headers.get("content-type", "application/sdp")
    status, data, resp_headers = _orch_request(
        "POST",
        f"/process/stream/{stream_id}/whip",
        body,
        headers={"Content-Type": content_type},
        timeout=30,
    )
    response = Response(
        content=data,
        status_code=status,
        media_type=resp_headers.get("Content-Type", "application/sdp"),
    )
    for key in ("Location", "ETag", "Link", "Livepeer-Playback-URL"):
        if key in resp_headers:
            response.headers[key] = resp_headers[key]
    return response


@app.get("/stream/{stream_id}/watch")
async def stream_watch(stream_id: str):
    """Continuous MPEG-TS stream — aggregates trickle segments into one HTTP response.

    Simulates j0sh's proposed /all endpoint: subscribes to trickle output
    server-side and streams MPEG-TS bytes continuously to the browser.
    Browser uses a single fetch() + ReadableStream → mux.js → MSE.
    Eliminates per-segment HTTP round-trip latency.
    """
    from starlette.responses import StreamingResponse

    # Build trickle subscribe URL for this stream's output
    from urllib.parse import urlparse
    pub_parsed = urlparse(ORCH_PUBLIC_URL)
    trickle_base = f"{ORCH_URL}/ai/trickle/{stream_id}-out"

    import aiohttp as _aiohttp

    async def generate():
        seq = -1
        connector = _aiohttp.TCPConnector(ssl=False)
        async with _aiohttp.ClientSession(connector=connector) as session:
            while True:
                try:
                    async with session.get(
                        f"{trickle_base}/{seq}",
                        timeout=_aiohttp.ClientTimeout(total=30),
                    ) as r:
                        if r.status == 200:
                            data = await r.read()
                            actual_seq = r.headers.get("Lp-Trickle-Seq")
                            if actual_seq:
                                seq = int(actual_seq) + 1
                            if data and len(data) > 0:
                                yield data
                        elif r.status == 470:
                            latest = r.headers.get("Lp-Trickle-Latest")
                            if latest:
                                seq = int(latest)
                            await asyncio.sleep(0.03)
                        elif r.status == 404:
                            break
                        else:
                            await asyncio.sleep(0.1)
                except asyncio.TimeoutError:
                    continue
                except Exception:
                    break

    return StreamingResponse(
        generate(),
        media_type="video/MP2T",
        headers={
            "Cache-Control": "no-cache, no-store",
            "X-Accel-Buffering": "no",
            "Access-Control-Allow-Origin": "*",
        },
    )


@app.post("/stream/{stream_id}/whep")
async def stream_whep_proxy(stream_id: str, request: Request):
    """Proxy WHEP signaling to the orchestrator for WebRTC playback."""
    body = await request.body()
    content_type = request.headers.get("content-type", "application/sdp")
    # WHEP URL pattern: /live/video-to-video/{stream_id}-out/whep
    status, data, resp_headers = _orch_request(
        "POST",
        f"/live/video-to-video/{stream_id}-out/whep",
        body,
        headers={"Content-Type": content_type},
        timeout=30,
    )
    response = Response(
        content=data,
        status_code=status,
        media_type=resp_headers.get("Content-Type", "application/sdp"),
    )
    for key in ("Location", "ETag", "Link"):
        if key in resp_headers:
            response.headers[key] = resp_headers[key]
    return response
