#!/bin/bash
# Deploy the livepeer-stream-adapter to the BYOC VM
# This adapter bridges between the BYOC orchestrator and scope-trickle on fal.ai
#
# Prerequisites:
#   - VM at 34.134.195.88 with Docker
#   - BYOC orchestrator running on :8935
#   - scope-trickle deployed on fal.ai
#
# Usage: ./deploy-stream-adapter.sh

set -euo pipefail

VM_IP="34.134.195.88"
VM_USER="qiang_han_livepeer_org"
ADAPTER_PORT=9093
ORCH_URL="https://localhost:8935"
ORCH_SECRET="${ORCH_SECRET:-secret}"
FAL_KEY="${FAL_KEY:-513f7292-e6a3-40b4-836f-32a644919d6f:c055899e3c22b36a8309932d83f7163d}"
SCOPE_TRICKLE_ENDPOINT="wss://Daydream-scope-trickle.gateway.alpha.fal.ai/ws"

ADAPTER_SRC="/Users/qiang.han/Documents/mycodespace/NaaP/containers/livepeer-stream-adapter"

echo "=== Deploying Stream Adapter to ${VM_IP} ==="

# 1. Copy adapter source to VM
echo "Copying adapter source..."
rsync -avz --exclude '__pycache__' --exclude '.git' --exclude '*.pyc' \
  "${ADAPTER_SRC}/" "${VM_USER}@${VM_IP}:~/livepeer-stream-adapter/"

# 2. Install deps and run on VM
echo "Starting adapter on VM..."
ssh "${VM_USER}@${VM_IP}" << 'REMOTE_EOF'
cd ~/livepeer-stream-adapter

# Install Python deps
pip3 install -e . websockets aiohttp numpy av 2>/dev/null || true

# Stop existing adapter if running
pkill -f "stream_server" 2>/dev/null || true
sleep 1

# Start adapter with scope-trickle configuration
export ADAPTER_PORT=9093
export ORCH_URL="https://localhost:8935"
export ORCH_SECRET="${ORCH_SECRET:-secret}"
export FAL_KEY="${FAL_KEY}"
export SCOPE_TRICKLE_ENDPOINT="wss://Daydream-scope-trickle.gateway.alpha.fal.ai/ws"
export ADAPTER_CALLBACK_URL="http://localhost:9093"
export CAPABILITIES='[{"name":"scope-live","model_id":"scope-trickle","capacity":2}]'

nohup python3 -m livepeer_stream_adapter.stream_server > /tmp/stream-adapter.log 2>&1 &

echo "Stream adapter started on port 9093"
echo "Logs: /tmp/stream-adapter.log"
REMOTE_EOF

echo "=== Deployment complete ==="
echo ""
echo "Test health: ssh ${VM_USER}@${VM_IP} curl http://localhost:9093/health"
echo "View logs:   ssh ${VM_USER}@${VM_IP} tail -f /tmp/stream-adapter.log"
