#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/5b7b9f5e831f041c6cf707bbaad7b5503c2f138d/install_ffmpeg.sh | bash -s $1
