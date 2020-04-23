#!/bin/bash

# Script to populate some environment variables in various CI processes. Should be
# invoked with `source ci_env.sh`.

set -e

# If we want to build with branch --> network support for any other networks, add them here!
NETWORK_BRANCHES="rinkeby mainnet"

branch=""
if [[ "${CIRCLE_BRANCH:-}" != "" ]]; then
  branch="$CIRCLE_BRANCH"
elif [[ "${TRAVIS_BRANCH:-}" != "" ]]; then
  branch="$TRAVIS_BRANCH"
fi

export HIGHEST_CHAIN_TAG=rinkeby
# for networkBranch in $NETWORK_BRANCHES; do
#   if [[ $branch == "$networkBranch" ]]; then
#     export HIGHEST_CHAIN_TAG=$networkBranch
#   fi
# done

# Disallow non-tagged mainnet releases
generatedVersion=$(./print_version.sh)
definedVersion=$(cat VERSION)
if [[ $HIGHEST_CHAIN_TAG == "mainnet" ]]; then
  if [[ $generatedVersion != $definedVersion ]]; then
    echo "disallowing mainnet release without semver tag; $generatedVersion != $definedVersion"
    exit 1
  fi
else
  if [[ $generatedVersion == $definedVersion ]]; then
    echo "disallowing semver tag release $generatedVersion on branch '$branch', should be 'mainnet'"
    exit 1
  fi
fi