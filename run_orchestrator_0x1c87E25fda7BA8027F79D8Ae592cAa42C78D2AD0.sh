#!/bin/bash
./livepeer -v 3 -ethController 0x77a0865438f2efd65667362d4a8937537ca7a5ef -datadir ./.lpdev2/orchestrator_0x1c87E25fda7BA8027F79D8Ae592cAa42C78D2AD0 \
    -ethAcctAddr 0x1c87E25fda7BA8027F79D8Ae592cAa42C78D2AD0 \
    -ethUrl http://localhost:8545/ \
    -ethPassword "" \
    -network=devenv \
    -blockPollingInterval 1 \
    -monitor=false -currentManifest=true -cliAddr 127.0.0.1:7936 -httpAddr 127.0.0.1:8936  -initializeRound=true \
    -serviceAddr 127.0.0.1:8936  -transcoder=true -orchestrator=true \
    -orchSecret secre -pricePerUnit 5000 -redeemer=true -gasPrice 200
    