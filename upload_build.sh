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
VERSION="$(./print_version.sh)"
if echo $VERSION | grep dirty; then
  echo "Error: git state dirty, refusing to upload build"
  git diff | cat
  git status
  exit 1
fi

mkdir $BASE
cp ./livepeer${EXT} $BASE
cp ./livepeer_cli${EXT} $BASE

# do a basic upload so we know if stuff's working prior to doing everything else
if [[ $ARCH == "windows" ]]; then
  FILE=$BASE.zip
  ls /mingw64/bin/
  # This list was produced by `ldd livepeer.exe`
  LIBS="libffi-6.dll libgcc_s_seh-1.dll libgmp-10.dll libgnutls-30.dll libhogweed-5.dll libiconv-2.dll libidn2-0.dll libintl-8.dll libnettle-7.dll libp11-kit-0.dll libtasn1-6.dll libunistring-2.dll libwinpthread-1.dll zlib1.dll"
  for LIB in $LIBS; do
    cp -r /mingw64/bin/$LIB ./$BASE || echo "$LIB not found"
  done
  zip -r ./$FILE ./$BASE
else
  FILE=$BASE.tar.gz
  tar -czvf ./$FILE ./$BASE
fi

if [[ "${GCLOUD_KEY:-}" == "" ]]; then
  echo "GCLOUD_KEY not found, not uploading to Google Cloud."
  exit 0
fi

# https://stackoverflow.com/a/44751929/990590
bucket=build.livepeer.live
resource="/${bucket}/${VERSION}/${FILE}"
contentType="application/x-compressed-tar"
dateValue=`date -R`
stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
signature=`echo -en ${stringToSign} | openssl sha1 -hmac ${GCLOUD_SECRET} -binary | base64`
curl -X PUT -T "${FILE}" \
  -H "Host: storage.googleapis.com" \
  -H "Date: ${dateValue}" \
  -H "Content-Type: ${contentType}" \
  -H "Authorization: AWS ${GCLOUD_KEY}:${signature}" \
  https://storage.googleapis.com${resource}

curl --fail -s -H "Content-Type: application/json" -X POST -d "{\"content\": \"Build succeeded ✅\nBranch: $BRANCH\nPlatform: $ARCH-amd64\nLast commit: $(git log -1 --pretty=format:'%s by %an')\nhttps://build.livepeer.live/$VERSION/${FILE}\"}" $DISCORD_URL 2>/dev/null
echo "done"
