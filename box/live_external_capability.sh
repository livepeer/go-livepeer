#!/bin/bash

# Script to register a "live" external capability worker to a Livepeer orchestrator
# This script sends worker configuration to the orchestrator's /capability endpoint

set -e

# Default values
ORCHESTRATOR_URL="http://localhost:8935"
WORKER_URL="http://localhost:8000"
CAPABILITY_NAME="live"
PRICE_PER_UNIT=0
PRICE_SCALING=1
CAPACITY=10
WARM=true
EXTERNAL=true
SECRET=""

# Function to display usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Register a 'live' external capability worker to a Livepeer orchestrator"
    echo ""
    echo "Options:"
    echo "  -o, --orchestrator URL    Orchestrator URL (default: $ORCHESTRATOR_URL)"
    echo "  -w, --worker URL          Worker URL (default: $WORKER_URL)"
    echo "  -n, --name NAME           Capability name (default: $CAPABILITY_NAME)"
    echo "  -p, --price PRICE         Price per unit (default: $PRICE_PER_UNIT)"
    echo "  -s, --scaling SCALING     Price scaling factor (default: $PRICE_SCALING)"
    echo "  -c, --capacity CAPACITY   Worker capacity (default: $CAPACITY)"
    echo "  -S, --secret SECRET       Authorization secret (required)"
    echo "  -h, --help                Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --secret mysecret"
    echo "  $0 --orchestrator http://orch.example.com:8935 --worker http://worker.example.com:8000 --secret mysecret"
    echo "  $0 --name live-streaming --price 200 --capacity 20 --secret mysecret"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--orchestrator)
            ORCHESTRATOR_URL="$2"
            shift 2
            ;;
        -w|--worker)
            WORKER_URL="$2"
            shift 2
            ;;
        -n|--name)
            CAPABILITY_NAME="$2"
            shift 2
            ;;
        -p|--price)
            PRICE_PER_UNIT="$2"
            shift 2
            ;;
        -s|--scaling)
            PRICE_SCALING="$2"
            shift 2
            ;;
        -c|--capacity)
            CAPACITY="$2"
            shift 2
            ;;
        -S|--secret)
            SECRET="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [[ -z "$SECRET" ]]; then
    echo "Error: Authorization secret is required. Use --secret option."
    echo ""
    usage
    exit 1
fi

# Remove trailing slashes from URLs
ORCHESTRATOR_URL=${ORCHESTRATOR_URL%/}
WORKER_URL=${WORKER_URL%/}

# Create the external capability configuration JSON
CAPABILITY_CONFIG=$(cat <<EOF
{
  "name": "$CAPABILITY_NAME",
  "type": "live",
  "url": "$WORKER_URL",
  "price_per_unit": $PRICE_PER_UNIT,
  "price_scaling": $PRICE_SCALING,
  "capacity": $CAPACITY,
}
EOF
)

# Display configuration
echo "Registering external capability with the following configuration:"
echo "  Orchestrator URL: $ORCHESTRATOR_URL"
echo "  Worker URL: $WORKER_URL"
echo "  Capability Name: $CAPABILITY_NAME"
echo "  Type: live"
echo "  Price Per Unit: $PRICE_PER_UNIT"
echo "  Price Scaling: $PRICE_SCALING"
echo "  Capacity: $CAPACITY"
echo ""

# Send the registration request
echo "Sending registration request..."
RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}\n" \
    -X POST \
    -H "Content-Type: application/json" \
    -H "Authorization: $SECRET" \
    -d "$CAPABILITY_CONFIG" \
    "$ORCHESTRATOR_URL/capability/register" 2>/dev/null)

# Extract HTTP status code
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

# Check response
if [[ "$HTTP_STATUS" == "200" ]]; then
    echo "✅ Successfully registered external capability '$CAPABILITY_NAME'"
    echo "Response: $RESPONSE_BODY"
elif [[ "$HTTP_STATUS" == "400" ]]; then
    echo "❌ Registration failed with bad request (400)"
    echo "Response: $RESPONSE_BODY"
    echo ""
    echo "Common causes:"
    echo "  - Invalid JSON configuration"
    echo "  - Missing required fields"
    echo "  - Invalid authorization secret"
    exit 1
elif [[ "$HTTP_STATUS" == "401" ]] || [[ "$HTTP_STATUS" == "403" ]]; then
    echo "❌ Registration failed with authorization error ($HTTP_STATUS)"
    echo "Response: $RESPONSE_BODY"
    echo ""
    echo "Please check your authorization secret."
    exit 1
else
    echo "❌ Registration failed with HTTP status: $HTTP_STATUS"
    echo "Response: $RESPONSE_BODY"
    exit 1
fi

docker run --rm --name byoc-${CAPABILITY_NAME} --network host livepeer/ai-runner:live-app-${CAPABILITY_NAME}
echo ""
echo "The worker at '$WORKER_URL' is now running and is registered as capability '$CAPABILITY_NAME' with the orchestrator at '$ORCHESTRATOR_URL'"
echo ""
echo "To unregister this capability, you can use:"
echo "curl -X POST -H \"Authorization: $SECRET\" -d \"$CAPABILITY_NAME\" \"$ORCHESTRATOR_URL/capability/unregister\""
