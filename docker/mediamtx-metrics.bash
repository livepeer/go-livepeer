#!/bin/bash

[ -v DEBUG ] && set -x

set -euo pipefail

if [ -v LP_PUBLISH_MEDIAMTX_METRICS ]; then
  METRIC_DATA=$(
    cat <<EOF
# HELP version Current software version as a tag, always 1
# TYPE version gauge
version{app="MediaMTX",version="$MEDIAMTX_VERSION",region="$MEDIAMTX_REGION",ecosystem="$MEDIAMTX_ECOSYSTEM"} 1
EOF
  )
  echo "$METRIC_DATA"

  if [ -z "$LP_PUBLISH_MEDIAMTX_METRICS_ENDPOINT" ]; then
    echo >&2 "No endpoint specified for publishing mediamtx metrics."
    exit 1
  fi
  echo "$METRIC_DATA" | curl -X POST --data-binary @- "$LP_PUBLISH_MEDIAMTX_METRICS_ENDPOINT"
fi
