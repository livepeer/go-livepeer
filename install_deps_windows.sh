set -x
set -o errexit
set -o pipefail
set -o nounset

export PATH="$PATH:/c/msys64/usr/bin:/c/msys64/mingw64/bin"
export HOME="/build"
mkdir -p $HOME

export PATH="$HOME/compiled/bin":$PATH
export PKG_CONFIG_PATH=$HOME/compiled/lib/pkgconfig

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
  ./configure --prefix="$HOME/compiled" --enable-pic --enable-static --host=x86_64-w64-mingw32 --disable-cli
  make
  make install-lib-static
fi
