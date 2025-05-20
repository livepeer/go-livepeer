#!/bin/bash
set -e

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

# Run processes in the background
gateway &
orchestrator &
mediamtx &

# Wait for all background processes to finish
wait

echo "All processes have completed."
