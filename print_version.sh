#!/bin/bash

# Prints the current build version of go-livepeer. Based on the following considerations:
# 1. If the VERSION file matches the tag of our current commit, then we're the canonical
#    x.y.z version, and that will be our version string.
# 2. Otherwise, our version string is x.y.z-SHA, provided by the VERSION file and
#    git describe --always --long --abbrev=8 --dirty

set -e
set -o nounset

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

currentTag="$(git describe --tags)"
currentVersion="$(cat "$DIR/VERSION")"
currentSha="$(git describe --always --long --dirty --abbrev=8)"

if [[ "$currentTag" == "v$currentVersion" ]]; then
  echo -en "$currentVersion"
else
  echo -en "$currentVersion-$currentSha"
fi
