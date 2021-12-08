#!/usr/bin/env bash

set -ex

ROOT="${1:-$HOME}"
ARCH="$(uname -m)"

if [[ $ARCH == "arm64" ]] && [[ $(uname) == "Darwin" ]]; then
  # Detect Apple Silicon
  IS_M1=1
fi
echo "Arch $ARCH ${IS_M1:+(Apple Silicon)}"

# Windows (MSYS2) needs a few tweaks
if [[ $(uname) == *"MSYS"* ]]; then
  ROOT="/build"
  export PATH="$PATH:/usr/bin:/mingw64/bin"
  export C_INCLUDE_PATH="${C_INCLUDE_PATH:-}:/mingw64/lib"

  export PATH="$ROOT/compiled/bin":$PATH
  export PKG_CONFIG_PATH=/mingw64/lib/pkgconfig

  export TARGET_OS="--target-os=mingw64"
  export HOST_OS="--host=x86_64-w64-mingw32"
  export BUILD_OS="--build=x86_64-w64-mingw32 --host=x86_64-w64-mingw32 --target=x86_64-w64-mingw32"

  # Needed for mbedtls
  export WINDOWS_BUILD=1
fi

export PATH="$ROOT/compiled/bin":$PATH
export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-}:$ROOT/compiled/lib/pkgconfig"

mkdir -p "$ROOT/"

# NVENC only works on Windows/Linux
if [ $(uname) != "Darwin" ]; then
  if [ ! -e "$ROOT/nv-codec-headers" ]; then
    git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git "$ROOT/nv-codec-headers"
    cd $ROOT/nv-codec-headers
    git checkout 250292dd20af60edc6e0d07f1d6e489a2f8e1c44
    make -e PREFIX="$ROOT/compiled"
    make install -e PREFIX="$ROOT/compiled"
  fi
fi

if [[ $(uname) != *"MSYS"* ]] && [[ ! $IS_M1 ]]; then
  if [ ! -e "$ROOT/nasm-2.14.02" ]; then
    # sudo apt-get -y install asciidoc xmlto # this fails :(
    cd "$ROOT"
    curl -o nasm-2.14.02.tar.gz https://www.nasm.us/pub/nasm/releasebuilds/2.14.02/nasm-2.14.02.tar.gz
    echo 'b34bae344a3f2ed93b2ca7bf25f1ed3fb12da89eeda6096e3551fd66adeae9fc  nasm-2.14.02.tar.gz' > nasm-2.14.02.tar.gz.sha256
    sha256sum -c nasm-2.14.02.tar.gz.sha256
    tar xf nasm-2.14.02.tar.gz
    rm nasm-2.14.02.tar.gz nasm-2.14.02.tar.gz.sha256
    cd "$ROOT/nasm-2.14.02"
    ./configure --prefix="$ROOT/compiled"
    make
    make install || echo "Installing docs fails but should be OK otherwise"
  fi
fi

if [ ! -e "$ROOT/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$ROOT/x264"
  cd "$ROOT/x264"
  if [[ ! $IS_M1 ]]; then
    # newer git master, compiles on Apple Silicon
    git checkout 66a5bc1bd1563d8227d5d18440b525a09bcf17ca
  else
    # git master as of this writing
    git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  fi
  ./configure --prefix="$ROOT/compiled" --enable-pic --enable-static ${HOST_OS:-} --disable-cli
  make
  make install-lib-static
fi

EXTRA_FFMPEG_FLAGS=""
EXTRA_LDFLAGS=""

if [ $(uname) == "Darwin" ]; then
  EXTRA_LDFLAGS="-framework CoreFoundation -framework Security"
else
  # If we have clang, we can compile with CUDA support!
  if which clang > /dev/null; then
    echo "clang detected, building with GPU support"

    EXTRA_FFMPEG_FLAGS="--enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid --enable-filter=scale_cuda,signature_cuda --enable-encoder=h264_nvenc"

    if [[ $BUILD_TAGS == *"experimental"* ]]; then
        echo "experimental tag detected, building with Tensorflow support"
        EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-libtensorflow"
    fi
  fi
fi

if [ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone https://github.com/livepeer/FFmpeg.git "$ROOT/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$ROOT/ffmpeg"
  git checkout 682c4189d8364867bcc49f9749e04b27dc37cded
  ./configure ${TARGET_OS:-} --fatal-warnings \
    --disable-programs --disable-doc --disable-sdl2 --disable-iconv \
    --disable-muxers --disable-demuxers --disable-parsers --disable-protocols \
    --disable-encoders --disable-decoders --disable-filters --disable-bsfs \
    --disable-postproc --disable-lzma \
    --enable-libx264 --enable-gpl \
    --enable-protocol=rtmp,file,pipe \
    --enable-muxer=mpegts,hls,segment,mp4,null --enable-demuxer=flv,mpegts,mp4,mov \
    --enable-bsf=h264_mp4toannexb,aac_adtstoasc,h264_metadata,h264_redundant_pps,extract_extradata \
    --enable-parser=aac,aac_latm,h264 \
    --enable-filter=abuffer,buffer,abuffersink,buffersink,afifo,fifo,aformat,format \
    --enable-filter=aresample,asetnsamples,fps,scale,hwdownload,select,livepeer_dnn,signature \
    --enable-encoder=aac,libx264 \
    --enable-decoder=aac,h264 \
    --extra-cflags="-I${ROOT}/compiled/include" \
    --extra-ldflags="-L${ROOT}/compiled/lib ${EXTRA_LDFLAGS}" \
    --prefix="$ROOT/compiled" \
    $EXTRA_FFMPEG_FLAGS
  make
  make install
fi
