#!/bin/bash
set -ex

PIPELINE=${PIPELINE:-noop}

if [[ "$PIPELINE" != "noop" && "$PIPELINE" != "comfyui" && "$PIPELINE" != "streamdiffusion" && "$PIPELINE" != "scope" && ! "$PIPELINE" =~ ^streamdiffusion- ]]; then
  echo "Error: PIPELINE must be either 'noop', 'comfyui', 'streamdiffusion', 'scope' or start with 'streamdiffusion-'"
  exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
AI_RUNNER_DIR="$SCRIPT_DIR/../../ai-runner"


# Build base image first (needed for all pipelines except comfyui)
if [ "${PIPELINE}" != "comfyui" ]; then
  cd "$AI_RUNNER_DIR/runner"
  docker build -t livepeer/ai-runner:live-base -f docker/Dockerfile.live-base .
fi

# Handle scope pipeline separately - it has its own build script
if [ "${PIPELINE}" = "scope" ]; then
  SCOPE_RUNNER_DIR="${SCOPE_RUNNER_DIR:-$SCRIPT_DIR/../../scope-runner}"
  if [ ! -d "$SCOPE_RUNNER_DIR" ]; then
    echo "Error: scope-runner directory not found at $SCOPE_RUNNER_DIR" >&2
    echo "Expected scope-runner to be a sibling directory of go-livepeer" >&2
    echo "Or set SCOPE_RUNNER_DIR environment variable to override the path" >&2
    exit 1
  fi
  USE_LATEST_BASE=1 bash $SCOPE_RUNNER_DIR/build-docker.sh
  docker stop live-video-to-video_${PIPELINE}_8900 || true
  exit 0
fi

VERSION="$(bash print_version.sh)"

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
