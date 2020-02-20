#!/usr/bin/env bash

set -ex

# Windows (MSYS2) needs a few tweaks
if [[ $(uname) == *"MSYS2_NT"* ]]; then
  export PATH="$PATH:/usr/bin:/mingw64/bin"
  export C_INCLUDE_PATH="${C_INCLUDE_PATH:-}:/msys64/mingw64/lib"
  export HOME="/build"
  mkdir -p $HOME

  export PATH="$HOME/compiled/bin":$PATH
  export PKG_CONFIG_PATH=/mingw64/lib/pkgconfig

  export TARGET_OS="--target-os=mingw64"
  export HOST_OS="--host=x86_64-w64-mingw32"

  # Needed for mbedtls
  export WINDOWS_BUILD=1
fi

export PATH="$HOME/compiled/bin":$PATH
export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-}:$HOME/compiled/lib/pkgconfig"

# NVENC only works on Windows/Linux
if [ $(uname) != "Darwin" ]; then
  if [ ! -e "$HOME/nv-codec-headers" ]; then
    git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git "$HOME/nv-codec-headers"
    cd $HOME/nv-codec-headers
    git checkout 9fdaf11b8f79d4e41cde9af89656238f25fec6fd
    make -e PREFIX="$HOME/compiled"
    make install -e PREFIX="$HOME/compiled"
  fi
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

if [ ! -e "$HOME/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$HOME/x264"
  cd "$HOME/x264"
  # git master as of this writing
  git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  ./configure --prefix="$HOME/compiled" --enable-pic --enable-static ${HOST_OS:-} --disable-cli
  make
  make install-lib-static
fi

# Static linking of gnutls on Linux/Mac
if [[ $(uname) != *"MSYS2_NT"* ]]; then
  # rm -rf "$HOME/gmp-6.1.2"
  if [ ! -e "$HOME/gmp-6.1.2" ]; then
    cd "$HOME"
    curl -LO https://ftp.gnu.org/gnu/gmp/gmp-6.1.2.tar.xz
    xz -d gmp-6.1.2.tar.xz
    tar xf gmp-6.1.2.tar
    rm gmp-6.1.2.tar
    cd "$HOME/gmp-6.1.2"
    ./configure --prefix="$HOME/compiled" --disable-shared  --with-pic --enable-fat
    make
    make install
  fi

  # rm -rf "$HOME/nettle-3.4.1"
  if [ ! -e "$HOME/nettle-3.4.1" ]; then
    cd $HOME
    curl -LO https://ftp.gnu.org/gnu/nettle/nettle-3.4.1.tar.gz
    tar xf nettle-3.4.1.tar.gz
    rm nettle-3.4.1.tar.gz
    cd nettle-3.4.1
    LDFLAGS="-L${HOME}/compiled/lib" CFLAGS="-I${HOME}/compiled/include" ./configure ${BUILD_OS:-} --prefix="$HOME/compiled" --disable-shared --enable-pic
    make
    make install
  fi

  # rm -rf "$HOME/gnutls-3.5.18"
  if [ ! -e "$HOME/gnutls-3.5.18" ]; then
    cd $HOME
    curl -LO https://www.gnupg.org/ftp/gcrypt/gnutls/v3.5/gnutls-3.5.18.tar.xz
    xz -d gnutls-3.5.18.tar.xz
    tar xf gnutls-3.5.18.tar
    rm gnutls-3.5.18.tar
    cd gnutls-3.5.18
    LDFLAGS="-L${HOME}/compiled/lib" CFLAGS="-I${HOME}/compiled/include" LIBS="-lhogweed -lnettle -lgmp" ./configure ${BUILD_OS:-} --prefix="$HOME/compiled" --enable-static --disable-shared --with-pic --with-included-libtasn1 --with-included-unistring --without-p11-kit --without-idn --without-zlib --disable-doc --disable-cxx --disable-tools
    make
    make install
    # gnutls doesn't properly set up its pkg-config or something? without this line ffmpeg and go
    # don't know that they need gmp, nettle, and hogweed
    sed -i'' -e 's/-lgnutls/-lgnutls -lhogweed -lnettle -lgmp/g' ~/compiled/lib/pkgconfig/gnutls.pc
  fi
fi

EXTRA_FFMPEG_FLAGS=""
EXTRA_LDFLAGS=""
# Only Linux supports CUDA... for now.
if [ $(uname) == "Linux" ]; then
  if which clang > /dev/null; then
    echo "clang detected, building with GPU support"
    EXTRA_FFMPEG_FLAGS="--enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid --enable-filter=scale_cuda --enable-encoder=h264_nvenc"
  fi
fi

if [ $(uname) == "Darwin" ]; then
  EXTRA_LDFLAGS="-framework CoreFoundation -framework Security"
fi

if [ ! -e "$HOME/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone -b livepeer-2019_11_19 https://github.com/livepeer/FFmpeg "$HOME/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$HOME/ffmpeg"
  ./configure ${TARGET_OS:-} --fatal-warnings \
    --disable-programs --disable-doc --disable-sdl2 --disable-iconv \
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
    --extra-cflags="-I${HOME}/compiled/include" \
    --extra-ldflags="-L${HOME}/compiled/lib ${EXTRA_LDFLAGS}" \
    --prefix="$HOME/compiled" \
    $EXTRA_FFMPEG_FLAGS
  make
  make install
fi
