#!/usr/bin/env bash

set -ex

export PATH="$HOME/compiled/bin":$PATH
export PKG_CONFIG_PATH=$HOME/compiled/lib/pkgconfig

if [ ! -e "$HOME/nv-codec-headers" ]; then
  git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git "$HOME/nv-codec-headers"
  cd $HOME/nv-codec-headers
  make -e PREFIX="$HOME/compiled"
  make install -e PREFIX="$HOME/compiled"
fi

if [ ! -e "$HOME/nasm/nasm" ]; then
  # sudo apt-get -y install asciidoc xmlto # this fails :(
  git clone -b nasm-2.14.02 https://repo.or.cz/nasm.git "$HOME/nasm"
  cd "$HOME/nasm"
  ./autogen.sh
  ./configure --prefix="$HOME/compiled"
  make
  make install || echo "Installing docs fails but should be OK otherwise"
fi

if [ ! -e "$HOME/x264/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$HOME/x264"
  cd "$HOME/x264"
  # git master as of this writing
  git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  ./configure --prefix="$HOME/compiled" --enable-pic --enable-static
  make
  make install-lib-static
fi

if [ ! -e "$HOME/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone --depth 1 -b n4.1 https://git.ffmpeg.org/ffmpeg.git "$HOME/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$HOME/ffmpeg"
  ./configure --disable-programs --disable-doc --disable-sdl2 --disable-iconv \
    --disable-muxers --disable-demuxers --disable-parsers --disable-protocols \
    --disable-encoders --disable-decoders --disable-filters --disable-bsfs \
    --disable-postproc --disable-lzma \
    --enable-gnutls --enable-libx264 --enable-gpl \
    --enable-protocol=https,rtmp,file \
    --enable-muxer=mpegts,hls,segment --enable-demuxer=flv,mpegts \
    --enable-bsf=h264_mp4toannexb,aac_adtstoasc,h264_metadata,h264_redundant_pps \
    --enable-parser=aac,aac_latm,h264 \
    --enable-filter=abuffer,buffer,abuffersink,buffersink,afifo,fifo,aformat \
    --enable-filter=aresample,asetnsamples,fps,scale \
    --enable-encoder=aac,libx264 \
    --enable-decoder=aac,h264 \
    --enable-cuda \
    --enable-cuvid \
    --enable-nvenc \
    --prefix="$HOME/compiled"
  make
  make install
fi
