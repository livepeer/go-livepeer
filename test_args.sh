#!/usr/bin/env bash

set -eux

# set a clean slate "home dir" for testing
TMPDIR=/tmp/livepeer-test-"$RANDOM"
DEFAULT_DATADIR="$TMPDIR"/.lpData
CUSTOM_DATADIR="$TMPDIR"/customDatadir
export HOME=$TMPDIR
rm -rf "$DEFAULT_DATADIR"
mkdir -p $TMPDIR  # goclient should make the lpData datadir

# build the binary
HIGHEST_CHAIN_TAG=mainnet
go build -tags "$HIGHEST_CHAIN_TAG" -o $TMPDIR/livepeer cmd/livepeer/*.go

# Set up ethereum key
cat > $TMPDIR/key <<ETH_KEY
{"address":"089d8ab6752bac616a1f17246294eb068ee23d3e","crypto":{"cipher":"aes-128-ctr","ciphertext":"e868446e99842291b4991ae2c8e6c6834296c81937c4182e45af2edf0af61968","cipherparams":{"iv":"62205b25b7c4b2c35717128d9702f3c8"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"dfa46628fec666ebd46d21d59cd3cbad0039d13f3c09dd5304128e32edfdec57"},"mac":"a5d4cde1863803a2b17718eb6b0770c0e7fcdfcf3659c9bd24f033d4529a5af3"},"id":"b84a7400-5fbd-4397-894e-c59403663b88","version":3}
ETH_KEY

ETH_ARGS="-ethKeystorePath $TMPDIR/key"
SLEEP=0.25


run_lp () {
    $TMPDIR/livepeer "$@" &
    pid=$!
    sleep $SLEEP
}

# sanity check that default datadir does not exist
[ ! -d "$DEFAULT_DATADIR" ]

# check that we exit early if node type is not set
res=0
$TMPDIR/livepeer || res=$?
[ $res -ne 0 ]


run_lp -broadcaster
[ -d "$DEFAULT_DATADIR"/offchain ]
kill $pid

# sanity check that custom datadir does not exist
[ ! -d "$CUSTOM_DATADIR" ]

# check custom datadir without a network (offchain)
run_lp -broadcaster -datadir "$CUSTOM_DATADIR"
[ -d "$CUSTOM_DATADIR" ]
[ ! -d  "$CUSTOM_DATADIR"/offchain ] # sanity check that network isn't included
kill $pid

CUSTOM_DATADIR="$TMPDIR"/customDatadir2

# sanity check that custom datadir does not exist
[ ! -d "$CUSTOM_DATADIR" ]

# check invalid service address via inserting control character
$TMPDIR/livepeer -orchestrator -orchSecret asdf -serviceAddr "hibye" 2>&1 | grep "Error getting service URI"
[ ${PIPESTATUS[0]} -ne 0 ]

# check missing service address via failed availability check
# Testing these isn't quite reliable enough (slow)
# XXX currently returns zero; should be nonzero
#$TMPDIR/livepeer -orchestrator -orchSecret asdf 2>&1 | grep "Orchestrator not available"
#$TMPDIR/livepeer -orchestrator -orchSecret asdf -serviceAddr livepeer.org 2>&1 | grep "Orchestrator not available"

# check that orchestrators require -orchSecret or -transcoder
# sanity check with -transcoder
run_lp -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder
kill $pid
# sanity check with -orchSecret
run_lp -orchestrator -serviceAddr 127.0.0.1:8935 -orchSecret asdf
kill $pid
# XXX need a better way of confirming the error type. specialized exit code?
res=0
$TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 || res=$?
[ $res -ne 0 ]

# Run mainnet tests
if [ -z ${MAINNET_ETH_URL+x} ]
then
    echo "MAINNET_ETH_URL is not set - skipping mainnet tests"
else
    # Exit early if -ethUrl is missing
    res=0
    $TMPDIR/livepeer -broadcaster -network mainnet $ETH_ARGS || res=$?
    [ $res -ne 0 ]

    OLD_ETH_ARGS=$ETH_ARGS
    ETH_ARGS="${ETH_ARGS} -ethUrl ${MAINNET_ETH_URL}"

    run_lp -broadcaster -network mainnet $ETH_ARGS
    [ -d "$DEFAULT_DATADIR"/mainnet ]
    kill $pid

    ETH_ARGS=$OLD_ETH_ARGS
fi

# Run Rinkeby tests
if [ -z ${RINKEBY_ETH_URL+x} ]
then
    echo "RINKEBY_ETH_URL is not set - skipping Rinkeby tests"
else
    # Exit early if -ethUrl is missing
    res=0
    $TMPDIR/livepeer -broadcaster -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]

    OLD_ETH_ARGS=$ETH_ARGS
    ETH_ARGS="${ETH_ARGS} -ethUrl ${RINKEBY_ETH_URL}"

    run_lp -broadcaster -network rinkeby $ETH_ARGS
    [ -d "$DEFAULT_DATADIR"/rinkeby ]
    kill $pid

    # Error if flags to set MaxBroadcastPrice aren't provided correctly
    res=0
    $TMPDIR/livepeer -broadcaster -network rinkeby $ETH_ARGS -maxPricePerUnit 0 -pixelsPerUnit -5 || res=$?
    [ $res -ne 0 ]

    run_lp -broadcaster -network anyNetwork $ETH_ARGS -v 99
    [ -d "$DEFAULT_DATADIR"/anyNetwork ]
    kill $pid

    # check custom datadir with a network
    run_lp -broadcaster -datadir "$CUSTOM_DATADIR" -network rinkeby $ETH_ARGS
    [ ! -d  "$CUSTOM_DATADIR"/rinkeby ] # sanity check that network isn't included
    kill $pid

    # Orchestrator needs to explicitly set PricePerUnit otherwise it will default to 0 resulting in a fatal error
    res=0
    $TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    # Orchestrator needs PricePerUnit > 0 
    res=0
    $TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder -pricePerUnit 0 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    res=0
    $TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder -pricePerUnit -5 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    # Orchestrator needs PixelsPerUnit > 0
    res=0
    $TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder -pixelsPerUnit 0 -pricePerUnit 5 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    res=0
    $TMPDIR/livepeer -orchestrator -serviceAddr 127.0.0.1:8935 -transcoder -pixelsPerUnit -5 -pricePerUnit 5 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]

    # Broadcaster needs a valid rational number for -maxTicketEV
    res=0
    $TMPDIR/livepeer -broadcaster -maxTicketEV abcd -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    # Broadcaster needs a non-negative number for -maxTicketEV
    res=0
    $TMPDIR/livepeer -broadcaster -maxTicketEV -1 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]
    # Broadcaster needs a postive number for -depositMultiplier
    res=0
    $TMPDIR/livepeer -broadcaster -depositMultiplier 0 -network rinkeby $ETH_ARGS || res=$?
    [ $res -ne 0 ]

    ETH_ARGS=$OLD_ETH_ARGS
fi

# transcoder needs -orchSecret
res=0
$TMPDIR/livepeer -transcoder || res=$?
[ $res -ne 0 ]

# exit early if webhook url is not http
res=0
$TMPDIR/livepeer -broadcaster -authWebhookUrl tcp://host/ || res=$?
[ $res -ne 0 ]

# exit early if webhook url is not properly formatted
res=0
$TMPDIR/livepeer -broadcaster -authWebhookUrl http\\://host/ || res=$?
[ $res -ne 0 ]

# exit early if orchestrator webhook URL is not http
res=0
$TMPDIR/livepeer -broadcaster -orchWebhookUrl tcp://host/ || res=$?
[ $res -ne 0 ]

# exit early if orchestrator webhook URL is not properly formatted
res=0
$TMPDIR/livepeer -broadcaster -orchWebhookUrl http\\://host/ || res=$?
[ $res -ne 0 ]

# exit early if maxSessions less or equal to zero
res=0
$TMPDIR/livepeer -broadcaster -maxSessions -1 || res=$?
[ $res -ne 0 ]

res=0
$TMPDIR/livepeer -broadcaster -maxSessions 0 || res=$?
[ $res -ne 0 ]

# Check that pprof is running on CLI port
run_lp -broadcaster
curl -sI http://127.0.0.1:7935/debug/pprof/allocs | grep "200 OK"
kill $pid

# exit early if verifier URL is not http
res=0
$TMPDIR/livepeer -broadcaster -verifierUrl tcp://host/ || res=$?
[ $res -ne 0 ]

# exit early if verifier URL is not properly formatted
res=0
$TMPDIR/livepeer -broadcaster -verifierUrl http\\://host/ || res=$?
[ $res -ne 0 ]

# Check that verifier shared path is required
$TMPDIR/livepeer -broadcaster -verifierUrl http://host 2>&1 | grep "Requires a path to the"

# Check OK with verifier shared path
run_lp -broadcaster -verifierUrl http://host -verifierPath path
kill $pid

# Check OK with verifier + external storage
run_lp -broadcaster -verifierUrl http://host -s3bucket foo/bar -s3creds baz/bat
kill $pid

# Check that HTTP ingest is disabled when -httpAddr is publicly accessible and there is no auth webhook URL and -httpIngest defaults to false
run_lp -broadcaster -httpAddr 0.0.0.0
curl -X PUT http://localhost:8935/live/movie/0.ts | grep "404 page not found"
kill $pid

# Check that HTTP ingest is disabled when -httpAddr is not publicly accessible and -httpIngest is set to false
run_lp -broadcaster -httpIngest=false
curl -X PUT http://localhost:8935/live/movie/0.ts | grep "404 page not found"
kill $pid

# Check that HTTP ingest is disabled when -httpAddr is publicly accessible and there is a auth webhook URL and -httpIngest is set to false
run_lp -broadcaster -httpAddr 0.0.0.0 -authWebhookUrl http://foo.com -httpIngest=false
curl -X PUT http://localhost:8935/live/movie/0.ts | grep "404 page not found"
kill $pid

# Check that HTTP ingest is enabled when -httpIngest is true
run_lp -broadcaster -httpAddr 0.0.0.0 -httpIngest
curl -X PUT http://localhost:8935/live/movie/0.ts | grep -v "404 page not found"
kill $pid

# Check that HTTP ingest is enabled when -httpAddr sets the hostname to 127.0.0.1
run_lp -broadcaster -httpAddr 127.0.0.1
curl -X PUT http://localhost:8935/live/movie/0.ts | grep -v "404 page not found"
kill $pid

# Check that HTTP ingest is enabled when -httpAddr sets the hostname to localhost
run_lp -broadcaster -httpAddr localhost 
curl -X PUT http://localhost:8935/live/movie/0.ts | grep -v "404 page not found"
kill $pid

# Check that HTTP ingest is enabled when there is an auth webhook URL
run_lp -broadcaster -httpAddr 0.0.0.0 -authWebhookUrl http://foo.com
curl -X PUT http://localhost:8935/live/movie/0.ts | grep -v "404 page not found"
kill $pid

rm -rf $TMPDIR
