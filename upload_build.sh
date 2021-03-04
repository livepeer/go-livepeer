#!/bin/bash

# CI script for uploading builds.

set -e
set -o nounset

if [[ $(uname) == *"MSYS"* ]]; then
  ARCH="windows"
  EXT=".exe"
else
  ARCH=$(uname | tr '[:upper:]' '[:lower:]')
  EXT=""
fi

BASE="livepeer-$ARCH-amd64"
BRANCH="${TRAVIS_BRANCH:-${CIRCLE_BRANCH:-unknown}}"
if [[ "${GITHUB_REF:-}" != "" ]]; then
  BRANCH="$(echo $GITHUB_REF | sed 's/refs\/heads\///')"
fi
VERSION="$(./print_version.sh)"
if echo $VERSION | grep dirty; then
  echo "Error: git state dirty, refusing to upload build"
  git diff | cat
  git status
  exit 1
fi

# If we want to build with branch --> network support for any other networks, add them here!
NETWORK_BRANCHES="rinkeby mainnet"
# If the binaries are built off a network branch then the resource path should include the network branch name i.e. X.Y.Z/rinkeby or X.Y.Z/mainnet
# If the binaries are not built off a network then the resource path should only include the version i.e. X.Y.Z
VERSION_AND_NETWORK=$VERSION
for networkBranch in $NETWORK_BRANCHES; do
  if [[ $BRANCH == "$networkBranch" ]]; then
    VERSION_AND_NETWORK="$VERSION/$BRANCH"
  fi
done

NODE="./livepeer${EXT}"
CLI="./livepeer_cli${EXT}"
BENCH="./livepeer_bench${EXT}"
ROUTER="./livepeer_router${EXT}"

mkdir $BASE
cp $NODE $BASE
cp $CLI $BASE
cp $BENCH $BASE
cp $ROUTER $BASE

# do a basic upload so we know if stuff's working prior to doing everything else
if [[ $ARCH == "windows" ]]; then
  FILE=$BASE.zip
  zip -r ./$FILE ./$BASE
else
  FILE=$BASE.tar.gz
  tar -czvf ./$FILE ./$BASE
fi

FILE_SHA256=`shasum -a 256 ${FILE}`

# Quick self-check to see if the thing can execute at all
(cd $BASE && $NODE -version)

if [[ "${GCLOUD_KEY:-}" == "" ]]; then
  echo "GCLOUD_KEY not found, not uploading to Google Cloud."
  exit 0
fi

# https://stackoverflow.com/a/44751929/990590
bucket=build.livepeer.live
resource="/${bucket}/${VERSION_AND_NETWORK}/${FILE}"
contentType="application/x-compressed-tar"
dateValue=`date -R`
stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${GCLOUD_SECRET} -binary | base64`
fullUrl="https://storage.googleapis.com${resource}"

# Failsafe - don't overwrite existing uploads!
if curl --head --fail $fullUrl 2>/dev/null; then
  echo "$fullUrl already exists, not overwriting!"
  exit 0
fi

curl -X PUT -T "${FILE}" \
  -H "Host: storage.googleapis.com" \
  -H "Date: ${dateValue}" \
  -H "Content-Type: ${contentType}" \
  -H "Authorization: AWS ${GCLOUD_KEY}:${signature}" \
  $fullUrl

curl --fail -s -H "Content-Type: application/json" -X POST -d "{\"content\": \"Build succeeded ✅\nBranch: $BRANCH\nPlatform: $ARCH-amd64\nLast commit: $(git log -1 --pretty=format:'%s by %an')\nhttps://build.livepeer.live/$VERSION_AND_NETWORK/${FILE}\nSHA256:\n${FILE_SHA256}\"}" $DISCORD_URL 2>/dev/null
echo "done"
