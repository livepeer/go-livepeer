# escape=`

# This kinda funny-looking Dockerfile successfully builds go-livepeer on Windows
# using mingw64. It has a dynamic depenency on a variety of mingw64 DLLs, so its
# artifact is a zip file that contains livepeer.exe, livepeer-cli.exe, and a
# handful of DLLs. `docker cp` does work on Windows, but only for stopped containers.

# To build windows executable, just run `.\docker\windowsbuild.ps1`.

# escape=`

# Builds ffmpeg libs on windows. To be used as base image
# for building livepeer no on windows.

# docker build -m 4gb -t livepeer/ffmpeg-base:windows -f Dockerfile.ffmpeg-windows .
# docker push livepeer/ffmpeg-base:windows

# windowsservercore snapshots are kinda unstable but dotnet/framework/sdk snapshots don't change
FROM mcr.microsoft.com/dotnet/framework/sdk:4.8-20190514-windowsservercore-ltsc2019

WORKDIR c:/temp
SHELL ["powershell", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]

# This commented section includes what's necessary for CUDA GPU support on Windows. But it doesn't work yet.

### START GPU SECTION

# RUN (new-object System.Net.WebClient).DownloadFile('https://developer.nvidia.com/compute/cuda/10.1/Prod/local_installers/cuda_10.1.168_425.25_win10.exe','C:\temp\cuda_10.1.168_425.25_win10.exe') ; `
#   Start-Process .\cuda_10.1.168_425.25_win10.exe -ArgumentList '-s npp_dev_10.1 nvcc_10.1' -Wait ; `
#   Remove-Item .\cuda_10.1.168_425.25_win10.exe

# # nvcc requires visual studio's cl.exe
# RUN (new-object System.Net.WebClient).DownloadFile('https://aka.ms/vs/16/release/vs_buildtools.exe','C:\temp\vs_buildtools.exe')

# # https://docs.microsoft.com/en-us/visualstudio/install/build-tools-container?view=vs-2019
# SHELL ["cmd", "/S", "/C"]
# RUN C:\temp\vs_buildtools.exe --quiet --wait --norestart --nocache `
#   --installPath C:\BuildTools `
#   --add Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
#   --remove Microsoft.VisualStudio.Component.Windows10SDK.10240 `
#   --remove Microsoft.VisualStudio.Component.Windows10SDK.10586 `
#   --remove Microsoft.VisualStudio.Component.Windows10SDK.14393 `
#   --remove Microsoft.VisualStudio.Component.Windows81SDK `
#   || IF "%ERRORLEVEL%"=="3010" EXIT 0

# SHELL ["powershell", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]

# If you're uncommenthing this, translate this line to Powershell please:
# RUN mv /c/Program\ Files/NVIDIA\ GPU\ Computing\ Toolkit /c/nvidia

### END GPU SECTION

# Download and extract MSYS2 with 7zip.

RUN Invoke-WebRequest -UserAgent 'DockerCI' -outfile 7zsetup.exe http://www.7-zip.org/a/7z1604-x64.exe

RUN Start-Process .\7zsetup -ArgumentList '/S /D=c:/7zip' -Wait

RUN (new-object System.Net.WebClient).DownloadFile('http://repo.msys2.org/distrib/msys2-x86_64-latest.tar.xz','C:\temp\msys2-x86_64-latest.tar.xz')
RUN Get-FileHash -Path msys2-x86_64-latest.tar.xz
RUN C:\7zip\7z e msys2-x86_64-latest.tar.xz -Wait
RUN C:\7zip\7z x msys2-x86_64-latest.tar -o"C:\\"

# Install MSYS2 and our required packages

RUN Write-Host 'Updating MSYSTEM and MSYSCON ...'; `
  [Environment]::SetEnvironmentVariable('MSYSTEM', 'MSYS2', [EnvironmentVariableTarget]::Machine); `
  [Environment]::SetEnvironmentVariable('MSYSCON', 'defterm', [EnvironmentVariableTarget]::Machine);

RUN C:\msys64\usr\bin\bash.exe -l -c 'exit 0'; `
  C:\msys64\usr\bin\bash.exe -l -c 'echo "Now installing MSYS2..."'; `
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -Syuu --needed --noconfirm --noprogressbar --ask=20'; `
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -Syu  --needed --noconfirm --noprogressbar --ask=20'; `
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -Su   --needed --noconfirm --noprogressbar --ask=20'; `
  # Install our packages. Use the mingw-w64-x86_64 version of everything run-time; it's faster
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -S perl binutils git make autoconf zip mingw-w64-x86_64-gcc mingw-w64-x86_64-libtool mingw-w64-x86_64-gnutls mingw-w64-x86_64-pkg-config --noconfirm --noprogressbar --ask=20'; `
  # Need golang 1.11 for now
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -U http://repo.msys2.org/mingw/x86_64/mingw-w64-x86_64-go-1.12.4-1-any.pkg.tar.xz --noconfirm --noprogressbar --ask=20'; `
  C:\msys64\usr\bin\bash.exe -l -c 'pacman -Scc --noconfirm'; `
  C:\msys64\usr\bin\bash.exe -l -c 'echo "Successfully installed MSYS2"'; `
  # MSYS2 leaves processes running or something - this step hangs? Hard shutdown to fix that.
  Stop-Computer -Force

WORKDIR C:\msys64\go\src\github.com\livepeer\go-livepeer

# Tell MSYS2 not to do "cd $HOME" all the freakin' time
ENV CHERE_INVOKING 1
# Add all mingw64 and CUDA paths to PATH
ENV PATH "/mingw64/bin:/go/bin:/c/nvidia/CUDA/v10.1/bin:/c/nvidia/CUDA/v10.1/nvvm/bin:/usr/local/bin:/usr/bin:/bin:/opt/bin:/c/BuildTools/VC/Tools/MSVC/14.21.27702/bin/Hostx64/x64:/c/Windows/System32:/c/Windows:/c/Windows/System32/Wbem:/c/Windows/System32/WindowsPowerShell/v1.0/:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl"
ENV HOME /build
ENV GOPATH "/go"
ENV GOROOT "/mingw64/lib/go"
ENV PKG_CONFIG_PATH "/mingw64/lib/pkgconfig:/build/compiled/lib/pkgconfig"

SHELL ["C:\\msys64\\usr\\bin\\bash.exe", "-c"]

CMD ["C:\\msys64\\usr\\bin\\bash.exe"]
