#!/bin/bash

# CI script for uploading builds.

set -e
set -o nounset

export CLOUDSDK_CORE_DISABLE_PROMPTS=1
ARCH=$(uname | tr '[:upper:]' '[:lower:]')
BASE="livepeer-$ARCH-amd64"
BRANCH="${TRAVIS_BRANCH:-${CIRCLE_BRANCH:-unknown}}"
VERSION="$(cat VERSION)-$(git describe --always --long --dirty)"

if [ ! -d $HOME/google-cloud-sdk/bin ]; then
  # The install script errors if this directory already exists,
  # but Travis already creates it when we mark it as cached.
  rm -rf $HOME/google-cloud-sdk;
  # The install script is overly verbose, which sometimes causes
  # problems on Travis, so ignore stdout.
  curl https://sdk.cloud.google.com | bash > /dev/null;
fi
source $HOME/google-cloud-sdk/path.bash.inc
gcloud version
echo $GCLOUD_TOKEN | base64 --decode > /tmp/token.json

gcloud auth activate-service-account --key-file=/tmp/token.json
# do a basic upload so we know if stuff's working prior to doing everything else
gsutil cp README.md gs://build.livepeer.live/README.md
mkdir $BASE
mv ./livepeer $BASE
mv ./livepeer_cli $BASE
tar -czvf ./$BASE.tar.gz ./$BASE
gsutil cp ./$BASE.tar.gz gs://build.livepeer.live/$VERSION/$BASE.tar.gz
curl --fail -H "Content-Type: application/json" -X POST -d "{\"content\": \"Build succeeded ✅\nBranch: $BRANCH\nPlatform: darwin-amd64\nhttps://build.livepeer.live/$VERSION/$BASE.tar.gz\"}" $DISCORD_URL
