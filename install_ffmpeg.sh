#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/ffde2327537517b3345162e9544704571bc58a34/install_ffmpeg.sh | bash -s $1
