#!/usr/bin/env bash
set -euo pipefail

# E2E BYOC Test Script
# Tests the full pipeline: client -> gateway -> orchestrator -> adapter -> proxy -> backend
#
# Prerequisites: docker compose up (from this directory)

GATEWAY_URL="${GATEWAY_URL:-http://localhost:9935}"
ORCH_URL="${ORCH_URL:-http://localhost:8935}"
ADAPTER_URL="${ADAPTER_URL:-http://localhost:9090}"
CAPABILITY="${CAPABILITY:-test-inference}"
TIMEOUT="${TIMEOUT:-30}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass=0
fail=0

log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((pass++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((fail++)); }
log_info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# ============================================
# Test 1: Health checks
# ============================================
echo ""
echo "=========================================="
echo " BYOC E2E Tests"
echo "=========================================="
echo ""

log_info "Testing component health..."

# Orchestrator health
if curl -sf "${ORCH_URL}/status" > /dev/null 2>&1; then
    log_pass "Orchestrator is healthy at ${ORCH_URL}"
else
    log_fail "Orchestrator not reachable at ${ORCH_URL}"
fi

# Adapter health
adapter_health=$(curl -sf "${ADAPTER_URL}/health" 2>/dev/null || echo "UNREACHABLE")
if echo "$adapter_health" | grep -q "healthy\|ok\|true"; then
    log_pass "Inference adapter is healthy at ${ADAPTER_URL}"
else
    log_fail "Inference adapter not healthy: ${adapter_health}"
fi

# ============================================
# Test 2: Capability registered
# ============================================
log_info "Testing capability registration..."

# Check via adapter health (it reports registration status)
if echo "$adapter_health" | grep -qi "registered\|healthy"; then
    log_pass "Capability '${CAPABILITY}' appears registered"
else
    log_fail "Capability registration unclear: ${adapter_health}"
fi

# ============================================
# Test 3: Direct adapter test (bypass orchestrator)
# ============================================
log_info "Testing direct adapter inference..."

adapter_resp=$(curl -sf -X POST "${ADAPTER_URL}/inference" \
    -H "Content-Type: application/json" \
    -d '{"prompt": "a cat wearing a top hat", "num_inference_steps": 4}' \
    2>/dev/null || echo "ERROR")

if echo "$adapter_resp" | grep -q "output\|images\|text\|result"; then
    log_pass "Direct adapter inference works"
    log_info "Response: $(echo "$adapter_resp" | head -c 200)"
else
    log_fail "Direct adapter inference failed: ${adapter_resp}"
fi

# ============================================
# Test 4: Full E2E via Gateway (BYOC flow)
# ============================================
log_info "Testing full E2E via gateway..."

# Build the Livepeer header (base64 encoded job request)
# The header contains: request (JSON string), capability, timeout_seconds
livepeer_header=$(echo -n "{\"request\": \"{\\\"run\\\": \\\"echo\\\"}\", \"capability\": \"${CAPABILITY}\", \"timeout_seconds\": ${TIMEOUT}}" | base64)

gateway_resp=$(curl -sf -X POST "${GATEWAY_URL}/process/request/${CAPABILITY}" \
    -H "Content-Type: application/json" \
    -H "Livepeer: ${livepeer_header}" \
    -d '{"prompt": "a cat wearing a top hat", "num_inference_steps": 4}' \
    --max-time 30 \
    2>/dev/null || echo "ERROR")

if echo "$gateway_resp" | grep -q "output\|images\|text\|result"; then
    log_pass "Full E2E gateway->orchestrator->adapter->backend works!"
    log_info "Response: $(echo "$gateway_resp" | head -c 200)"
elif echo "$gateway_resp" | grep -qi "error\|ERROR"; then
    log_fail "E2E gateway test failed: ${gateway_resp}"
    log_info "This may be expected if gateway<->orchestrator BYOC routing is not fully wired in offchain mode"
    log_info "The direct adapter test above confirms the NaaP containers work correctly"
else
    log_fail "E2E gateway test got unexpected response: ${gateway_resp}"
fi

# ============================================
# Test 5: Get Job Token (orchestrator direct)
# ============================================
log_info "Testing job token from orchestrator..."

# Create a dummy ETH address header
eth_addr=$(echo -n '{"address":"0x0000000000000000000000000000000000000001"}' | base64)

token_resp=$(curl -sf "${ORCH_URL}/process/token" \
    -H "Livepeer-Eth-Address: ${eth_addr}" \
    -H "Livepeer-Capability: ${CAPABILITY}" \
    --max-time 10 \
    2>/dev/null || echo "ERROR")

if echo "$token_resp" | grep -qi "error\|ERROR\|not found"; then
    log_fail "Job token request failed: ${token_resp}"
    log_info "This may need the capability to be registered via the BYOC orchestrator server routes"
else
    log_pass "Job token endpoint responded"
    log_info "Token response: $(echo "$token_resp" | head -c 200)"
fi

# ============================================
# Summary
# ============================================
echo ""
echo "=========================================="
echo " Results: ${pass} passed, ${fail} failed"
echo "=========================================="
echo ""

if [ $fail -gt 0 ]; then
    log_info "Some tests failed. See GAPS section in the design doc for known issues."
    exit 1
fi
