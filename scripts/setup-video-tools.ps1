param([switch]$Download)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$taskVideoRoot=Join-Path $taskRoot '.tools/ffmpeg'
$taskVideoExe=Join-Path $taskVideoRoot 'bin/ffmpeg.exe'
if (-not (Test-Path -LiteralPath $taskVideoExe)) {
    if (-not $Download) { throw 'FFmpeg is missing. Run scripts/setup-video-tools.ps1 -Download or configure FFMPEG_PATH and FFPROBE_PATH.' }
    New-Item -ItemType Directory -Path $taskVideoRoot -Force | Out-Null
    $taskArchive=Join-Path $taskVideoRoot 'essentials.zip'
    Invoke-WebRequest -Uri 'https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip' -OutFile $taskArchive
    $taskUnpacked=Join-Path $taskVideoRoot 'unpacked'
    Expand-Archive -LiteralPath $taskArchive -DestinationPath $taskUnpacked -Force
    $taskBuild=Get-ChildItem -LiteralPath $taskUnpacked -Directory | Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'bin/ffmpeg.exe') } | Select-Object -First 1
    if (-not $taskBuild) { throw 'The archive does not contain FFmpeg.' }
    Copy-Item -LiteralPath (Join-Path $taskBuild.FullName 'bin') -Destination (Join-Path $taskVideoRoot 'bin') -Recurse -Force
    Copy-Item -LiteralPath (Join-Path $taskBuild.FullName 'LICENSE') -Destination (Join-Path $taskVideoRoot 'LICENSE') -Force
}
$taskProbe=Join-Path $taskVideoRoot 'bin/ffprobe.exe'
if (-not (Test-Path -LiteralPath $taskProbe)) { throw 'ffprobe.exe is missing.' }
$taskVideoVersion = & $taskVideoExe -version
$taskVideoResult = $LASTEXITCODE
$taskProbeVersion = & $taskProbe -version
$taskProbeResult = $LASTEXITCODE
if ($taskVideoResult -ne 0 -or $taskProbeResult -ne 0) { throw 'Video tool verification failed.' }
Write-Output $taskVideoVersion[0]
Write-Output $taskProbeVersion[0]
Write-Output ('FFMPEG_PATH='+$taskVideoExe)
Write-Output ('FFPROBE_PATH='+$taskProbe)
