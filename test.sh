#!/usr/bin/env bash

set -eux

# Test script to run all the tests except of e2e tests for continuous integration
go test -coverprofile cover.out $(go list ./... | grep -v 'test/e2e')

./test_args.sh

printf "\n\nAll Tests Passed\n\n"
