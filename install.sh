#!/usr/bin/env bash

os=$([ $(uname | grep 'Darwin') ] && echo 'darwin' || echo 'linux');
curl -s https://api.github.com/repos/livepeer/go-livepeer/releases/latest \
  | grep browser_download_url \
  | grep "${os}.tar" \
  | cut -d '"' -f 4 \
  | xargs -n 1 curl -L \
  | tar xz \
  && cp -R "livepeer_${os}/." /usr/local/bin \
  && rm -rf "livepeer_${os}";
