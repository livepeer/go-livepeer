set -x
set -o errexit
set -o pipefail
set -o nounset

export PATH="$PATH:/usr/bin:/mingw64/bin"
export C_INCLUDE_PATH="${C_INCLUDE_PATH:-}:/msys64/mingw64/lib"
export HOME="/build"
mkdir -p $HOME

export PATH="$HOME/compiled/bin":$PATH
export PKG_CONFIG_PATH=$HOME/compiled/lib/pkgconfig:/mingw64/lib/pkgconfig

if [ ! -e "$HOME/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone --depth 1 -b n4.1 https://git.ffmpeg.org/ffmpeg.git "$HOME/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$HOME/ffmpeg"
  ./configure --target-os=mingw64 --disable-programs --disable-doc --disable-sdl2 --disable-iconv \
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
    --prefix="$HOME/compiled"
  make
  make install
fi
