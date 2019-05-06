#!/usr/bin/env bash

#Test script to run all the tests for continuous integration

cd core
go test -logtostderr=true
t1=$?
cd ..

cd server
go test -logtostderr=true
t2=$?
cd ..

cd monitor
go test -logtostderr=true
t3=$?
cd ..

cd eth
go test -logtostderr=true
t4=$?
cd ..

cd eth/eventservices
go test -logtostderr=true
t5=$?
cd ../..

cd common
go test -logtostderr=true
t6=$?
cd ..

cd discovery
go test -logtostderr=true
t7=$?
cd ..

cd pm
go test -logtostderr=true
t8=$?
cd ..

./test_args.sh
t_args=$?

if (($t1!=0||$t2!=0||$t3!=0||$t4!=0||$t5!=0||$t6!=0||$t7!=0||$t8!=0||$t_args!=0))
then
    printf "\n\nSome Tests Failed\n\n"
    exit -1
else
    printf "\n\nAll Tests Passed\n\n"
    exit 0
fi
