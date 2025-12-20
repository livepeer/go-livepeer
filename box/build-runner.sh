#!/bin/bash
set -ex

PIPELINE=${PIPELINE:-noop}

if [[ "$PIPELINE" != "noop" && "$PIPELINE" != "comfyui" && "$PIPELINE" != "streamdiffusion" && "$PIPELINE" != "scope" && ! "$PIPELINE" =~ ^streamdiffusion- ]]; then
  echo "Error: PIPELINE must be either 'noop', 'comfyui', 'streamdiffusion', 'scope' or start with 'streamdiffusion-'"
  exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
AI_RUNNER_DIR="$SCRIPT_DIR/../../ai-runner"
cd "$AI_RUNNER_DIR/runner"

VERSION="$(bash print_version.sh)"

# comfyui uses its own external base (livepeer/comfyui-base)
if [ "${PIPELINE}" != "comfyui" ]; then
  docker build -t livepeer/ai-runner:live-base -f docker/Dockerfile.live-base .
fi


cd "$AI_RUNNER_DIR"
DOCKERFILE_PATH="live/${PIPELINE}/Dockerfile"
EXTRA_ARGS=""

if [ "${PIPELINE}" = "noop" ]; then
  # noop is built from the runner directory
  cd "$AI_RUNNER_DIR/runner"
  DOCKERFILE_PATH="docker/Dockerfile.live-app-noop"
elif [[ "${PIPELINE}" == "streamdiffusion" || "${PIPELINE}" =~ ^streamdiffusion- ]]; then
  DOCKERFILE_PATH="live/streamdiffusion/Dockerfile"

  SUBVARIANT=""
  if [[ "$PIPELINE" =~ ^streamdiffusion- ]]; then
    SUBVARIANT="${PIPELINE#streamdiffusion-}"
  fi
  EXTRA_ARGS="--build-arg SUBVARIANT=$SUBVARIANT"
fi

docker build \
  -f ${DOCKERFILE_PATH} \
  -t livepeer/ai-runner:live-app-${PIPELINE} \
  --build-arg VERSION=${VERSION} \
  $EXTRA_ARGS \
  .

docker stop live-video-to-video_${PIPELINE}_8900 || true
