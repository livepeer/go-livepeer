
if (Test-Path 'Dockerfile.windows') {
  Write-Host 'Should be run from `go-livepeer` repository root, like this: `docker\windowsbuild.ps1`';
  exit 1
}
Write-Host 'Building livepeer node...';

docker build -m 4gb -t livepeer-windows -f .\docker\Dockerfile.windows .
docker run --name wlive livepeer-windows
docker cp wlive:c:/temp/livepeer-windows-amd64.zip .
docker rm wlive

Write-Host 'Livepeer node executable saved to "livepeer-windows-amd64.zip"'
