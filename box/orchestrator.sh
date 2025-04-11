#!/bin/bash
./livepeer -orchestrator -aiWorker -aiModels ./box/aiModels.json -serviceAddr localhost:8935 -transcoder -v 6 -liveAITrickleHostForRunner 'host.docker.internal:8935' -monitor
