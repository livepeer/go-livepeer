# Real-Time AI Container Orchestration — Implementation Plan

## Overview

This document describes the plan for adding hardware-aware orchestrator selection, container sandboxing, and custom container support to go-livepeer's AI inference pipeline. It is designed to be referenced by other components in the Livepeer AI stack.

### Integration Points

| Component | Interface | Description |
|---|---|---|
| **Gateway / Router** | `OrchestratorInfo.hardware[]` | GPU tier + sandbox info for job routing |
| **AI Runner Containers** | HTTP contract (`/health`, `/hardware/info`, `POST /{pipeline}`) | Standard interface all AI containers implement |
| **On-chain Registry** | `Capability` bitstring + constraints | Discovery of custom model capabilities |
| **Payment Module** | `SetBasePriceForCap` | Per-model / per-tier pricing |
| **BYOC Framework** | `Capability_BYOC` + `ExternalCapabilities` | Third-party container registration |

---

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
- Container isolation enforcement appropriate for GPU workloads
- Custom container image support (currently hardcoded to `livepeer/ai-runner:*` variants)
- Container image verification (signatures, allowlists)
- Resource limits and network isolation per container

---

## Container Isolation Strategy

### The GPU Problem

AI inference containers need direct GPU access via NVIDIA kernel modules. This constrains which isolation technologies are viable:

| Approach | GPU Support | Isolation Level | Overhead | Status |
|---|---|---|---|---|
| **Kata + Cloud Hypervisor** | Full (VFIO passthrough) | Strong (lightweight VM) | Low-medium | Recommended |
| **Kata + QEMU** | Full (VFIO passthrough) | Strong (VM) | Medium | Mature fallback |
| **gVisor + nvproxy** | Partial (experimental) | Medium (syscall filter) | Low | Limited CUDA support |
| **Plain containers + NVIDIA runtime** | Full | Weak (namespaces only) | Lowest | Current default |

### Recommendation

**Primary: Kata Containers with Cloud Hypervisor** for orchestrators running untrusted or custom AI containers. Kata provides real VM isolation while supporting full GPU passthrough via VFIO, which is essential for CUDA workloads.

**Fallback: gVisor** remains useful for non-GPU containers (preprocessing, postprocessing, lightweight models on CPU). The existing `ai/worker/runtime.go` gVisor integration should be retained for these workloads.

**Rationale:** gVisor intercepts syscalls in userspace. GPU access requires direct kernel driver interaction (NVIDIA kernel modules), and gVisor's `nvproxy` is experimental and does not support all CUDA operations needed for production AI inference. Kata VMs run a real Linux kernel, so NVIDIA drivers work natively with VFIO GPU passthrough.

### Isolation Tiers (used throughout this plan)

| Tier | Runtime | GPU | Use Case |
|---|---|---|---|
| **Tier 0 — None** | Default Docker (runc) | NVIDIA runtime | Trusted Livepeer images, development |
| **Tier 1 — Syscall Filter** | gVisor (runsc) | nvproxy (limited) | CPU-only or light GPU tasks |
| **Tier 2 — VM Isolation** | Kata + Cloud Hypervisor | VFIO passthrough | Custom/untrusted containers needing GPU |
| **Tier 3 — Confidential** | Kata + CC hardware | VFIO + CC attestation | Sensitive data, privacy-required inference |

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
  GPUTier tier = 8;
  bool cc_enabled = 9;          // Actually running in CC mode
  bytes attestation_report = 10; // Optional CC attestation
}
```

### 1.2 — GPU Detection & Classification

**New file:** `ai/worker/gpu_classify.go`

- Map GPU names to tiers using a lookup table (e.g., "H100" -> CC_CAPABLE, "A100" -> DATACENTER, "RTX 4090" -> CONSUMER)
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

## Phase 2: Container Isolation Enforcement

**Goal:** Containers run with appropriate isolation based on trust level and GPU requirements.

### 2.1 — Kata Containers Integration

**New file:** `ai/worker/runtime_kata.go`

- Detect Kata runtime availability (`kata-runtime` or `kata-fc` in Docker daemon config)
- Configure VFIO GPU passthrough for Kata containers
- Health check that Kata runtime + GPU passthrough is functional before accepting isolated jobs
- Operator config: `--ai-runtime kata|gvisor|runc` (default: `runc`)

### 2.2 — Retain and Harden gVisor for CPU Workloads

**File:** `ai/worker/runtime.go`

- Current state: `--ai-runtime gvisor` flag exists, downloads runsc binary
- Keep gVisor as option for non-GPU or light-GPU containers
- Add: Verify runsc binary integrity (checksum verification already exists, ensure it's enforced)
- Add: Health check that gVisor runtime is actually functional before accepting jobs
- Add: Clear logging when gVisor is selected but GPU workload is requested (warn about nvproxy limitations)

### 2.3 — Per-Container Runtime Selection

**File:** `ai/worker/docker.go`

In `createContainer()` (~line 400), set `HostConfig.Runtime` based on:
1. **Container image trust level:** Livepeer-signed -> runc (Tier 0), unknown -> Kata (Tier 2)
2. **GPU requirement:** GPU-heavy -> Kata (not gVisor), CPU-only -> gVisor is fine
3. **Operator config:** `--ai-runtime` flag as override
4. **Job request:** Gateway can request specific isolation tier

Add runtime field to `RunnerContainer` struct for tracking.

### 2.4 — Container Security Hardening (All Runtimes)

**File:** `ai/worker/docker.go`

Add to `createContainer()` HostConfig regardless of runtime:
- Seccomp profile (default Docker profile at minimum)
- Drop all Linux capabilities, only add back what's needed (`SYS_ADMIN` for GPU)
- Read-only root filesystem where possible
- Network isolation: `--network none` for inference containers (they only need GPU + model volume)
- Memory/CPU limits from operator config
- PID limits to prevent fork bombs

### 2.5 — Report Isolation Status

**File:** `net/lp_rpc.proto`

```protobuf
enum IsolationType {
  ISOLATION_NONE = 0;       // runc (default Docker)
  ISOLATION_GVISOR = 1;     // Syscall filtering
  ISOLATION_KATA = 2;       // VM-based (Cloud Hypervisor / QEMU)
  ISOLATION_CC = 3;         // Confidential Computing (Kata + CC hardware)
}

// Add to HardwareInformation or OrchestratorInfo:
IsolationType isolation_type = N;
```

Gateway can then route sensitive workloads to appropriately isolated orchestrators.

**Deliverable:** Containers run with isolation appropriate to their trust level and GPU needs. Gateways know which orchestrators offer what isolation.

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
- **Custom containers requiring GPU must use Kata isolation (Tier 2+)** unless explicitly trusted via image policy

### 3.3 — Container Interface Contract

**New file:** `ai/worker/container_interface.go`

Define the HTTP API contract that custom containers must implement:
- `GET /health` -> `{"status": "IDLE"|"LOADING"|"ERROR"}`
- `GET /hardware/info` -> `HardwareInformation` JSON
- `POST /{pipeline}` -> Pipeline-specific request/response
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

### 4.1 — Unified Trust Model

Combine phases 1-3 into enforcement:

| Trust Tier | Hardware | Isolation | Container Policy | Use Case |
|---|---|---|---|---|
| **Tier 3 — Open** | Any | None (runc) | Livepeer images only | Standard AI inference |
| **Tier 2 — Sandboxed** | Any | Kata VM | Verified custom images | Semi-trusted custom models |
| **Tier 1 — Confidential** | H100+ | CC mode + Kata | Any verified image | Sensitive data processing |

### 4.2 — Attestation Flow (CC-capable nodes only)

- Orchestrator generates NVIDIA CC attestation report on startup
- Attestation included in `OrchestratorInfo` response
- Gateway/client verifies attestation before sending sensitive data
- On-chain attestation registry (optional, longer term)

### 4.3 — Monitoring & Observability

- Per-container isolation type in Prometheus metrics
- Alert on isolation downgrades (Kata unavailable, falling back to runc)
- Container escape detection (unexpected host namespace access)

---

## File Change Summary

| Phase | Files Modified | Files Created |
|---|---|---|
| **Phase 1** | `net/lp_rpc.proto`, `ai/worker/container.go`, `server/selection_algorithm.go`, `server/selection.go`, `discovery/discovery.go`, `server/rpc.go`, `core/orchestrator.go` | `ai/worker/gpu_classify.go` |
| **Phase 2** | `ai/worker/runtime.go`, `ai/worker/docker.go`, `net/lp_rpc.proto` | `ai/worker/runtime_kata.go` |
| **Phase 3** | `ai/worker/docker.go`, `core/ai.go`, `core/capabilities.go` | `ai/worker/image_policy.go`, `ai/worker/container_interface.go` |

## Execution Order

**Phase 1 -> Phase 2 -> Phase 3** (sequential, each builds on the previous)

- Phase 1 is foundational — everything else depends on knowing what hardware you're routing to
- Phase 2 is independent of Phase 3 but should come first (establish isolation before allowing custom containers)
- Phase 3 is the most complex and benefits from having the security foundation of Phase 2

## Key Decisions & Trade-offs

### Why Kata over gVisor for GPU workloads

gVisor intercepts syscalls in userspace and uses `nvproxy` for experimental GPU support. This approach has fundamental limitations for production CUDA workloads:

1. **nvproxy coverage** — Not all NVIDIA ioctls are implemented; complex CUDA operations (multi-stream, unified memory, NCCL) may fail
2. **Driver compatibility** — Each NVIDIA driver update can introduce new ioctls that nvproxy doesn't handle
3. **Performance** — Syscall interception adds latency to every GPU kernel launch

Kata with VFIO passthrough gives the GPU direct access to a real kernel, so all CUDA operations work natively with no compatibility risk.

### When gVisor is still the right choice

- CPU-only inference (ONNX Runtime, lightweight models)
- Preprocessing/postprocessing containers
- Containers that don't need GPU access
- Development/testing environments where Kata is not available
