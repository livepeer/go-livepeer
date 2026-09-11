#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/467824663359a0a48f04e8563e99e1bec1561455/install_ffmpeg.sh | bash -s $1
