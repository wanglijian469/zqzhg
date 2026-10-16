$taskRoot = Split-Path -Parent $PSScriptRoot
foreach ($taskGoDirectory in @('.tools/go-cache','.tools/tmp')) {
    New-Item -ItemType Directory -Path (Join-Path $taskRoot $taskGoDirectory) -Force | Out-Null
}
$env:GOCACHE = Join-Path $taskRoot '.tools/go-cache'
$env:GOTMPDIR = Join-Path $taskRoot '.tools/tmp'
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:GOTMPDIR
