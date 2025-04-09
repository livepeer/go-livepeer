#!/bin/bash
STREAM_KEY="my-stream"
STREAM_ID="my-stream-id"
ffmpeg -re -f lavfi -i testsrc=size=1920x1080:rate=30,format=yuv420p -vf scale=1280:720 -c:v libx264 -b:v 1000k -x264-params keyint=60 -f flv rtmp://127.0.0.1:1935/${STREAM_KEY}?pipeline=noop\&rtmpOutput=rtmp://rtmp.livepeer.com/live/d146-1bfm-uo3g-ww1v\&streamId=${STREAM_ID}
