#!/usr/bin/env bash

docker build -t go-livepeer-test . && docker run --rm --name go-livepeer-test-run go-livepeer-test
