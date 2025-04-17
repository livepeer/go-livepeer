#! /bin/bash

set -e

NUM=${NUM:-2}

declare -A rtmp_outputs
rtmp_outputs[1]="rtmp://rtmp.livepeer.com/live/0f5f-zq4w-ef5n-wg5m"
rtmp_outputs[2]="rtmp://rtmp.livepeer.com/live/0974-7x87-ha7l-i2yv"


pids=()

# Function to kill all remaining processes if one exits
cleanup() {
  echo "One of the streams failed. Stopping all streams..."
  for pid in "${pids[@]}"; do
    kill $pid 2>/dev/null || true
  done
  exit 1
}

# Start each stream
for i in $(seq 1 $NUM); do
  IDX=$i RTMP_OUTPUT="${rtmp_outputs[$i]}" ./box/stream.sh start &
  pids+=($!)
done

wait -n
cleanup
