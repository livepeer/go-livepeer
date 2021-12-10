#!/usr/bin/env bash

set -ex

ROOT="${1:-$HOME}"

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

# Static linking of gnutls on Linux/Mac
if [[ $(uname) != *"MSYS"* ]]; then
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

  # rm -rf "$ROOT/gmp-6.1.2"
  if [ ! -e "$ROOT/gmp-6.1.2" ]; then
    cd "$ROOT"
    curl -LO https://github.com/livepeer/livepeer-builddeps/raw/34900f2b1be4e366c5270e3ee5b0d001f12bd8a4/gmp-6.1.2.tar.xz
    tar xf gmp-6.1.2.tar.xz
    cd "$ROOT/gmp-6.1.2"
    ./configure --prefix="$ROOT/compiled" --disable-shared  --with-pic --enable-fat
    make
    make install
  fi

  # rm -rf "$ROOT/nettle-3.7"
  if [ ! -e "$ROOT/nettle-3.7" ]; then
    cd $ROOT
    curl -LO https://github.com/livepeer/livepeer-builddeps/raw/657a86b78759b1ab36dae227253c26ff50cb4b0a/nettle-3.7.tar.gz
    tar xf nettle-3.7.tar.gz
    cd nettle-3.7
    LDFLAGS="-L${ROOT}/compiled/lib" CFLAGS="-I${ROOT}/compiled/include" ./configure --prefix="$ROOT/compiled" --disable-shared --enable-pic
    make
    make install
  fi

fi

# H.264 support
if [ ! -e "$ROOT/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$ROOT/x264"
  cd "$ROOT/x264"
  # git master as of this writing
  git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  ./configure --prefix="$ROOT/compiled" --enable-pic --enable-static ${HOST_OS:-} --disable-cli
  make
  make install-lib-static
fi

if [ ! -e "$ROOT/x265" ]; then
  if [ $(uname) == "Linux" ]; then
    sudo apt-get install -y libnuma-dev cmake
  fi
  git clone https://bitbucket.org/multicoreware/x265_git.git "$ROOT/x265"
  cd "$ROOT/x265"
  git checkout 17839cc0dc5a389e27810944ae2128a65ac39318
  cd build/linux/
  cmake -DCMAKE_INSTALL_PREFIX=$ROOT/compiled -G "Unix Makefiles" ../../source
  make
  make install
fi

# VP8/9 support
if [ ! -e "$ROOT/libvpx" ]; then
  git clone https://chromium.googlesource.com/webm/libvpx.git "$ROOT/libvpx"
  cd "$ROOT/libvpx"
  git checkout ab35ee100a38347433af24df05a5e1578172a2ae
  ./configure --prefix="$ROOT/compiled" --disable-examples --disable-unit-tests --enable-vp9-highbitdepth --enable-shared --as=nasm
  make
  make install
fi

if [ ! -e "$ROOT/gnutls-3.7.0" ]; then
  EXTRA_GNUTLS_LIBS=""
  if [[ $(uname) == *"MSYS"* ]]; then
    EXTRA_GNUTLS_LIBS="-lncrypt -lcrypt32 -lwsock32 -lws2_32 -lwinpthread"
  fi
  cd $ROOT
  curl -LO https://www.gnupg.org/ftp/gcrypt/gnutls/v3.7/gnutls-3.7.0.tar.xz
  tar xf gnutls-3.7.0.tar.xz
  cd gnutls-3.7.0
  LDFLAGS="-L${ROOT}/compiled/lib" CFLAGS="-I${ROOT}/compiled/include -O2" LIBS="-lhogweed -lnettle -lgmp $EXTRA_GNUTLS_LIBS" ./configure ${BUILD_OS:-} --prefix="$ROOT/compiled" --enable-static --disable-shared --with-pic --with-included-libtasn1 --with-included-unistring --without-p11-kit --without-idn --without-zlib --disable-doc --disable-cxx --disable-tools --disable-hardware-acceleration --disable-guile --disable-libdane --disable-tests --disable-rpath --disable-nls
  make
  make install
  # gnutls doesn't properly set up its pkg-config or something? without this line ffmpeg and go
  # don't know that they need gmp, nettle, and hogweed
  sed -i'' -e "s/-lgnutls/-lgnutls -lhogweed -lnettle -lgmp $EXTRA_GNUTLS_LIBS/g" $ROOT/compiled/lib/pkgconfig/gnutls.pc
fi

EXTRA_FFMPEG_FLAGS=""
EXTRA_LDFLAGS=""
# all flags which should present for production build, but should be replaced/removed for debug build
DEV_FFMPEG_FLAGS="--disable-programs"
FFMPEG_MAKE_EXTRA_ARGS=""

if [ $(uname) == "Darwin" ]; then
  EXTRA_LDFLAGS="-framework CoreFoundation -framework Security"
else
  # If we have clang, we can compile with CUDA support!
  if which nvidia-smi > /dev/null; then
    echo "CUDA detected, building with GPU support"
    sudo apt install -y clang
    EXTRA_FFMPEG_FLAGS="--enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid,hevc_cuvid,vp8_cuvid,vp9_cuvid --enable-filter=scale_cuda,signature_cuda,hwupload_cuda --enable-encoder=h264_nvenc,hevc_nvenc"
    if [[ $BUILD_TAGS == *"experimental"* ]]; then
        LIBTENSORFLOW_VERSION=2.3.0 \
        && curl -LO https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-cpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz \
        && sudo tar -C /usr/local -xzf libtensorflow-cpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz \
        && sudo ldconfig
        echo "experimental tag detected, building with Tensorflow support"
        EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-libtensorflow"
    fi
  fi
fi

if [[ $BUILD_TAGS == *"debug-video"* ]]; then
    echo "video debug mode, building ffmpeg with tools and debug info"
    DEV_FFMPEG_FLAGS="--enable-filter=ssim --enable-encoder=wrapped_avframe,pcm_s16le --enable-shared --enable-debug=3 --disable-stripping --disable-optimizations"
    FFMPEG_MAKE_EXTRA_ARGS="-j4"
fi

if [ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" ]; then
  git clone https://github.com/livepeer/FFmpeg.git "$ROOT/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$ROOT/ffmpeg"
  git checkout 682c4189d8364867bcc49f9749e04b27dc37cded
  ./configure ${TARGET_OS:-} --fatal-warnings \
    --disable-doc --disable-sdl2 --disable-iconv \
    --disable-muxers --disable-demuxers --disable-parsers --disable-protocols \
    --disable-encoders --disable-decoders --disable-filters --disable-bsfs \
    --disable-postproc --disable-lzma \
    --enable-gnutls --enable-libx264 --enable-libx265 --enable-libvpx --enable-gpl \
    --enable-protocol=https,http,rtmp,file,pipe \
    --enable-muxer=mpegts,hls,segment,mp4,hevc,matroska,opus,webm,webm_chunk,webm_dash_manifest,null --enable-demuxer=flv,mpegts,mp4,mov,matroska,webm_dash_manifest \
    --enable-bsf=h264_mp4toannexb,aac_adtstoasc,h264_metadata,h264_redundant_pps,hevc_mp4toannexb,extract_extradata \
    --enable-parser=aac,aac_latm,h264,hevc,vp8,vp9 \
    --enable-filter=abuffer,buffer,abuffersink,buffersink,afifo,fifo,aformat,format \
    --enable-filter=aresample,asetnsamples,fps,scale,hwdownload,select,livepeer_dnn,signature \
    --enable-encoder=aac,libx264,libx265,libvpx_vp8,libvpx_vp9 \
    --enable-decoder=aac,h264,hevc,libvpx_vp8,libvpx_vp9 \
    --extra-cflags="-I${ROOT}/compiled/include" \
    --extra-ldflags="-L${ROOT}/compiled/lib ${EXTRA_LDFLAGS}" \
    --prefix="$ROOT/compiled" \
    $EXTRA_FFMPEG_FLAGS \
    $DEV_FFMPEG_FLAGS
fi

if [[ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" || $BUILD_TAGS == *"debug-video"* ]]; then
  cd "$ROOT/ffmpeg"
  make $FFMPEG_MAKE_EXTRA_ARGS
  make install
fi
