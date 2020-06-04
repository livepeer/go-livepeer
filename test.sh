#!/usr/bin/env bash

set -eux

#Test script to run all the tests for continuous integration

go test ./...

cd core
# Be more strict with load balancer tests: run with race detector enabled
go test -run LB_ -race
# Be more strict with nvidia tests: run with race detector enabled
go test -run Nvidia_ -race
cd ..

# Be more strict with discovery tests: run with race detector enabled
cd discovery
go test -race
cd ..

# Be more strict with HTTP push tests: run with race detector enabled
cd server
go test -run Push_ -race
cd ..

./test_args.sh

printf "\n\nAll Tests Passed\n\n"
