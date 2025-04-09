#!/bin/bash
./livepeer -orchestrator -aiWorker -aiModels 'live-video-to-video:noop:true' -serviceAddr localhost:8935 -transcoder -v 6 -liveAITrickleHostForRunner 'host.docker.internal:8935' -monitor
