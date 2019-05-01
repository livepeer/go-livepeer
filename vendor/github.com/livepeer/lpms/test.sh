#Test script to run all the tests for continuous integration

echo 'Testing core'
cd core
go test -logtostderr=true
t1=$?
cd ..

echo 'Testing vidplayer'
cd vidplayer
go test -logtostderr=true
t2=$?
cd ..

echo 'Testing vidlistener'
cd vidlistener
go test -logtostderr=true
t3=$?
cd ..

echo 'Testing Stream'
cd stream
go test -logtostderr=true
t4=$?
cd ..

echo 'Testing Transcoder'
cd transcoder
go test -logtostderr=true
t5=$?
cd ..

echo 'Testing Segmener'
cd segmenter
go test -logtostderr=true
t6=$?
cd ..

echo 'Testing FFmpeg'
cd ffmpeg
go test -logtostderr=true
t7=$?
cd ..

echo 'Testing example program'
go run cmd/transcoding/transcoding.go transcoder/test.ts P144p30fps16x9,P240p30fps16x9 sw
t8=$?

if (($t1!=0||$t2!=0||$t3!=0||$t4!=0||$t5!=0||$t6!=0||$t7!=0||$t8!=0))
then
    printf "\n\nSome Tests Failed\n\n"
    exit -1
else 
    printf "\n\nAll Tests Passed\n\n"
    exit 0
fi
