
if (Test-Path 'windowsbuild.ps1') {
  Write-Host 'Should be run from `go-livepeer` repository root, like this: `docker\windowsbuild.ps1`';
  exit 1
}
Write-Host 'Building livepeer node...';

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
docker build -m 4gb -t 'livepeerci/build-platform:latest' -f .\docker\Dockerfile.build-windows .
docker build -m 4gb -t 'livepeerci/build:latest' -f .\docker\Dockerfile.build .
docker run --name lpbuild 'livepeerci/build:latest'
docker cp lpbuild:c:/msys64/go/src/github.com/livepeer/go-livepeer/livepeer-windows-amd64.zip .

Write-Host 'Livepeer node executable saved to "livepeer-windows-amd64.zip"'
