#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/9696b33800795e59ee041063caee91c44590186a/install_ffmpeg.sh | bash -s $1
