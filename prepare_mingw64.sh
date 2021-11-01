#!/bin/bash

set -eo pipefail
set -x

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

export MINGW_INSTALLS=mingw64
pacman -S --noconfirm --noprogressbar --ask=20 --needed mingw-w64-x86_64-binutils mingw-w64-x86_64-gcc mingw-w64-x86_64-pkg-config mingw-w64-x86_64-go mingw-w64-x86_64-nasm mingw-w64-x86_64-clang git make autoconf automake patch libtool texinfo gtk-doc zip zlib
gpg --keyserver keyserver.ubuntu.com --recv-keys F3599FF828C67298 249B39D24F25E3B6 2071B08A33BD3F06 29EE58B996865171 D605848ED7E69871

echo "removing all dlls"
# https://narkive.com/Fjlrbrjg:20.646.48
find /mingw64 -name "*.dll.a" -exec rm -v {} \;
