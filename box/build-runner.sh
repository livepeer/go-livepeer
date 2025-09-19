#!/bin/bash
set -ex

PIPELINE=${PIPELINE:-noop}

if [[ "$PIPELINE" != "noop" && "$PIPELINE" != "comfyui" && "$PIPELINE" != "streamdiffusion" && ! "$PIPELINE" =~ ^streamdiffusion- ]]; then
  echo "Error: PIPELINE must be either 'noop', 'comfyui', 'streamdiffusion' or start with 'streamdiffusion-'"
  exit 1
fi

# Switch to neighbour ai-runner directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR/../../ai-runner/runner"

VERSION="$(bash print_version.sh)"

docker build -t livepeer/ai-runner:live-base -f docker/Dockerfile.live-base .
if [ "${PIPELINE}" = "noop" ]; then
    docker build -t livepeer/ai-runner:live-app-noop -f docker/Dockerfile.live-app-noop --build-arg VERSION=${VERSION} .
else
    BASE_PIPELINE=${PIPELINE}
    INFERPY_INITIAL_PARAMS=""
    if [[ "$PIPELINE" =~ ^streamdiffusion- ]]; then
        BASE_PIPELINE="streamdiffusion"
        FILE_NAME="${PIPELINE//-/_}"
        JSON_FILE=./app/live/pipelines/${FILE_NAME}_default_params.json
        INFERPY_INITIAL_PARAMS=$(tr -d '\n' < "$JSON_FILE")
    fi
    docker build -t livepeer/ai-runner:live-base-${BASE_PIPELINE} -f docker/Dockerfile.live-base-${BASE_PIPELINE} .
    docker build \
      -f docker/Dockerfile.live-app__PIPELINE__ \
      -t livepeer/ai-runner:live-app-${PIPELINE} \
      --build-arg PIPELINE=${BASE_PIPELINE} \
      --build-arg VERSION=${VERSION} \
      --build-arg INFERPY_INITIAL_PARAMS="$INFERPY_INITIAL_PARAMS" \
      .
fi

docker stop live-video-to-video_${PIPELINE}_8900 || true
