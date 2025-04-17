#!/bin/bash

DOCKER=${DOCKER:-false}

if [ "$DOCKER" = "false" ]; then
    mediamtx ./box/mediamtx.yml
else
    docker run --rm --name mediamtx --network host -v $(pwd)/box/mediamtx.yml:/mediamtx.yml livepeerci/mediamtx
fi
