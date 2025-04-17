#!/bin/bash

DOCKER=${DOCKER:-false}

if [ "$DOCKER" = "false" ]; then
    ./livepeer -gateway -rtmpAddr :1936 -httpAddr :5936 -orchAddr localhost:8935 -v 6 -monitor
else
    docker run --rm --name gateway --network host livepeer/go-livepeer -gateway -rtmpAddr :1936 -httpAddr :5936 -orchAddr localhost:8935 -v 6 -monitor
fi
