#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/fe5aff1fa6a21c4899f07608ea367959085f4a5c/install_ffmpeg.sh | bash -s $1
