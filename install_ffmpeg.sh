#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/d462238dad5e05f0326e8d9ee9f20898f4b68e6f/install_ffmpeg.sh | bash -s $1
