#!/bin/bash
./livepeer -v 99 -ethController 0x77a0865438f2efd65667362d4a8937537ca7a5ef -datadir ./.lpdev2/broadcaster_0xFE54a39F32645107841E2332afFF25976dBd3325 \
    -ethAcctAddr 0xFE54a39F32645107841E2332afFF25976dBd3325 \
    -ethUrl http://localhost:8545/ \
    -ethPassword "" \
    -network=devenv \
    -blockPollingInterval 1 \
    -monitor=false -currentManifest=true -cliAddr 127.0.0.1:7935 -httpAddr 127.0.0.1:8935  -broadcaster=true -rtmpAddr 127.0.0.1:1935