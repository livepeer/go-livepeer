#!/bin/bash

# CI script for uploading builds.

set -e
set -o nounset

BASE_DIR="$(realpath $(dirname "$0"))"

cd "$BASE_DIR"
RELEASES_DIR="${BASE_DIR}/${RELEASES_DIR:-releases}/"

mkdir -p "$RELEASES_DIR"

if [[ "${GOOS:-}" != "" ]]; then
  PLATFORM="$GOOS"
elif [[ $(uname) == *"MSYS"* ]]; then
  PLATFORM="windows"
else
  PLATFORM=$(uname | tr '[:upper:]' '[:lower:]')
fi

EXT=""
if [[ "$PLATFORM" == "windows" ]]; then
  EXT=".exe"
fi
if [[ "$PLATFORM" != "linux" ]] && [[ "$PLATFORM" != "darwin" ]] && [[ "$PLATFORM" != "windows" ]]; then
  echo "Unknown/unsupported platform: $PLATFORM"
  exit 1
fi

if [[ -n "${RELEASE_TAG:-}" ]]; then
  PLATFORM="$PLATFORM-$RELEASE_TAG"
fi

ARCH="$(uname -m)"

if [[ "${GOARCH:-}" != "" ]]; then
  ARCH="$GOARCH"
fi
if [[ "$ARCH" == "x86_64" ]]; then
  ARCH="amd64"
fi
if [[ "$ARCH" == "aarch64" ]]; then
  ARCH="arm64"
fi
if [[ "$ARCH" != "amd64" ]] && [[ "$ARCH" != "arm64" ]]; then
  echo "Unknown/unsupported architecture: $ARCH"
  exit 1
fi

BASE="livepeer-$PLATFORM-$ARCH"
BRANCH="${TRAVIS_BRANCH:-unknown}"
if [[ "${GHA_REF:-}" != "" ]]; then
  BRANCH="$(echo $GHA_REF | sed 's:refs/heads/::')"
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

mkdir -p "${BASE_DIR}/$BASE"

# Optionally step into build directory, if set anywhere
cd "${GO_BUILD_DIR:-./}"
cp "$NODE" "$CLI" "$BENCH" "$ROUTER" "${BASE_DIR}/$BASE"
cd -

# do a basic upload so we know if stuff's working prior to doing everything else
if [[ $PLATFORM == "windows" ]]; then
  FILE="$BASE.zip"
  zip -r "${RELEASES_DIR}/$FILE" ./$BASE
else
  FILE="$BASE.tar.gz"
  tar -czvf "${RELEASES_DIR}/$FILE" ./$BASE
fi

cd "$RELEASES_DIR"

FILE_SHA256=$(shasum -a 256 ${FILE})

if [[ "${GCLOUD_KEY:-}" == "" ]]; then
  echo "GCLOUD_KEY not found, not uploading to Google Cloud."
  exit 0
fi

# https://stackoverflow.com/a/44751929/990590
BUCKET="build.livepeer.live"
PROJECT="go-livepeer"
BUCKET_PATH="${PROJECT}/${VERSION_AND_NETWORK}/${FILE}"
resource="/${BUCKET}/${BUCKET_PATH}"
contentType="application/x-compressed-tar"
dateValue="$(date -R)"
stringToSign="PUT\n\n${contentType}\n${dateValue}\n${resource}"
signature="$(echo -en ${stringToSign} | openssl sha1 -hmac ${GCLOUD_SECRET} -binary | base64)"
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

echo "upload done"

curl -X POST --fail -s \
  -H "Content-Type: application/json" \
  -d "{\"content\": \"Build succeeded ✅\nBranch: $BRANCH\nPlatform: $PLATFORM-$ARCH\nLast commit: $(git log -1 --pretty=format:'%s by %an')\nhttps://build.livepeer.live/${BUCKET_PATH}\nSHA256:\n${FILE_SHA256}\"}" \
  $DISCORD_URL
echo "done"
