#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/1bf8bff19aa9468d428e0cfbb398afbe618bb839/install_ffmpeg.sh | bash -s $1
