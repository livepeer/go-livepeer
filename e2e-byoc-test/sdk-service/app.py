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

import logging
import os
import time
from typing import Any, Optional

from fastapi import FastAPI, HTTPException
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
ADAPTER_URLS = os.environ.get(
    "ADAPTER_URLS", "http://34.134.195.88:9090,http://34.134.195.88:9091,http://34.134.195.88:9092"
).split(",")

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
    if req.prompt:
        payload["prompt"] = req.prompt

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
