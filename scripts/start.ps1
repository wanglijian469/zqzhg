param([switch]$MapPreview)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/go-env.ps1"
$taskRoot = Split-Path -Parent $PSScriptRoot
$serverDirectory = Join-Path $taskRoot 'server'
$localDirectory = Join-Path $taskRoot '.local'
New-Item -ItemType Directory -Path $localDirectory -Force | Out-Null
if (-not (Test-Path -LiteralPath (Join-Path $serverDirectory '.env'))) { throw 'Create server/.env from server/.env.example first.' }
Push-Location $serverDirectory
try { go build -o exhibition.exe .; if ($LASTEXITCODE -ne 0) { throw 'Backend build failed.' } } finally { Pop-Location }
$previousPreviewFlag=$env:MAP_PREVIEW_ENABLED
try {
 if ($MapPreview) { $env:MAP_PREVIEW_ENABLED='true' }
 $taskProcess = Start-Process -FilePath (Join-Path $serverDirectory 'exhibition.exe') -WorkingDirectory $serverDirectory -WindowStyle Hidden -RedirectStandardOutput (Join-Path $localDirectory 'server.stdout.log') -RedirectStandardError (Join-Path $localDirectory 'server.stderr.log') -PassThru
} finally { $env:MAP_PREVIEW_ENABLED=$previousPreviewFlag }
$taskProcess.Id | Set-Content -LiteralPath (Join-Path $localDirectory 'server.pid')
Write-Output ('Server process: {0}; website: http://localhost:8080; administration: http://localhost:8080/admin' -f $taskProcess.Id)
