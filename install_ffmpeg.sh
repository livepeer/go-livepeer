#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/b33cac634b43d2ecd160224417daf8e920b0f500/install_ffmpeg.sh | bash -s $1
