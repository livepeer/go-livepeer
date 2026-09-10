#!/bin/bash
# Smoke test the live runner path with the published example apps from
# https://github.com/livepeer/runner-app-examples.
set -e

DOCKER=${DOCKER:-false}
APP=${APP:-hello-world}
ORCH_SECRET=${ORCH_SECRET:-abcdef}
ORCH_URL=${ORCH_URL:-https://127.0.0.1:8935}
EXAMPLES_REF=${EXAMPLES_REF:-main}
FFMPEG=${FFMPEG:-ffmpeg}
FFPLAY=${FFPLAY:-ffplay}
CACHE_DIR="$(dirname "${BASH_SOURCE[0]}")/.cache"

ORCH_ARGS=(
  -orchestrator
  -useLiveRunners
  -orchSecret "$ORCH_SECRET"
  -serviceAddr 127.0.0.1:8935
  -liveRunnerAddr "$ORCH_URL"
  -cliAddr 127.0.0.1:7935
  -network offchain
  -monitor=false
  -v 6
)

discover() {
  curl -sk "$ORCH_URL/discovery"
}

wait_for_runner() {
  for _ in $(seq 1 30); do
    URL=$(discover | jq -r '.[0].runners[0].url // empty')
    [ -n "$URL" ] && return 0
    sleep 1
  done
  echo "No live runner registered at $ORCH_URL/discovery" >&2
  exit 1
}

case "$1" in
  orchestrator)
    if [ "$DOCKER" = "false" ]; then
      ./livepeer "${ORCH_ARGS[@]}"
    else
      docker run --rm --name live-runner-orchestrator --network host livepeer/go-livepeer "${ORCH_ARGS[@]}"
    fi
    ;;
  app)
    docker run --rm --name "live-runner-$APP" --network host \
      "ghcr.io/livepeer/runner-example-$APP:latest" \
      --host=0.0.0.0 --orchestrator="$ORCH_URL" --orchSecret="$ORCH_SECRET" \
      --runner-url=http://127.0.0.1:8989
    ;;
  call)
    wait_for_runner
    discover | jq '.[].runners[] | {app, mode, url}'
    curl -sk -X POST "$URL/hello" -H 'Content-Type: application/json' -d '{"name":"box"}'
    echo
    ;;
  echo)
    wait_for_runner
    mkdir -p "$CACHE_DIR"
    if [ ! -f "$CACHE_DIR/echo-client.py" ]; then
      curl -sfo "$CACHE_DIR/echo-client.py" \
        "https://raw.githubusercontent.com/livepeer/runner-app-examples/$EXAMPLES_REF/echo/client.py"
    fi
    "$FFMPEG" -re -f lavfi -i testsrc=size=1280x720:rate=30 \
      -c:v libx264 -tune zerolatency -preset ultrafast -pix_fmt yuv420p -f mpegts - \
      | uv run --with av --with aiohttp --with 'livepeer-gateway>=1.0.0' \
          "$CACHE_DIR/echo-client.py" - --mode blur --discovery "$ORCH_URL/discovery" --output - \
      | "$FFPLAY" -fflags nobuffer -flags low_delay -framedrop -i -
    ;;
  *)
    echo "Usage: $0 {orchestrator|app|call|echo}"
    exit 1
    ;;
esac
