#!/usr/bin/env bash

#Test script to run all the tests for continuous integration

go test ./... -v
t1=$?

cd core
# Be more strict with load balancer tests: run with race detector enabled
go test -logtostderr=true -run LB_ -race
t2_lb=$?
# Be more strict with nvidia tests: run with race detector enabled
go test -logtostderr=true -run Nvidia_ -race
t2_nv=$?
cd ..

./test_args.sh
t_args=$?

if (($t1!=0||$t2_lb!=0||$t2_nv!=0||$t_args!=0))
then
    printf "\n\nSome Tests Failed\n\n"
    exit -1
else
    printf "\n\nAll Tests Passed\n\n"
    exit 0
fi
