# Real-Time AI Container Orchestration — Implementation Plan

## Current State

**What already exists:**
- `ai/worker/docker.go` — Full Docker container lifecycle (pull, warm, borrow, return, health check)
- `ai/worker/runtime.go` — gVisor runtime download/install (basic support exists)
- `net/lp_rpc.proto` — `HardwareInformation` + `GPUComputeInfo` (name, memory, compute version)
- `core/capabilities.go` — Capability bitstring system (38 capabilities defined)
- `core/ai_orchestrator.go` — Remote AI worker management with capacity tracking
- `server/selection_algorithm.go` — Probability-based + live selection algorithms
- `server/rpc.go` — Orchestrator info returned with hardware info to gateways

**What's missing:**
- GPU class/tier classification (consumer vs datacenter vs CC-capable)
- Hardware-aware job routing at the gateway level
- gVisor enforcement (runtime exists but isn't enforced per-container)
- Custom container image support (currently hardcoded to `livepeer/ai-runner:*` variants)
- Container image verification (signatures, allowlists)
- Resource limits and network isolation per container

---

## Phase 1: Hardware-Based Orchestrator Selection

**Goal:** Gateways can route jobs to orchestrators based on GPU hardware tier.

### 1.1 — Extend GPU Classification in Protobuf

**File:** `net/lp_rpc.proto`

Add GPU tier enum and extend `GPUComputeInfo`:

```protobuf
enum GPUTier {
  GPU_TIER_UNKNOWN = 0;
  GPU_TIER_CONSUMER = 1;       // RTX 3090, 4090
  GPU_TIER_DATACENTER = 2;     // A100, L40
  GPU_TIER_CC_CAPABLE = 3;     // H100, H200, B100, B200
}

// Extend GPUComputeInfo:
message GPUComputeInfo {
  // ... existing fields ...
  GPUTier tier = 6;
  bool cc_enabled = 7;          // Actually running in CC mode
  bytes attestation_report = 8; // Optional CC attestation
}
```

### 1.2 — GPU Detection & Classification

**New file:** `ai/worker/gpu_classify.go`

- Map GPU names to tiers using a lookup table (e.g., "H100" → CC_CAPABLE, "A100" → DATACENTER, "RTX 4090" → CONSUMER)
- Parse GPU name from existing `GPUComputeInfo.name` field
- Detect CC mode via NVIDIA driver sysfs/nvml (if available)
- Populate new tier fields when `HardwareInformation` is built in `ai/worker/container.go`

### 1.3 — Hardware-Aware Selection at Gateway

**Files:** `server/selection_algorithm.go`, `server/selection.go`

- Add `RequiredGPUTier` field to job/session requests
- Extend `ProbabilitySelectionAlgorithm.Select()` to filter orchestrators by GPU tier before scoring
- Add `HardwareFilter` predicate in `discovery/discovery.go` for orchestrator pool filtering
- Gateway CLI flag: `--min-gpu-tier` (default: 0/any)

### 1.4 — Expose Hardware Tier in Discovery

**Files:** `server/rpc.go`, `core/orchestrator.go`

- Include GPU tier in `OrchestratorInfo` response (already has `hardware[]`, just needs tier populated)
- Cache hardware tier on orchestrator startup (no need to re-detect per request)

**Deliverable:** Gateway can filter orchestrators by GPU class. Jobs requiring CC-capable hardware only route to H100+ nodes.

---

## Phase 2: gVisor Sandboxing Enforcement

**Goal:** Containers run under gVisor by default, with per-container runtime selection.

### 2.1 — Harden Existing gVisor Integration

**File:** `ai/worker/runtime.go`

- Current state: `--ai-runtime gvisor` flag exists, downloads runsc binary
- Add: Verify runsc binary integrity (checksum verification on download)
- Add: Health check that gVisor runtime is actually functional before accepting jobs
- Add: Fallback behavior when gVisor unavailable (reject job vs run without sandbox)

### 2.2 — Per-Container Runtime Selection

**File:** `ai/worker/docker.go`

- In `createContainer()` (line ~400), set `HostConfig.Runtime` based on:
  1. Container image trust level (Livepeer-signed → optional gVisor, unknown → require gVisor)
  2. Operator config (`--ai-runtime` flag)
  3. Job request (gateway can request sandboxed execution)
- Add runtime field to `RunnerContainer` struct for tracking

### 2.3 — Container Security Hardening

**File:** `ai/worker/docker.go`

Add to `createContainer()` HostConfig:
- Seccomp profile (default Docker profile at minimum)
- Drop all Linux capabilities, only add back what's needed (`SYS_ADMIN` for GPU)
- Read-only root filesystem where possible
- Network isolation: `--network none` for inference containers (they only need GPU + model volume)
- Memory/CPU limits from operator config
- PID limits to prevent fork bombs

### 2.4 — Report Sandbox Status

**File:** `net/lp_rpc.proto`

```protobuf
enum SandboxType {
  SANDBOX_NONE = 0;
  SANDBOX_GVISOR = 1;
  SANDBOX_CC = 2;      // NVIDIA Confidential Computing
}

// Add to HardwareInformation or OrchestratorInfo:
SandboxType sandbox_type = N;
```

Gateway can then route sensitive workloads to sandboxed orchestrators.

**Deliverable:** Containers run sandboxed by default. Gateways know which orchestrators offer sandboxed execution.

---

## Phase 3: Custom Container Support

**Goal:** Orchestrators can run arbitrary (verified) AI containers, not just `livepeer/ai-runner`.

### 3.1 — Container Image Policy

**New file:** `ai/worker/image_policy.go`

- Allowlist of trusted image registries/repos (configurable via CLI/config file)
- Image signature verification using cosign/notary (verify publisher identity)
- Image hash pinning (operator can pin exact digests)
- Default policy: only `livepeer/*` images allowed; operator opts in to custom images

```go
type ImagePolicy struct {
    AllowedRegistries []string          // e.g., ["docker.io/livepeer", "ghcr.io/livepeer"]
    AllowedImages     []string          // Exact image names
    PinnedDigests     map[string]string // image -> sha256 digest
    RequireSignature  bool
    SigningKeys       []string          // cosign public keys
}
```

### 3.2 — Dynamic Container Registration

**Files:** `ai/worker/docker.go`, `core/ai.go`

- Extend `AIModelConfig` (in `core/ai.go`) to accept custom container images:
  ```
  --aiModels pipeline=custom-model,model_id=my-model,container_image=myregistry/my-runner:v1
  ```
- `DockerManager.Warm()` and `Borrow()` use the model config's container image instead of hardcoded `livepeer/ai-runner:*`
- Container image override already partially exists via `ImageOverrides` in docker.go — extend this to be per-model

### 3.3 — Container Interface Contract

**New file:** `ai/worker/container_interface.go`

Define the HTTP API contract that custom containers must implement:
- `GET /health` → `{"status": "IDLE"|"LOADING"|"ERROR"}`
- `GET /hardware/info` → `HardwareInformation` JSON
- `POST /{pipeline}` → Pipeline-specific request/response
- Document this as a spec that third-party container builders follow

### 3.4 — Resource Isolation Per Container

**File:** `ai/worker/docker.go`

- GPU memory partitioning: Use NVIDIA MIG (Multi-Instance GPU) on supported hardware, or `CUDA_MEM_LIMIT` env var
- Volume mounts: Only mount model directory read-only, no host filesystem access
- Tmpfs for scratch space with size limits
- Container-specific environment (no host env leakage)

### 3.5 — Capability Advertisement for Custom Containers

**Files:** `core/capabilities.go`, `net/lp_rpc.proto`

- Custom containers register as `Capability_BYOC` (already exists in proto)
- The `constraint` field carries the custom pipeline/model identifier
- Gateway discovers custom capabilities through existing capability constraints system
- Pricing set per custom model via existing `SetBasePriceForCap` mechanism

**Deliverable:** Operators can run verified third-party AI containers. Custom models are discoverable and routable through the existing capability system.

---

## Phase 4: Integration & Trust Tiers (Future)

### 4.1 — Trust Tier System

Combine phases 1-3 into a unified trust model:

| Tier | Hardware | Sandbox | Container Policy | Use Case |
|------|----------|---------|-----------------|----------|
| **Tier 3** | Any | None | Livepeer images only | Standard AI inference |
| **Tier 2** | Any | gVisor | Verified custom images | Semi-trusted custom models |
| **Tier 1** | H100+ | CC mode | Any verified image | Sensitive data processing |

### 4.2 — Attestation Flow (CC-capable nodes only)

- Orchestrator generates NVIDIA CC attestation report on startup
- Attestation included in `OrchestratorInfo` response
- Gateway/client verifies attestation before sending sensitive data
- On-chain attestation registry (optional, longer term)

---

## File Change Summary

| Phase | Files Modified | Files Created |
|-------|---------------|---------------|
| **Phase 1** | `net/lp_rpc.proto`, `ai/worker/container.go`, `server/selection_algorithm.go`, `server/selection.go`, `discovery/discovery.go`, `server/rpc.go`, `core/orchestrator.go` | `ai/worker/gpu_classify.go` |
| **Phase 2** | `ai/worker/runtime.go`, `ai/worker/docker.go`, `net/lp_rpc.proto` | — |
| **Phase 3** | `ai/worker/docker.go`, `core/ai.go`, `core/capabilities.go` | `ai/worker/image_policy.go`, `ai/worker/container_interface.go` |

## Execution Order

**Phase 1 → Phase 2 → Phase 3** (sequential, each builds on the previous)

- Phase 1 is foundational — everything else depends on knowing what hardware you're routing to
- Phase 2 is independent of Phase 3 but should come first (sandbox before allowing custom containers)
- Phase 3 is the most complex and benefits from having the security foundation of Phase 2
