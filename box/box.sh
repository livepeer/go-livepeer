#!/bin/bash
set -e

FRONTEND=${FRONTEND:-false}

# Start multiple processes and output their logs to the console
gateway() {
  echo "Starting Gateway..."
  ./box/gateway.sh | tee gateway.log
}

orchestrator() {
  echo "Starting Orchestrator..."
  ./box/orchestrator.sh | tee orchestrator.log
}

mediamtx() {
  echo "Starting MediaMTX..."
  ./box/mediamtx.sh | tee mediamtx.log
}

supabase() {
  echo "Starting Supabase..."
  ./box/supabase.sh | tee supabase.log
}

frontend() {
  echo "Starting Frontend..."
  ./box/frontend.sh | tee supabase.log
}

# Run processes in the background
gateway &
orchestrator &
mediamtx &

if [ "$DOCKER" = "true" ]; then
  supabase &
  mediamtx &
fi

# Wait for all background processes to finish
wait

echo "All processes have completed."
