#Test script to run all the tests for continuous integration

cd core
go test -logtostderr=true
t1=$?
cd ..

cd vidplayer
go test -logtostderr=true
t2=$?
cd ..

cd vidlistener
go test -logtostderr=true
t3=$?
cd ..

cd stream
go test -logtostderr=true
t4=$?
cd ..

cd transcoder
go test -logtostderr=true
t5=$?
cd ..

cd segmenter
go test -logtostderr=true
t6=$?
cd ..

if (($t1!=0||$t2!=0||$t3!=0||$t4!=0||$t5!=0||$t6!=0))
then
    printf "\n\nSome Tests Failed\n\n"
    exit -1
else 
    printf "\n\nAll Tests Passed\n\n"
    exit 0
fi
