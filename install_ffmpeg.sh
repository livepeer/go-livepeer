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
    make -j$(nproc)
    make -j$(nproc) install || echo "Installing docs fails but should be OK otherwise"
  fi
fi

if [ ! -e "$ROOT/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$ROOT/x264"
  cd "$ROOT/x264"
  if [[ $IS_M1 ]]; then
    # newer git master, compiles on Apple Silicon
    git checkout 66a5bc1bd1563d8227d5d18440b525a09bcf17ca
  else
    # older git master, does not compile on Apple Silicon
    git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  fi
  ./configure --prefix="$ROOT/compiled" --enable-pic --enable-static ${HOST_OS:-} --disable-cli
  make -j$(nproc)
  make -j$(nproc) install-lib-static
fi

if [[ $(uname) == "Linux" && $BUILD_TAGS == *"debug-video"* ]]; then
  sudo apt-get install -y libnuma-dev cmake
  if [ ! -e "$ROOT/x265" ]; then
    git clone https://bitbucket.org/multicoreware/x265_git.git "$ROOT/x265"
    cd "$ROOT/x265"
    git checkout 17839cc0dc5a389e27810944ae2128a65ac39318
    cd build/linux/
    cmake -DCMAKE_INSTALL_PREFIX=$ROOT/compiled -G "Unix Makefiles" ../../source
    make -j$(nproc)
    make -j$(nproc) install
  fi
  # VP8/9 support
  if [ ! -e "$ROOT/libvpx" ]; then
    git clone https://chromium.googlesource.com/webm/libvpx.git "$ROOT/libvpx"
    cd "$ROOT/libvpx"
    git checkout ab35ee100a38347433af24df05a5e1578172a2ae
    ./configure --prefix="$ROOT/compiled" --disable-examples --disable-unit-tests --enable-vp9-highbitdepth --enable-shared --as=nasm
    make -j$(nproc)
    make -j$(nproc) install
  fi
fi

EXTRA_FFMPEG_FLAGS=""
DISABLE_FFMPEG_COMPONENTS=""
EXTRA_LDFLAGS=""
# all flags which should be present for production build, but should be replaced/removed for debug build
DEV_FFMPEG_FLAGS="--disable-programs"
FFMPEG_MAKE_EXTRA_ARGS=""

if [ $(uname) == "Darwin" ]; then
  EXTRA_LDFLAGS="-framework CoreFoundation -framework Security"
else
  # If we have clang, we can compile with CUDA support!
  if which clang > /dev/null; then
    echo "clang detected, building with GPU support"
    EXTRA_FFMPEG_FLAGS="--enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid,hevc_cuvid,vp8_cuvid,vp9_cuvid --enable-filter=scale_cuda,signature_cuda,hwupload_cuda --enable-encoder=h264_nvenc,hevc_nvenc"
    if [[ $BUILD_TAGS == *"experimental"* ]]; then
        if [ ! -e "/usr/local/lib/libtensorflow_framework.so" ]; then
          LIBTENSORFLOW_VERSION=2.3.0 \
          && curl -LO https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz \
          && sudo tar -C /usr/local -xzf libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz \
          && rm libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz
        fi
        echo "experimental tag detected, building with Tensorflow support"
        EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-libtensorflow"
    fi
  fi
fi

if [[ $BUILD_TAGS == *"debug-video"* ]]; then
    echo "video debug mode, building ffmpeg with tools, debug info and additional capabilities for running tests"
    DEV_FFMPEG_FLAGS="--enable-muxer=md5,flv --enable-demuxer=hls --enable-filter=ssim,tinterlace --enable-encoder=wrapped_avframe,pcm_s16le "
    DEV_FFMPEG_FLAGS+="--enable-shared --enable-debug=3 --disable-stripping --disable-optimizations --enable-encoder=libx265,libvpx_vp8,libvpx_vp9 "
    DEV_FFMPEG_FLAGS+="--enable-decoder=hevc,libvpx_vp8,libvpx_vp9 --enable-libx265 --enable-libvpx --enable-bsf=noise "
    FFMPEG_MAKE_EXTRA_ARGS="-j4"
else
    # disable all unnecessary features for production build
    DISABLE_FFMPEG_COMPONENTS+=" --disable-doc --disable-sdl2 --disable-iconv --disable-muxers --disable-demuxers --disable-parsers --disable-protocols "
    DISABLE_FFMPEG_COMPONENTS+=" --disable-encoders --disable-decoders --disable-filters --disable-bsfs --disable-postproc --disable-lzma "
fi

if [ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone https://github.com/livepeer/FFmpeg.git "$ROOT/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$ROOT/ffmpeg"
  git checkout 1ece0e65b1ce2e330e673df0cda23b904979a1a6
  ./configure ${TARGET_OS:-} $DISABLE_FFMPEG_COMPONENTS --fatal-warnings \
    --enable-libx264 --enable-gpl \
    --enable-protocol=rtmp,file,pipe \
    --enable-muxer=mpegts,hls,segment,mp4,hevc,matroska,webm,null --enable-demuxer=flv,mpegts,mp4,mov,webm,matroska \
    --enable-bsf=h264_mp4toannexb,aac_adtstoasc,h264_metadata,h264_redundant_pps,hevc_mp4toannexb,extract_extradata \
    --enable-parser=aac,aac_latm,h264,hevc,vp8,vp9 \
    --enable-filter=abuffer,buffer,abuffersink,buffersink,afifo,fifo,aformat,format \
    --enable-filter=aresample,asetnsamples,fps,scale,hwdownload,select,livepeer_dnn,signature \
    --enable-encoder=aac,opus,libx264 \
    --enable-decoder=aac,opus,h264 \
    --extra-cflags="-I${ROOT}/compiled/include" \
    --extra-ldflags="-L${ROOT}/compiled/lib ${EXTRA_LDFLAGS}" \
    --prefix="$ROOT/compiled" \
    $EXTRA_FFMPEG_FLAGS \
    $DEV_FFMPEG_FLAGS
fi

if [[ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" || $BUILD_TAGS == *"debug-video"* ]]; then
  cd "$ROOT/ffmpeg"
  make -j$(nproc) $FFMPEG_MAKE_EXTRA_ARGS
  make -j$(nproc) install
fi
