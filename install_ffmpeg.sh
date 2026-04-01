#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/38b00f0947bfd05ac39e594d94ea38950ea82cd2/install_ffmpeg.sh | bash -s $1
