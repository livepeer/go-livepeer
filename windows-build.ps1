$ErrorActionPreference = 'Stop';
$ProgressPreference = 'SilentlyContinue';
$Env:CHERE_INVOKING = "1"

# Remove previous MSYS2 install
Get-ChildItem ./msys2 -Recurse | Remove-Item

# Helper function to invoke msys2 bash, ignore stderr, throw an error if bash exits non-zero
function Run-Bash {
  param ( $Command )
  .\msys2\usr\bin\bash.exe -l -c "
    exec 2>&1 # Redirect all stderr to stdout so that Powershell doesn't see it as an error
    set -ex 
    $Command
  "
  if ($LASTEXITCODE -ne 0)
  {
    throw "Exit code $LASTEXITCODE"
  }
}

if (!(Test-Path "./msys2")) {
  $env:ChocolateyInstall="./msys2-install"

  # Install MSYS2
  choco install -y msys2 --force --no-progress --params="/InstallDir:./msys2/ /NoUpdate /NoPath"

  # MSYS2 core system update
  Run-Bash 'pacman -Syu --needed --noconfirm --noprogressbar --ask=20'

  # MSYS2 core system update part 2
  Run-Bash 'pacman -Su --needed --noconfirm --noprogressbar --ask=20'

  # MSYS2 package update
  Run-Bash 'pacman -Fy --noconfirm --noprogressbar --ask=20'
}

# Install needed packages and build
Run-Bash '
  pacman -S --noconfirm --noprogressbar --ask=20 \
    perl binutils git make autoconf zip \
    mingw-w64-x86_64-gcc mingw-w64-x86_64-libtool mingw-w64-x86_64-gnutls \
    mingw-w64-x86_64-pkg-config mingw-w64-x86_64-go mingw-w64-x86_64-clang
    
  export PATH="/mingw64/bin:/go/bin:/usr/local/bin:/usr/bin:/bin:/opt/bin:/c/Windows/System32:/c/Windows:/c/Windows/System32/Wbem:/c/Windows/System32/WindowsPowerShell/v1.0/:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl"
  export PKG_CONFIG_PATH=/mingw64/lib/pkgconfig:/build/compiled/lib/pkgconfig 
  export GOROOT=/mingw64/lib/go
  ./install_ffmpeg.sh 
  source ./ci_env.sh && make livepeer livepeer_cli
  ./upload_build.sh 
'

Write-Output "Performing self-check; if we fail after this line there are missing DLLs"
cd livepeer-windows-amd64
.\livepeer.exe -version
