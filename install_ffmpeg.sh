#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/bd7f8b07600446f018eab6075c22601aa3301bf6/install_ffmpeg.sh | bash -s $1
