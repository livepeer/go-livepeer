#!/usr/bin/env bash

set -eux

go test $(go list ./... | grep 'test/e2e') --timeout 15m
