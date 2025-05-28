#!/bin/bash

DOCKER=${DOCKER:-false}

if [ "$DOCKER" = "false" ]; then
    LIVE_AI_WHIP_ADDR=":7280" LIVE_AI_ALLOW_CORS="1" ./livepeer -gateway -rtmpAddr :1936 -httpAddr :5936 -orchAddr localhost:8935 -v 6 -monitor
else
    docker run -e LIVE_AI_WHIP_ADDR=":7280" -e LIVE_AI_ALLOW_CORS="1" --rm --name gateway --network host livepeer/go-livepeer -gateway -rtmpAddr :1936 -httpAddr :5936 -orchAddr localhost:8935 -v 6 -monitor
fi
