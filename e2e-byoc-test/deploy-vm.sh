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

VM_IP="${1:?Usage: $0 <VM_IP> <FAL_KEY> [GEMINI_KEY]}"
FAL_KEY="${2:?Usage: $0 <VM_IP> <FAL_KEY> [GEMINI_KEY]}"
GEMINI_KEY="${3:-}"
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
          {"name":"gemini-image","model_id":"gemini/gemini-2.0-flash-exp-image-generation","capacity":5},
          {"name":"ltx-t2v","model_id":"fal-ai/ltx-2.3/text-to-video/fast","capacity":3},
          {"name":"ltx-t2v-23","model_id":"fal-ai/ltx-2.3/text-to-video","capacity":3},
          {"name":"ltx-i2v","model_id":"fal-ai/ltx-2.3/image-to-video","capacity":3},
          {"name":"lucy-i2v","model_id":"decart/lucy-14b/image-to-video","capacity":3},
          {"name":"wan-v2v","model_id":"fal-ai/wan/v2.2-a14b/video-to-video","capacity":3}
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

networks:
  default:
    name: byoc_network
COMPOSE

# --- Step 3: Create .env file ---
echo "==> Writing .env..."
ssh "${SSH_USER}@${VM_IP}" "cat > ${REMOTE_DIR}/.env" <<ENV
FAL_KEY=${FAL_KEY}
GEMINI_KEY=${GEMINI_KEY}
ORCH_SECRET=offchain-test-secret-$(openssl rand -hex 8)
ENV

# --- Step 4: Build proxy and adapter images on VM ---
echo "==> Copying container source for build..."
NAAP_DIR="${SCRIPT_DIR}/../../../NaaP/containers"
scp -r "${NAAP_DIR}/livepeer-serverless-proxy" "${SSH_USER}@${VM_IP}:${REMOTE_DIR}/"
scp -r "${NAAP_DIR}/livepeer-inference-adapter" "${SSH_USER}@${VM_IP}:${REMOTE_DIR}/"

echo "==> Building images on VM..."
ssh "${SSH_USER}@${VM_IP}" "cd ${REMOTE_DIR} && \
  docker build -t livepeer-serverless-proxy:latest ./livepeer-serverless-proxy && \
  docker build -t livepeer-inference-adapter:latest ./livepeer-inference-adapter"

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
