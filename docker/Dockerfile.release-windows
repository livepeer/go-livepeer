# escape=`
FROM livepeerci/build:latest as packager

# Package all the necessary DLLs and such
RUN ./upload_build.sh

FROM mcr.microsoft.com/windows/servercore:1809

COPY --from=packager C:\msys64\go\src\github.com\livepeer\go-livepeer\livepeer-windows-amd64 C:\livepeer-windows-amd64

ENTRYPOINT ["C:\\livepeer-windows-amd64\\livepeer.exe"]
