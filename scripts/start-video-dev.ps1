param([switch]$DisableDouyin,[switch]$Background)
$ErrorActionPreference='Stop'
$taskVideoRoot=Join-Path (Split-Path -Parent $PSScriptRoot) '.local/douyin-only'
$taskFixture=Get-Content -LiteralPath (Join-Path $taskVideoRoot 'full-fixture.json') -Raw | ConvertFrom-Json
if($taskFixture.database -notmatch '^zqzt_exhibition_test_douyin_full_[0-9]+$' -or -not $taskFixture.isolated_settings){throw 'Refusing to start outside the configured full-data snapshot.'}
if(-not (Test-Path -LiteralPath $taskFixture.media_directory)){throw 'Missing isolated media directory.'}
$taskExpectedMedia=[IO.Path]::GetFullPath((Join-Path $taskVideoRoot 'full/media'))
if(-not [IO.Path]::GetFullPath($taskFixture.media_directory).Equals($taskExpectedMedia,[StringComparison]::OrdinalIgnoreCase)){throw 'Refusing a media directory outside this isolated fixture.'}
$env:DB_NAME=$taskFixture.database
$env:DB_HOST='127.0.0.1';$env:DB_PORT='13306';$env:DB_USER='root';$env:DB_PASSWORD='root'
$env:APP_ENV='development';$env:HTTP_ADDR='127.0.0.1:18086'
$env:MEDIA_DIR=$taskFixture.media_directory;$env:WEB_DIR=Join-Path $taskVideoRoot 'web'
$env:PUBLIC_BASE_URL='';$env:CORS_ORIGIN='http://localhost:18086';$env:SMS_PROVIDER='development'
# The legacy SMS_ENABLED switch overrides SMS_PROVIDER. Remove it so tests use
# the original development provider and never send real SMS.
Remove-Item Env:SMS_ENABLED -ErrorAction SilentlyContinue
$env:DOUYIN_VIDEO_ENABLED=if($DisableDouyin){'false'}else{'true'}
$env:BACKGROUND_WORKERS_ENABLED='true'
$env:MAP_PREVIEW_ENABLED='true';$env:MAP_PREVIEW_DIR=$taskFixture.map_directory
$env:MAP_DEMO_ENABLED='false';$env:MAP_DEMO_DIR=Join-Path $taskVideoRoot 'full/map-demo'
$env:LOCAL_STORAGE_ROOTS_JSON='[]'
$env:OPERATIONS_DIR=Join-Path $taskVideoRoot '.operations-isolated-video'
if(Test-Path -LiteralPath (Join-Path $env:OPERATIONS_DIR 'active.json')){
    $taskActive=Get-Content -LiteralPath (Join-Path $env:OPERATIONS_DIR 'active.json') -Raw | ConvertFrom-Json
    if($taskActive.database -ne $taskFixture.database -or -not [IO.Path]::GetFullPath($taskActive.media_dir).Equals($taskExpectedMedia,[StringComparison]::OrdinalIgnoreCase)){throw 'Isolated active target does not match the fixture.'}
}
$env:FFMPEG_PATH=Join-Path (Split-Path -Parent $PSScriptRoot) '.tools/ffmpeg/bin/ffmpeg.exe'
$env:FFPROBE_PATH=Join-Path (Split-Path -Parent $PSScriptRoot) '.tools/ffmpeg/bin/ffprobe.exe'
if($Background){
 $taskStamp=Get-Date -Format 'yyyyMMdd-HHmmss'
 $taskProcess=Start-Process -FilePath (Join-Path $taskVideoRoot 'server.exe') -WorkingDirectory $taskVideoRoot -RedirectStandardOutput (Join-Path $taskVideoRoot "server-$taskStamp.out.log") -RedirectStandardError (Join-Path $taskVideoRoot "server-$taskStamp.err.log") -WindowStyle Hidden -PassThru
 Write-Output "Isolated Douyin preview PID: $($taskProcess.Id)"
 return
}
Push-Location $taskVideoRoot
try{& ./server.exe *> ('server-'+(Get-Date -Format 'yyyyMMdd-HHmmss')+'.log')}finally{Pop-Location}
