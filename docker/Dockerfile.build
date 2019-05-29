# This script is downstream of build-platform and works for both Windows and Linux.

# Don't mess with WORKDIR if you can avoid it, as it's different on Windows/Linux. Use relative
# paths instead.

FROM livepeerci/build-platform:latest

ADD ./install_ffmpeg.sh ./install_ffmpeg.sh
RUN ./install_ffmpeg.sh

ADD ./install_dependencies.sh ./install_dependencies.sh
RUN ./install_dependencies.sh

ADD . .
RUN make livepeer livepeer_cli

CMD  ./upload_build.sh
