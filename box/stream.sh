#!/bin/bash

STREAM_KEY="my-stream"
STREAM_ID="my-stream-id"

case "$1" in
  start)
    ffmpeg -re -f lavfi -i testsrc=size=1920x1080:rate=30,format=yuv420p -vf scale=1280:720 -c:v libx264 -b:v 1000k -x264-params keyint=60 -f flv rtmp://127.0.0.1:1935/${STREAM_KEY}?pipeline=noop\&streamId=${STREAM_ID}
    ;;
  playback)
    ffplay rtmp://127.0.0.1:1935/${STREAM_KEY}-out
    ;;
  *)
    echo "Usage: $0 {start|playback}"
    exit 1
    ;;
esac
