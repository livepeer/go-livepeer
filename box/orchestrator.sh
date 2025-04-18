#!/bin/bash

DOCKER=${DOCKER:-false}
PIPELINE=${PIPELINE:-noop}

DOCKER_HOST="172.17.0.1"
if [[ "$(uname)" == "Darwin" ]]; then
  # Docker on macOS has a special host address
  DOCKER_HOST="host.docker.internal"
fi

NVIDIA=""
AI_MODELS_DIR=${AI_MODELS_DIR:-}
if [[ "$PIPELINE" != "noop" ]]; then
  NVIDIA="-nvidia all"
  if [[ "$AI_MODELS_DIR" = "" ]]; then
      AI_MODELS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && cd ../ai-runner/runner/models && pwd )"
  fi
  AI_MODELS_DIR_FLAG="-aiModelsDir ${AI_MODELS_DIR}"
fi

if [ "$DOCKER" = "false" ]; then
  ./livepeer \
    -orchestrator \
    -aiWorker \
    -aiModels ./box/aiModels-${PIPELINE}.json \
    ${AI_MODELS_DIR_FLAG} \
    ${NVIDIA} \
    -serviceAddr localhost:8935 \
    -transcoder \
    -v 6 \
    -liveAITrickleHostForRunner "$DOCKER_HOST:8935" \
    -monitor
else
  docker run --rm --name orchestrator \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v ./box/aiModels-${PIPELINE}.json:/opt/aiModels.json \
  livepeer/go-livepeer \
    -orchestrator \
    -aiWorker \
    -aiModels /opt/aiModels.json \
    ${AI_MODELS_DIR_FLAG} \
    -serviceAddr 127.0.0.1:8935 \
    -transcoder \
    -v 6 \
    -liveAITrickleHostForRunner '172.17.0.1:8935' \
    -monitor
fi
