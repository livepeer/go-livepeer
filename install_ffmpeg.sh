#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/258bcb383e0725d088e4cdea9f93ec7e590f0d37/install_ffmpeg.sh | bash -s $1
