#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/5fe66b1044f876b47248c36aed581622fbbe093d/install_ffmpeg.sh | bash -s $1
