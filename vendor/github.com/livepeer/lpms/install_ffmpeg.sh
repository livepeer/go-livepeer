set -ex

export PATH=$PATH:"$HOME/compiled/bin"

if [ ! -e "$HOME/nasm/nasm" ]; then
  # sudo apt-get -y install asciidoc xmlto # this fails :(
  git clone -b nasm-2.13.02 http://repo.or.cz/nasm.git "$HOME/nasm"
  cd "$HOME/nasm"
  ./autogen.sh
  ./configure --prefix="$HOME/compiled"
  make
  sudo make install || echo "Installing docs fails but should be OK otherwise"
fi

if [ ! -e "$HOME/x264/x264" ]; then
  git clone http://git.videolan.org/git/x264.git "$HOME/x264"
  cd "$HOME/x264"
  ./configure --prefix="$HOME/compiled" --enable-pic --enable-static
  make
  sudo make install-lib-static
fi

if [ ! -e "$HOME/ffmpeg/ffmpeg" ]; then
  git clone https://git.ffmpeg.org/ffmpeg.git "$HOME/ffmpeg"
  cd "$HOME/ffmpeg"
  ./configure --prefix="$HOME/compiled" --enable-gpl --enable-libx264
  make
  sudo make install
fi
