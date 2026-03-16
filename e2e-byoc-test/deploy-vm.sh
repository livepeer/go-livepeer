#!/bin/bash
# Deploy BYOC stack to a remote VM for offchain testing
#
# Prerequisites:
#   - A VM with Docker + Docker Compose installed (any cloud provider)
#   - SSH access to the VM
#   - The VM's public IP
#
# Usage:
#   ./deploy-vm.sh <VM_IP> <FAL_KEY> [GEMINI_KEY]
#
# What this does:
#   1. Copies docker-compose and container source to the VM
#   2. Builds and starts all 3 containers (offchain, no wallet needed)
#   3. The orchestrator listens on :8935 (HTTPS, self-signed cert)
#   4. The agent-client.py can connect from anywhere
#
# Example:
#   ./deploy-vm.sh 34.56.78.90 "your-fal-key" "your-gemini-key"
#   ORCH_URL=https://34.56.78.90:8935 python3 agent-client.py reg ls

set -euo pipefail

VM_IP="${1:?Usage: $0 <VM_IP> <FAL_KEY> [GEMINI_KEY] [REPLICATE_KEY] [RUNPOD_KEY]}"
FAL_KEY="${2:?Usage: $0 <VM_IP> <FAL_KEY> [GEMINI_KEY] [REPLICATE_KEY] [RUNPOD_KEY]}"
GEMINI_KEY="${3:-}"
REPLICATE_KEY="${4:-}"
RUNPOD_KEY="${5:-}"
SSH_USER="${SSH_USER:-root}"
REMOTE_DIR="/opt/byoc"

echo "==> Deploying BYOC stack to ${SSH_USER}@${VM_IP}"

# --- Step 1: Create remote directory structure ---
echo "==> Creating remote directories..."
ssh "${SSH_USER}@${VM_IP}" "mkdir -p ${REMOTE_DIR}"

# --- Step 2: Copy files ---
echo "==> Copying files to VM..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Copy docker-compose (production version, uses pre-built images)
cat <<'COMPOSE' | ssh "${SSH_USER}@${VM_IP}" "cat > ${REMOTE_DIR}/docker-compose.yaml"
# BYOC Offchain Stack -- deployed to VM
# All containers on one machine, localhost networking

services:
  orchestrator:
    image: livepeer/go-livepeer:latest
    platform: linux/amd64
    container_name: byoc_orch
    restart: unless-stopped
    command: >
      -orchestrator
      -network offchain
      -serviceAddr 0.0.0.0:8935
      -cliAddr 0.0.0.0:7935
      -orchSecret ${ORCH_SECRET}
      -v 6
    ports:
      - "8935:8935"
      - "7935:7935"

  serverless-proxy:
    image: ${PROXY_IMAGE:-livepeer-serverless-proxy:latest}
    container_name: byoc_proxy
    restart: unless-stopped
    environment:
      PROVIDER: fal-ai
      API_KEY: ${FAL_KEY}
      PORT: "8080"
      EXTRA_PROVIDERS: '{"gemini": {"api_key": "${GEMINI_KEY}"}}'
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:8080/health')"]
      interval: 5s
      timeout: 3s
      retries: 10

  inference-adapter:
    image: ${ADAPTER_IMAGE:-livepeer-inference-adapter:latest}
    container_name: byoc_adapter
    restart: unless-stopped
    environment:
      ORCH_URL: https://byoc_orch:8935
      ORCH_SECRET: ${ORCH_SECRET}
      BACKEND_URL: http://byoc_proxy:8080
      BACKEND_INFERENCE_PATH: /inference
      BACKEND_TIMEOUT: "300"
      ADAPTER_PORT: "9090"
      ADAPTER_CALLBACK_URL: http://byoc_adapter:9090
      HEALTH_CHECK_INTERVAL: "5"
      REGISTER_INTERVAL: "15"
      CAPABILITIES: >
        [
          {"name":"nano-banana","model_id":"fal-ai/nano-banana-2","capacity":5},
          {"name":"recraft-v4","model_id":"fal-ai/recraft/v4/pro/text-to-image","capacity":5},
          {"name":"gemini-image","model_id":"gemini/gemini-2.5-flash-image","capacity":5},
          {"name":"gemini-text","model_id":"gemini/gemini-2.0-flash","capacity":10},
          {"name":"flux-schnell","model_id":"fal-ai/flux/schnell","capacity":5},
          {"name":"flux-dev","model_id":"fal-ai/flux/dev","capacity":3},
          {"name":"flux-pro","model_id":"fal-ai/flux-2-pro","capacity":3},
          {"name":"qwen-image","model_id":"fal-ai/qwen-image-2/text-to-image","capacity":3},
          {"name":"kontext-edit","model_id":"fal-ai/flux-pro/kontext","capacity":3},
          {"name":"reve-edit","model_id":"fal-ai/reve/edit","capacity":3},
          {"name":"topaz-upscale","model_id":"fal-ai/topaz/upscale/image","capacity":3},
          {"name":"bg-remove","model_id":"fal-ai/bria/background/remove","capacity":5},
          {"name":"ltx-t2v","model_id":"fal-ai/ltx-2.3/text-to-video/fast","capacity":3},
          {"name":"ltx-t2v-23","model_id":"fal-ai/ltx-2.3/text-to-video","capacity":3},
          {"name":"ltx-i2v","model_id":"fal-ai/ltx-2.3/image-to-video","capacity":3},
          {"name":"lucy-i2v","model_id":"decart/lucy-14b/image-to-video","capacity":3},
          {"name":"wan-v2v","model_id":"fal-ai/wan/v2.2-a14b/video-to-video","capacity":3},
          {"name":"kling-i2v","model_id":"fal-ai/kling-video/v3/pro/image-to-video","capacity":2},
          {"name":"fast-lcm","model_id":"fal-ai/fast-lcm-diffusion","capacity":5},
          {"name":"chatterbox-tts","model_id":"fal-ai/chatterbox/text-to-speech","capacity":3},
          {"name":"lux-tts","model_id":"fal-ai/lux-tts","capacity":3}
        ]
    ports:
      - "9090:9090"
    depends_on:
      - orchestrator
      - serverless-proxy
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:9090/health')"]
      interval: 5s
      timeout: 3s
      retries: 15

  # --- Replicate provider ---
  serverless-proxy-replicate:
    image: ${PROXY_IMAGE:-livepeer-serverless-proxy:latest}
    container_name: byoc_proxy_replicate
    restart: unless-stopped
    environment:
      PROVIDER: replicate
      API_KEY: ${REPLICATE_KEY}
      PORT: "8080"
    ports:
      - "8081:8080"
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:8080/health')"]
      interval: 5s
      timeout: 3s
      retries: 10

  inference-adapter-replicate:
    image: ${ADAPTER_IMAGE:-livepeer-inference-adapter:latest}
    container_name: byoc_adapter_replicate
    restart: unless-stopped
    environment:
      ORCH_URL: https://byoc_orch:8935
      ORCH_SECRET: ${ORCH_SECRET}
      BACKEND_URL: http://byoc_proxy_replicate:8080
      BACKEND_INFERENCE_PATH: /inference
      BACKEND_TIMEOUT: "300"
      ADAPTER_PORT: "9091"
      ADAPTER_CALLBACK_URL: http://byoc_adapter_replicate:9091
      HEALTH_CHECK_INTERVAL: "5"
      REGISTER_INTERVAL: "15"
      CAPABILITIES: >
        [
          {"name":"seedream-4","model_id":"bytedance/seedream-4","capacity":3},
          {"name":"z-image-turbo","model_id":"prunaai/z-image-turbo","capacity":3},
          {"name":"imagen-4","model_id":"google/imagen-4","capacity":3},
          {"name":"hunyuan-image-3","model_id":"tencent/hunyuan-image-3","capacity":3},
          {"name":"gen-4.5","model_id":"runwayml/gen-4.5","capacity":2},
          {"name":"pixverse-v5.6","model_id":"pixverse/pixverse-v5.6","capacity":2},
          {"name":"dreamactor-m2","model_id":"bytedance/dreamactor-m2.0","capacity":2},
          {"name":"gpt-5-nano","model_id":"openai/gpt-5-nano","capacity":5},
          {"name":"dall-e-3","model_id":"openai/dall-e-3","capacity":3},
          {"name":"claude-opus","model_id":"anthropic/claude-opus-4.6","capacity":3}
        ]
    ports:
      - "9091:9091"
    depends_on:
      - orchestrator
      - serverless-proxy-replicate

  # --- RunPod provider ---
  serverless-proxy-runpod:
    image: ${PROXY_IMAGE:-livepeer-serverless-proxy:latest}
    container_name: byoc_proxy_runpod
    restart: unless-stopped
    environment:
      PROVIDER: runpod
      API_KEY: ${RUNPOD_KEY}
      MODEL_ID: lx5lawokk3qfle
      PORT: "8080"
    ports:
      - "8082:8080"
    healthcheck:
      test: ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://localhost:8080/health')"]
      interval: 5s
      timeout: 3s
      retries: 10

  inference-adapter-runpod:
    image: ${ADAPTER_IMAGE:-livepeer-inference-adapter:latest}
    container_name: byoc_adapter_runpod
    restart: unless-stopped
    environment:
      ORCH_URL: https://byoc_orch:8935
      ORCH_SECRET: ${ORCH_SECRET}
      BACKEND_URL: http://byoc_proxy_runpod:8080
      BACKEND_INFERENCE_PATH: /inference
      BACKEND_TIMEOUT: "300"
      ADAPTER_PORT: "9092"
      ADAPTER_CALLBACK_URL: http://byoc_adapter_runpod:9092
      HEALTH_CHECK_INTERVAL: "5"
      REGISTER_INTERVAL: "15"
      CAPABILITIES: >
        [
          {"name":"wan-animate","model_id":"lx5lawokk3qfle","capacity":2}
        ]
    ports:
      - "9092:9092"
    depends_on:
      - orchestrator
      - serverless-proxy-runpod

  # --- Scope live streaming adapter ---
  stream-adapter:
    image: ${STREAM_ADAPTER_IMAGE:-livepeer-stream-adapter:latest}
    container_name: byoc_stream_adapter
    restart: unless-stopped
    environment:
      ORCH_URL: https://byoc_orch:8935
      ORCH_SECRET: ${ORCH_SECRET}
      FAL_KEY: ${FAL_KEY}
      ADAPTER_PORT: "9093"
      ADAPTER_CALLBACK_URL: http://byoc_stream_adapter:9093
      REGISTER_INTERVAL: "15"
      CAPABILITIES: >
        [
          {"name":"scope-live","model_id":"daydream/scope-app","capacity":2}
        ]
    ports:
      - "9093:9093"
    depends_on:
      - orchestrator

networks:
  default:
    name: byoc_network
COMPOSE

# --- Step 3: Create .env file ---
echo "==> Writing .env..."
ssh "${SSH_USER}@${VM_IP}" "cat > ${REMOTE_DIR}/.env" <<ENV
FAL_KEY=${FAL_KEY}
GEMINI_KEY=${GEMINI_KEY}
REPLICATE_KEY=${REPLICATE_KEY}
RUNPOD_KEY=${RUNPOD_KEY}
ORCH_SECRET=offchain-test-secret-$(openssl rand -hex 8)
ENV

# --- Step 4: Build proxy and adapter images on VM ---
echo "==> Copying container source for build..."
NAAP_DIR="${SCRIPT_DIR}/../../../NaaP/containers"
scp -r "${NAAP_DIR}/livepeer-serverless-proxy" "${SSH_USER}@${VM_IP}:${REMOTE_DIR}/"
scp -r "${NAAP_DIR}/livepeer-inference-adapter" "${SSH_USER}@${VM_IP}:${REMOTE_DIR}/"
scp -r "${NAAP_DIR}/livepeer-stream-adapter" "${SSH_USER}@${VM_IP}:${REMOTE_DIR}/"

echo "==> Building images on VM..."
ssh "${SSH_USER}@${VM_IP}" "cd ${REMOTE_DIR} && \
  docker build -t livepeer-serverless-proxy:latest ./livepeer-serverless-proxy && \
  docker build -t livepeer-inference-adapter:latest ./livepeer-inference-adapter && \
  docker build -t livepeer-stream-adapter:latest ./livepeer-stream-adapter"

# --- Step 5: Start the stack ---
echo "==> Starting BYOC stack..."
ssh "${SSH_USER}@${VM_IP}" "cd ${REMOTE_DIR} && docker compose up -d"

echo ""
echo "==> Waiting for services to be healthy..."
sleep 10

# --- Step 6: Verify ---
ssh "${SSH_USER}@${VM_IP}" "cd ${REMOTE_DIR} && docker compose ps"

echo ""
echo "============================================"
echo " BYOC stack deployed to ${VM_IP}"
echo "============================================"
echo ""
echo " Orchestrator: https://${VM_IP}:8935"
echo " Adapter:      http://${VM_IP}:9090"
echo ""
echo " Test from your laptop:"
echo "   ORCH_URL=https://${VM_IP}:8935 ADAPTER_URL=http://${VM_IP}:9090 python3 agent-client.py reg ls"
echo "   ORCH_URL=https://${VM_IP}:8935 ADAPTER_URL=http://${VM_IP}:9090 python3 agent-client.py --task \"a cat as image using nano\""
echo ""
echo " SSH into the VM:"
echo "   ssh ${SSH_USER}@${VM_IP}"
echo "   cd ${REMOTE_DIR} && docker compose logs -f"
echo ""
