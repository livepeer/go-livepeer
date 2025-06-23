#!/bin/bash
set -e

DOCKER=${DOCKER:-false}
PIPELINE=${PIPELINE:-noop}
AI_RUNNER_CONTAINERS_PER_GPU=${AI_RUNNER_CONTAINERS_PER_GPU:-1}
VERBOSE=${VERBOSE:-0}

DOCKER_HOSTNAME="172.17.0.1"
if [[ "$(uname)" == "Darwin" ]]; then
  # Docker on macOS has a special host address
  DOCKER_HOSTNAME="host.docker.internal"
fi

NVIDIA=""
AI_MODELS_DIR=${AI_MODELS_DIR:-}
if [[ "$PIPELINE" != "noop" ]]; then
  NVIDIA="-nvidia all"
  if [[ "$AI_MODELS_DIR" = "" ]]; then
      AI_MODELS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && cd ../../ai-runner/runner/models && pwd )"
  fi
  AI_MODELS_DIR_FLAG="-aiModelsDir ${AI_MODELS_DIR}"
fi

VERBOSE_FLAG=""
if [ "$VERBOSE" = "1" ]; then
  VERBOSE_FLAG="-aiVerboseLogs"
fi

if [ "$DOCKER" = "false" ]; then
  ./livepeer \
    -orchestrator \
    -aiWorker \
    -aiModels ./box/aiModels-${PIPELINE}.json \
    -aiRunnerContainersPerGPU ${AI_RUNNER_CONTAINERS_PER_GPU} \
    ${AI_MODELS_DIR_FLAG} \
    ${VERBOSE_FLAG} \
    ${NVIDIA} \
    -serviceAddr localhost:8935 \
    -transcoder \
    -v 6 \
    -liveAITrickleHostForRunner "$DOCKER_HOSTNAME:8935" \
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
    -aiRunnerContainersPerGPU ${AI_RUNNER_CONTAINERS_PER_GPU} \
    ${VERBOSE_FLAG} \
    ${AI_MODELS_DIR_FLAG} \
    -serviceAddr 127.0.0.1:8935 \
    -transcoder \
    -v 6 \
    -liveAITrickleHostForRunner '172.17.0.1:8935' \
    -monitor
fi
