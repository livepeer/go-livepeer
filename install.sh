#!/usr/bin/env bash

set -e

PLATFORM="$([ $(uname | grep -i "Darwin") ] && echo 'darwin' || echo 'linux')"
VERSION="${1:-latest}"
GPG_VERIFICATION_KEY="A2F9039A8603C44C21414432A2224D4537874DB2"
LP_TMP_DIR="$(mktemp -d)"
GPG_VERIFY="${GPG_VERIFY:-true}"
SHA_VERIFY="${SHA_VERIFY:-true}"

if ! command -v curl >/dev/null; then
  echo >&2 "Could not find 'curl'. Please install and run the script again."
  exit 1
fi

if ! command -v gpg >/dev/null; then
  echo >&2 "Could not find 'gpg'. Signature verification will be skipped."
  GPG_VERIFY="false"
fi

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl https://api.github.com/repos/livepeer/go-livepeer/releases/latest 2>/dev/null | grep -F '"name": "v' | cut -d '"' -f4)"
else
  VERSION="v${VERSION}"
fi

get_system_architecture() {
  local arch="$(uname -m)"
  case "$arch" in
  aarch64 | aarch64_be | arm*)
    arch="arm64"
    ;;
  x86_64)
    arch="amd64"
    ;;
  esac
  printf "%s" "$arch"
}

get_archive_name() {
  printf "livepeer-%s-%s.tar.gz" "$PLATFORM" "$(get_system_architecture)"
}

get_download_url() {
  printf "https://github.com/livepeer/go-livepeer/releases/download/${VERSION}"
}

download_files() {
  curl -sLO "$(get_download_url)/$(get_archive_name)" 2>/dev/null
  curl -sLO "$(get_download_url)/${VERSION}_checksums.txt" 2>/dev/null
  if [[ "$GPG_VERIFY" == "true" ]]; then
    curl -sLO "$(get_download_url)/$(get_archive_name).sig" 2>/dev/null
  fi
}

verify_download() {
  if [[ "$SHA_VERIFY" == "true" ]]; then
    sha256sum --ignore-missing -c "${VERSION}_checksums.txt"
  fi
  if [[ "$GPG_VERIFY" == "true" ]]; then
    gpg --keyserver keyserver.ubuntu.com --recv-keys "${GPG_VERIFICATION_KEY}"
    gpg --verify "$(get_archive_name).sig" "$(get_archive_name)"
  fi
}

install_to_path() {
  local archive_name="$(get_archive_name)"
  tar xzf "${archive_name}"
  mv "${archive_name%%.*}/*" /usr/local/bin/
}

cd "$LP_TMP_DIR"
download_files
verify_download
install_to_path
cd -
rm -rf "$LP_TMP_DIR"
