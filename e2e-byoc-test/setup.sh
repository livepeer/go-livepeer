#!/usr/bin/env bash
set -euo pipefail

# E2E BYOC Test Setup Script
#
# Builds and starts all containers for local E2E testing.
# No real LPT, no staking, no tickets - pure offchain mode.
#
# Usage:
#   # Mock backend (no API key needed):
#   ./setup.sh
#
#   # With fal.ai:
#   FAL_KEY=your-key ./setup.sh fal
#
#   # With custom HTTP endpoint:
#   ENDPOINT_URL=http://your-server:8080 ./setup.sh custom
#
#   # Tear down:
#   ./setup.sh down
#
#   # View logs:
#   ./setup.sh logs

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${YELLOW}[INFO]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $1"; }
log_err()   { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    cat <<EOF
Usage: ./setup.sh [command] [options]

Commands:
  (default)   Start with mock backend
  fal         Start with fal.ai backend (requires FAL_KEY env var)
  custom      Start with custom HTTP endpoint (requires ENDPOINT_URL env var)
  down        Tear down all containers
  logs        Tail all container logs
  status      Show container status
  test        Run E2E tests

Environment Variables:
  FAL_KEY       - fal.ai API key (for 'fal' command)
  MODEL_ID      - Model ID for fal.ai (default: fal-ai/flux/dev)
  ENDPOINT_URL  - Custom endpoint URL (for 'custom' command)
  CAPABILITY_NAME - Capability name to register (default: test-inference)
EOF
}

wait_for_health() {
    local url=$1
    local name=$2
    local max_wait=${3:-120}
    local elapsed=0

    log_info "Waiting for ${name} to be healthy..."
    while [ $elapsed -lt $max_wait ]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            log_ok "${name} is healthy (${elapsed}s)"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    log_err "${name} did not become healthy within ${max_wait}s"
    return 1
}

cmd_up_mock() {
    log_info "Starting E2E BYOC test stack with MOCK backend..."
    log_info "No API keys needed. Pure offchain mode."
    echo ""

    export PROVIDER=custom
    export ENDPOINT_URL=http://mock-backend:8080
    export CAPABILITY_NAME=${CAPABILITY_NAME:-test-inference}

    docker compose up --build -d

    echo ""
    wait_for_health "http://localhost:8935/status" "Orchestrator" 120 || true
    wait_for_health "http://localhost:9090/health" "Inference Adapter" 60 || true

    echo ""
    log_ok "Stack is running!"
    echo ""
    echo "  Orchestrator:  http://localhost:8935"
    echo "  Gateway:       http://localhost:9935"
    echo "  Adapter:       http://localhost:9090"
    echo "  Mock Backend:  http://localhost:8080"
    echo ""
    echo "  Run tests:  ./test-byoc.sh"
    echo "  View logs:  ./setup.sh logs"
    echo "  Tear down:  ./setup.sh down"
}

cmd_up_fal() {
    if [ -z "${FAL_KEY:-}" ]; then
        log_err "FAL_KEY environment variable is required for fal.ai mode"
        echo "  Usage: FAL_KEY=your-key ./setup.sh fal"
        exit 1
    fi

    log_info "Starting E2E BYOC test stack with fal.ai backend..."
    echo ""

    export PROVIDER=fal-ai
    export MODEL_ID=${MODEL_ID:-fal-ai/flux/dev}
    export CAPABILITY_NAME=${CAPABILITY_NAME:-flux-dev}
    # No mock-backend needed for fal.ai
    export ENDPOINT_URL=""

    docker compose up --build -d orchestrator gateway serverless-proxy inference-adapter

    echo ""
    wait_for_health "http://localhost:8935/status" "Orchestrator" 120 || true
    wait_for_health "http://localhost:9090/health" "Inference Adapter" 60 || true

    echo ""
    log_ok "Stack is running with fal.ai!"
    echo ""
    echo "  Orchestrator:  http://localhost:8935"
    echo "  Gateway:       http://localhost:9935"
    echo "  Adapter:       http://localhost:9090"
    echo "  Provider:      fal.ai (${MODEL_ID})"
    echo "  Capability:    ${CAPABILITY_NAME}"
    echo ""
    echo "  Run tests:  CAPABILITY=${CAPABILITY_NAME} ./test-byoc.sh"
}

cmd_up_custom() {
    if [ -z "${ENDPOINT_URL:-}" ]; then
        log_err "ENDPOINT_URL environment variable is required for custom mode"
        echo "  Usage: ENDPOINT_URL=http://your-server:8080 ./setup.sh custom"
        exit 1
    fi

    log_info "Starting E2E BYOC test stack with custom endpoint..."
    echo ""

    export PROVIDER=custom
    export CAPABILITY_NAME=${CAPABILITY_NAME:-custom-inference}

    docker compose up --build -d orchestrator gateway serverless-proxy inference-adapter

    echo ""
    wait_for_health "http://localhost:8935/status" "Orchestrator" 120 || true
    wait_for_health "http://localhost:9090/health" "Inference Adapter" 60 || true

    echo ""
    log_ok "Stack is running with custom endpoint!"
    echo ""
    echo "  Orchestrator:  http://localhost:8935"
    echo "  Gateway:       http://localhost:9935"
    echo "  Custom Backend: ${ENDPOINT_URL}"
}

cmd_down() {
    log_info "Tearing down E2E BYOC test stack..."
    docker compose down -v --remove-orphans
    log_ok "All containers stopped and removed"
}

cmd_logs() {
    docker compose logs -f --tail=100
}

cmd_status() {
    docker compose ps
}

cmd_test() {
    if [ -f "./test-byoc.sh" ]; then
        bash ./test-byoc.sh
    else
        log_err "test-byoc.sh not found"
        exit 1
    fi
}

# Main
case "${1:-mock}" in
    mock|"")     cmd_up_mock ;;
    fal)         cmd_up_fal ;;
    custom)      cmd_up_custom ;;
    down)        cmd_down ;;
    logs)        cmd_logs ;;
    status)      cmd_status ;;
    test)        cmd_test ;;
    -h|--help)   usage ;;
    *)           usage; exit 1 ;;
esac
