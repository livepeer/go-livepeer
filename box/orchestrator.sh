#!/bin/bash

PIPELINE=${PIPELINE:-noop}

DOCKER_HOST="172.17.0.1"
if [[ "$(uname)" == "Darwin" ]]; then
  # Docker on macOS has a special host address
  DOCKER_HOST="host.docker.internal"
fi

./livepeer \
  -orchestrator \
  -aiWorker \
  -aiModels ./box/aiModels-${PIPELINE}.json \
  -serviceAddr localhost:8935 \
  -transcoder \
  -v 6 \
  -liveAITrickleHostForRunner "$DOCKER_HOST:8935" \
  -monitor
