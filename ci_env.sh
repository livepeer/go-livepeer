#!/bin/bash

# Script to populate some environment variables in various CI processes. Should be
# invoked by `ci_env.sh [script-name]`.

set -e

# Populate necessary Windows build path stuff
if [[ $(uname) == *"MSYS"* ]]; then
  export PATH="/usr/bin:/mingw64/bin:$PATH"
  export HOME="/build"
  export C_INCLUDE_PATH="$HOME/compiled/lib:/mingw64/lib:/mingw64/lib:${C_INCLUDE_PATH:-}"
  mkdir -p $HOME

  export PATH="$HOME/compiled/bin":$PATH
  export PKG_CONFIG_PATH="/mingw64/lib/pkgconfig:$HOME/compiled/lib/pkgconfig"
  export GOROOT=/mingw64/lib/go
  export GOPATH=/mingw64
else
  export PKG_CONFIG_PATH="$HOME/compiled/lib/pkgconfig"
fi

echo "PKG_CONFIG_PATH: ${PKG_CONFIG_PATH}"
if [ -d $PKG_CONFIG_PATH ]; then
  echo "Contents of ${PKG_CONFIG_PATH}"
  ls -1 $PKG_CONFIG_PATH
else
  echo "PKG_CONFIG_PATH: ${PKG_CONFIG_PATH} does not exist" 
fi

# If we want to build with branch --> network support for any other networks, add them here!
NETWORK_BRANCHES="dev rinkeby"

branch=""
if [[ "${TRAVIS_BRANCH:-}" != "" ]]; then
  branch="$TRAVIS_BRANCH"
elif [[ "${GHA_REF:-}" != "" ]]; then
  branch="$(echo $GHA_REF | sed 's/refs\/heads\///')"
fi

# By default we build with mainnet support
# If we are on the dev branch then we do not build with Rinkeby or mainnet support
# If we are on the rinkeby branch then we build with Rinkeby support, but not mainnet support
export HIGHEST_CHAIN_TAG=mainnet
for networkBranch in $NETWORK_BRANCHES; do
  if [[ $branch == "$networkBranch" ]]; then
    export HIGHEST_CHAIN_TAG=$networkBranch
  fi
done

# Allow non-tagged mainnet builds, but tagged releases should have mainnet support
generatedVersion=$(./print_version.sh)
definedVersion=$(cat VERSION)
if [[ $HIGHEST_CHAIN_TAG != "mainnet" ]]; then
  if [[ $generatedVersion == $definedVersion ]]; then
    echo "disallowing semver tag release $generatedVersion on branch '$branch', should be 'mainnet'"
    exit 1
  fi
fi

exec "$@"
