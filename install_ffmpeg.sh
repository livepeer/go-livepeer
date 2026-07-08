#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/bc67cafe732afb08053328483f73cf000a28d330/install_ffmpeg.sh | bash -s $1
