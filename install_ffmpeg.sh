#!/usr/bin/env bash
echo 'WARNING: downloading and executing lpms/install_ffmpeg.sh, use it directly in case of issues'
curl https://raw.githubusercontent.com/livepeer/lpms/e1872bf609de6befe3cfcdc7a464d1e3469ea843/install_ffmpeg.sh | bash -s $1
