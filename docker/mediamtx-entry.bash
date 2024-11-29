#!/bin/bash

[ -v DEBUG ] && set -x

set -euo pipefail

if [ -v LP_PUBLISH_MEDIAMTX_METRICS ]; then
  if [ -z "$LP_PUBLISH_MEDIAMTX_METRICS_ENDPOINT" ]; then
    echo >&2 "No endpoint specified for publishing mediamtx metrics."
  fi
  echo <<EOF |
# HELP version Current software version as a tag, always 1
# TYPE version gauge
version{app="MediaMTX",version="$MEDIAMTX_VERSION"} 1
EOF
    curl -X POST --data-binary @- "$LP_PUBLISH_MEDIAMTX_METRICS_ENDPOINT"
fi

/usr/local/bin/mediamtx
