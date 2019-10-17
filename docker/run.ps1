# Shim to properly invoke Bash from "RUN" commands on Windows
$ErrorActionPreference = 'Stop';
$ProgressPreference = 'SilentlyContinue';
$output = Start-Process -FilePath C:\msys64\usr\bin\bash.exe -PassThru -Wait -NoNewWindow -ArgumentList "-c","'$args'"
If($output.Exitcode -ne 0)
{
     Throw "bash errored"
}