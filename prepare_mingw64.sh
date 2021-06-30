#!/bin/bash

set -eo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

export MINGW_INSTALLS=mingw64
pacman -S --noconfirm --noprogressbar --ask=20 --needed mingw-w64-x86_64-binutils mingw-w64-x86_64-gcc mingw-w64-x86_64-pkg-config mingw-w64-x86_64-go mingw-w64-x86_64-nasm mingw-w64-x86_64-clang git make autoconf automake patch libtool texinfo gtk-doc zip
gpg --keyserver keyserver.ubuntu.com --recv-keys F3599FF828C67298 249B39D24F25E3B6 2071B08A33BD3F06 29EE58B996865171 D605848ED7E69871

# winpthreads needs git config for some reason
git config --get user.email || git config --global user.email "fakeemail@example.com"
git config --get user.name || git config --global user.name "Livepeer Windows Build"

rm -rf "$DIR/.build"
mkdir -p "$DIR/.build"
git clone https://github.com/msys2/MINGW-packages.git "$DIR/.build/MINGW-packages"
cd "$DIR/.build/MINGW-packages"
git checkout f4c989755fdbca217e0c53c01a6206d36a9c3e0d

for x in zlib gmp nettle winpthreads-git; do
  echo ""
  echo "---------"
  echo "building $x"
  echo "---------"
  echo ""
  cd "$DIR/.build/MINGW-packages/mingw-w64-$x"
  set +e
  makepkg-mingw -CL --nocheck
  EXIT=$?
  set -e
  if [[ $EXIT != "13" ]] && [[ $EXIT != "0" ]]; then
    echo "exiting $EXIT"
    exit $EXIT
  fi
  pacman -U --noconfirm *.zst
  echo "prepare_mingw64: deleting shared libraries"
  # https://narkive.com/Fjlrbrjg:20.646.48
  find /mingw64/lib -name "*.dll.a" -exec rm -v {} \;
done
# And one more dynamic library to remove, this one in the system itself...
rm -rfv /mingw64/x86_64-w64-mingw32/lib/libwinpthread.dll.a
