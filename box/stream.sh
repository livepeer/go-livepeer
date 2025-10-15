#!/bin/bash
set -ex

PIPELINE=${PIPELINE:-noop}
STREAM_KEY="my-stream"
STREAM_ID="my-stream-id"
RTMP_OUTPUT=${RTMP_OUTPUT:-""}
FPS=${FPS:-30}
INPUT_VIDEO=${INPUT_VIDEO:-""}

case "$1" in
  start)
    QUERY="pipeline=${PIPELINE}\&streamId=${STREAM_ID}"
    if [ -n "$RTMP_OUTPUT" ]; then
      QUERY="${QUERY}\&rtmpOutput=${RTMP_OUTPUT}"
    fi

    if [ -n "$PARAMS" ]; then
      ENCODED_PARAMS=$(node -p "encodeURIComponent('$PARAMS')")
      QUERY="${QUERY}\&params=${ENCODED_PARAMS}"
    fi

    INPUT_FLAGS="-f lavfi -i testsrc=size=1280x720:rate=${FPS},format=yuv420p"
    if [ -n "$INPUT_VIDEO" ]; then
      INPUT_FLAGS="-stream_loop -1 -i ${INPUT_VIDEO}"
    fi

    ffmpeg -re ${INPUT_FLAGS} \
      -vf "scale=1280:720:force_original_aspect_ratio=increase,crop=1280:720" \
      -c:v libx264 \
      -b:v 1000k \
      -x264-params keyint=$((FPS * 2)) \
      -f flv rtmp://127.0.0.1:1935/${STREAM_KEY}?${QUERY}
    ;;
  playback)
    ffplay rtmp://127.0.0.1:1935/${STREAM_KEY}-out
    ;;
  *)
    echo "Usage: $0 {start|playback}"
    exit 1
    ;;
esac
