#!/bin/bash

set -e

NUM=${NUM:-2}
PIPELINE=${PIPELINE:-noop}

pids=()

# Function to kill all remaining processes if one exits
cleanup() {
  echo "One of the processes failed. Stopping all log streams..."
  for pid in "${pids[@]}"; do
    kill $pid 2>/dev/null || true
  done
  exit 1
}

# Start log streaming for each container
for i in $(seq 0 $((NUM-1))); do
  PORT="89${i}0"
  CONTAINER_NAME="live-video-to-video_${PIPELINE}_${PORT}"

  # Stream logs with a prefix showing which instance (i) and port
  docker logs --follow=true "${CONTAINER_NAME}" 2>&1 | grep --line-buffered 'Output FPS' | sed "s/^/[RUNNER-${i}:${PORT}] /" &
  pids+=($!)

  echo "Started log streaming for container ${CONTAINER_NAME}"
done

# Wait for any process to exit, then clean up
wait -n
cleanup
