set -ex

export PATH="$PATH:/usr/bin:/mingw64/bin"
export C_INCLUDE_PATH="${C_INCLUDE_PATH:-}:/msys64/mingw64/lib"
export HOME="/build"
mkdir -p $HOME

export PATH="$HOME/compiled/bin":$PATH
export PKG_CONFIG_PATH=$HOME/compiled/lib/pkgconfig:/mingw64/lib/pkgconfig

export TARGET_OS="--target-os=mingw64"
export HOST_OS="--host=x86_64-w64-mingw32"

bash /c/temp/install_ffmpeg.sh
