#!/usr/bin/env bash

set -exuo pipefail

ROOT="${1:-$HOME}"
NPROC=${NPROC:-$(nproc)}
EXTRA_CFLAGS=""
EXTRA_LDFLAGS=""
EXTRA_X264_FLAGS=""
EXTRA_FFMPEG_FLAGS=""
BUILD_TAGS="${BUILD_TAGS:-}"

# Build platform flags
BUILDOS=$(uname -s | tr '[:upper:]' '[:lower:]')
BUILDARCH=$(uname -m | tr '[:upper:]' '[:lower:]')
if [[ $BUILDARCH == "aarch64" ]]; then
  BUILDARCH=arm64
fi
if [[ $BUILDARCH == "x86_64" ]]; then
  BUILDARCH=amd64
fi

# Override these for cross-compilation
export GOOS="${GOOS:-$BUILDOS}"
export GOARCH="${GOARCH:-$BUILDARCH}"

echo "BUILDOS: $BUILDOS"
echo "BUILDARCH: $BUILDARCH"
echo "GOOS: $GOOS"
echo "GOARCH: $GOARCH"

function check_sysroot() {
  if ! stat $SYSROOT > /dev/null; then
    echo "cross-compilation sysroot not found at $SYSROOT, try setting SYSROOT to the correct path"
    exit 1
  fi
}

if [[ "$BUILDARCH" == "amd64" && "$BUILDOS" == "linux" && "$GOARCH" == "arm64" && "$GOOS" == "linux" ]]; then
  echo "cross-compiling linux-amd64 --> linux-arm64"
  export CC="clang-14"
  export STRIP="llvm-strip-14"
  export AR="llvm-ar-14"
  export RANLIB="llvm-ranlib-14"
  EXTRA_CFLAGS="--target=aarch64-linux-gnu -I/usr/local/cuda_arm64/include $EXTRA_CFLAGS"
  EXTRA_LDFLAGS="-fuse-ld=lld --target=aarch64-linux-gnu -L/usr/local/cuda_arm64/lib64 $EXTRA_LDFLAGS"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --arch=aarch64 --enable-cross-compile --cc=clang --strip=llvm-strip-14"
  HOST_OS="--host=aarch64-linux-gnu"
fi

if [[ "$BUILDARCH" == "arm64" && "$BUILDOS" == "darwin" && "$GOARCH" == "arm64" && "$GOOS" == "linux" ]]; then
  SYSROOT="${SYSROOT:-"/tmp/sysroot-aarch64-linux-gnu"}"
  check_sysroot
  echo "cross-compiling darwin-arm64 --> linux-arm64"
  LLVM_PATH="${LLVM_PATH:-/opt/homebrew/opt/llvm/bin}"
  if [[ ! -f "$LLVM_PATH/ld.lld" ]]; then
    echo "llvm linker not found at '$LLVM_PATH/ld.lld'. try 'brew install llvm' or set LLVM_PATH to your LLVM bin directory"
    exit 1
  fi
  export CC="$LLVM_PATH/clang --sysroot=$SYSROOT"
  export AR="/opt/homebrew/opt/llvm/bin/llvm-ar"
  export RANLIB="/opt/homebrew/opt/llvm/bin/llvm-ranlib"
  EXTRA_CFLAGS="--target=aarch64-linux-gnu $EXTRA_CFLAGS"
  EXTRA_LDFLAGS="--target=aarch64-linux-gnu -fuse-ld=$LLVM_PATH/ld.lld $EXTRA_LDFLAGS"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --arch=aarch64 --enable-cross-compile --cc=$LLVM_PATH/clang --sysroot=$SYSROOT --ar=$AR --ranlib=$RANLIB --target-os=linux"
  EXTRA_X264_FLAGS="$EXTRA_X264_FLAGS --sysroot=$SYSROOT --ar=$AR --ranlib=$RANLIB"
  HOST_OS="--host=aarch64-linux-gnu"
fi

if [[ "$BUILDOS" == "linux" && "$GOARCH" == "amd64" && "$GOOS" == "windows" ]]; then
  echo "cross-compiling linux-$BUILDARCH --> windows-amd64"
  SYSROOT="${SYSROOT:-"/usr/x86_64-w64-mingw32"}"
  check_sysroot
  EXTRA_CFLAGS="-L$SYSROOT/lib -I$SYSROOT/include  $EXTRA_CFLAGS"
  EXTRA_LDFLAGS="-L$SYSROOT/lib $EXTRA_LDFLAGS"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --arch=x86_64 --enable-cross-compile --cross-prefix=x86_64-w64-mingw32- --target-os=mingw64 --sysroot=$SYSROOT"
  EXTRA_X264_FLAGS="$EXTRA_X264_FLAGS --cross-prefix=x86_64-w64-mingw32- --sysroot=$SYSROOT"
  HOST_OS="--host=mingw64"
  # Workaround for https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=967969
  export PKG_CONFIG_LIBDIR="/usr/local/x86_64-w64-mingw32/lib/pkgconfig"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --pkg-config=$(which pkg-config)"
fi

if [[ "$BUILDARCH" == "amd64" && "$BUILDOS" == "darwin" && "$GOARCH" == "arm64" && "$GOOS" == "darwin" ]]; then
  echo "cross-compiling darwin-amd64 --> darwin-arm64"
  EXTRA_CFLAGS="$EXTRA_CFLAGS --target=arm64-apple-macos11"
  EXTRA_LDFLAGS="$EXTRA_LDFLAGS --target=arm64-apple-macos11"
  HOST_OS="--host=aarch64-darwin"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --arch=aarch64 --enable-cross-compile"
fi

# Windows (MSYS2) needs a few tweaks
if [[ "$BUILDOS" == *"MSYS"* ]]; then
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

export PATH="$ROOT/compiled/bin:${PATH}"
export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-}:$ROOT/compiled/lib/pkgconfig"

mkdir -p "$ROOT/"

# NVENC only works on Windows/Linux
if [[ "$GOOS" != "darwin" ]]; then
  if [[ ! -e "$ROOT/nv-codec-headers" ]]; then
    git clone https://git.videolan.org/git/ffmpeg/nv-codec-headers.git "$ROOT/nv-codec-headers"
    cd $ROOT/nv-codec-headers
    git checkout n9.1.23.1
    make -e PREFIX="$ROOT/compiled"
    make install -e PREFIX="$ROOT/compiled"
  fi
fi

if [[ "$GOOS" != "windows" && "$GOARCH" == "amd64" ]]; then
  if [[ ! -e "$ROOT/nasm-2.14.02" ]]; then
    # sudo apt-get -y install asciidoc xmlto # this fails :(
    cd "$ROOT"
    curl -o nasm-2.14.02.tar.gz https://www.nasm.us/pub/nasm/releasebuilds/2.14.02/nasm-2.14.02.tar.gz
    echo 'b34bae344a3f2ed93b2ca7bf25f1ed3fb12da89eeda6096e3551fd66adeae9fc  nasm-2.14.02.tar.gz' >nasm-2.14.02.tar.gz.sha256
    sha256sum -c nasm-2.14.02.tar.gz.sha256
    tar xf nasm-2.14.02.tar.gz
    rm nasm-2.14.02.tar.gz nasm-2.14.02.tar.gz.sha256
    cd "$ROOT/nasm-2.14.02"
    ./configure --prefix="$ROOT/compiled"
    make -j$NPROC
    make -j$NPROC install || echo "Installing docs fails but should be OK otherwise"
  fi
fi

if [[ ! -e "$ROOT/x264" ]]; then
  git clone http://git.videolan.org/git/x264.git "$ROOT/x264"
  cd "$ROOT/x264"
  if [[ $GOARCH == "arm64" ]]; then
    # newer git master, compiles on Apple Silicon
    git checkout 66a5bc1bd1563d8227d5d18440b525a09bcf17ca
  else
    # older git master, does not compile on Apple Silicon
    git checkout 545de2ffec6ae9a80738de1b2c8cf820249a2530
  fi
  ./configure --prefix="$ROOT/compiled" --enable-pic --enable-static ${HOST_OS:-} --disable-cli --extra-cflags="$EXTRA_CFLAGS" --extra-asflags="$EXTRA_CFLAGS" --extra-ldflags="$EXTRA_LDFLAGS" $EXTRA_X264_FLAGS || (cat $ROOT/x264/config.log && exit 1)
  make -j$NPROC
  make -j$NPROC install-lib-static
fi

if [[ "$GOOS" == "linux" && "$BUILD_TAGS" == *"debug-video"* ]]; then
  sudo apt-get install -y libnuma-dev cmake
  if [[ ! -e "$ROOT/x265" ]]; then
    git clone https://bitbucket.org/multicoreware/x265_git.git "$ROOT/x265"
    cd "$ROOT/x265"
    git checkout 17839cc0dc5a389e27810944ae2128a65ac39318
    cd build/linux/
    cmake -DCMAKE_INSTALL_PREFIX=$ROOT/compiled -G "Unix Makefiles" ../../source
    make -j$NPROC
    make -j$NPROC install
  fi
  # VP8/9 support
  if [[ ! -e "$ROOT/libvpx" ]]; then
    git clone https://chromium.googlesource.com/webm/libvpx.git "$ROOT/libvpx"
    cd "$ROOT/libvpx"
    git checkout ab35ee100a38347433af24df05a5e1578172a2ae
    ./configure --prefix="$ROOT/compiled" --disable-examples --disable-unit-tests --enable-vp9-highbitdepth --enable-shared --as=nasm
    make -j$NPROC
    make -j$NPROC install
  fi
fi

DISABLE_FFMPEG_COMPONENTS=""
EXTRA_FFMPEG_LDFLAGS="$EXTRA_LDFLAGS"
# all flags which should be present for production build, but should be replaced/removed for debug build
DEV_FFMPEG_FLAGS=""

if [[ "$BUILDOS" == "darwin" && "$GOOS" == "darwin" ]]; then
  EXTRA_FFMPEG_LDFLAGS="$EXTRA_FFMPEG_LDFLAGS -framework CoreFoundation -framework Security"
elif [[ "$GOOS" == "windows" ]]; then
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid,hevc_cuvid,vp8_cuvid,vp9_cuvid --enable-filter=scale_cuda,signature_cuda,hwupload_cuda --enable-encoder=h264_nvenc,hevc_nvenc"
elif [[ -e "/usr/local/cuda/lib64" ]]; then
  echo "CUDA SDK detected, building with GPU support"
  EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-nonfree --enable-cuda-nvcc --enable-libnpp --enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvenc --enable-decoder=h264_cuvid,hevc_cuvid,vp8_cuvid,vp9_cuvid --enable-filter=scale_npp,signature_cuda,hwupload_cuda --enable-encoder=h264_nvenc,hevc_nvenc"
  if [[ "$GOARCH" == "amd64" && ! -e "${ROOT}/compiled/lib/libtensorflow_framework.so" ]]; then
    LIBTENSORFLOW_VERSION=2.12.1 &&
      curl -LO https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz &&
      tar -C ${ROOT}/compiled/ -xzf libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz &&
      rm libtensorflow-gpu-linux-x86_64-${LIBTENSORFLOW_VERSION}.tar.gz
    EXTRA_FFMPEG_FLAGS="$EXTRA_FFMPEG_FLAGS --enable-libtensorflow"
  fi
else
  echo "No CUDA SDK detected, building without GPU support"
fi

if [[ $BUILD_TAGS == *"debug-video"* ]]; then
  echo "video debug mode, building ffmpeg with tools, debug info and additional capabilities for running tests"
  DEV_FFMPEG_FLAGS="--enable-muxer=md5,flv --enable-demuxer=hls --enable-filter=ssim,tinterlace --enable-encoder=wrapped_avframe,pcm_s16le "
  DEV_FFMPEG_FLAGS+="--enable-shared --enable-debug=3 --disable-stripping --disable-optimizations --enable-encoder=libx265,libvpx_vp8,libvpx_vp9 "
  DEV_FFMPEG_FLAGS+="--enable-decoder=hevc,libvpx_vp8,libvpx_vp9 --enable-libx265 --enable-libvpx --enable-bsf=noise "
else
  # disable all unnecessary features for production build
  DISABLE_FFMPEG_COMPONENTS+=" --disable-doc --disable-sdl2 --disable-iconv --disable-muxers --disable-demuxers --disable-parsers --disable-protocols "
  DISABLE_FFMPEG_COMPONENTS+=" --disable-encoders --disable-decoders --disable-filters --disable-bsfs --disable-postproc --disable-lzma "
fi

if [[ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" ]]; then
  git clone https://github.com/livepeer/FFmpeg.git "$ROOT/ffmpeg" || echo "FFmpeg dir already exists"
  cd "$ROOT/ffmpeg"
  git checkout 2e18d069668c143f3c251067abd25389e411d022
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
    --extra-cflags="${EXTRA_CFLAGS} -I${ROOT}/compiled/include -I/usr/local/cuda/include" \
    --extra-ldflags="${EXTRA_FFMPEG_LDFLAGS} -L${ROOT}/compiled/lib -L/usr/local/cuda/lib64" \
    --prefix="$ROOT/compiled" \
    $EXTRA_FFMPEG_FLAGS \
    $DEV_FFMPEG_FLAGS || (tail -100 ${ROOT}/ffmpeg/ffbuild/config.log && exit 1)
    # If configure fails, then print the last 100 log lines for debugging and exit.
fi

if [[ ! -e "$ROOT/ffmpeg/libavcodec/libavcodec.a" || $BUILD_TAGS == *"debug-video"* ]]; then
  cd "$ROOT/ffmpeg"
  make -j$NPROC
  make -j$NPROC install
fi
