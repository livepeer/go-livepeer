#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/d9c78b62effdb4f5c8fc438b6033da1090d04a03/install_ffmpeg.sh | bash -s $1
