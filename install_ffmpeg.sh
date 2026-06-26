#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/dca1710ae43850b2ef752815a060d359ed597f7f/install_ffmpeg.sh | bash -s $1
