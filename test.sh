#!/usr/bin/env bash

set -eux

# Test script to run all the tests except of e2e tests for continuous integration
go test -coverprofile cover.out $(go list ./... | grep -v 'test/e2e')

cd core
# Be more strict with load balancer tests: run with race detector enabled
go test -run LB_ -race
# Be more strict with nvidia tests: run with race detector enabled
go test -run Nvidia_ -race
go test -run Capabilities_ -race
cd ..

# Be more strict with discovery tests: run with race detector enabled
cd discovery
go test -race
cd ..

# Be more strict with HTTP push tests: run with race detector enabled
cd server
go test -run TestSelectSession_ -race
go test -run RegisterConnection -race
cd ..

./test_args.sh

printf "\n\nAll Tests Passed\n\n"
