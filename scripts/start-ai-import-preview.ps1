param([switch]$Build, [switch]$Stop, [ValidateRange(1024,65535)][int]$Port=8094)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path $PSScriptRoot -Parent
$candidatePath=Join-Path $taskRoot '.local/ai-import-server-final.exe'
$running=@(Get-Process | Where-Object { $_.Path -eq $candidatePath })
if($Stop) { $running | Stop-Process; Write-Output 'Local AI preview stopped.'; return }
if($running.Count -gt 0 -and !$Build) { Write-Output "Local AI preview already running: http://127.0.0.1:$Port/merchant/ai-import"; return }
# This helper is only for the local development database, never a remote server.
$privateEnv=Join-Path $taskRoot 'server/.env'
function Read-PreviewSetting([string]$Name,[string]$Default) {
 $current=[Environment]::GetEnvironmentVariable($Name)
 if($current) { return $current }
 if(Test-Path -LiteralPath $privateEnv) {
  foreach($line in [IO.File]::ReadAllLines($privateEnv)) {
   if($line -match ('^\s*'+[regex]::Escape($Name)+'\s*=\s*(.*?)\s*$')) { return $Matches[1].Trim('"',"'") }
  }
 }
 return $Default
}
$dbHost=Read-PreviewSetting 'DB_HOST' '127.0.0.1'
$dbName=Read-PreviewSetting 'DB_NAME' 'zqzt_exhibition'
if($dbHost -notin @('127.0.0.1','localhost') -or $dbName -ne 'zqzt_exhibition') { throw 'Preview requires the existing loopback development database zqzt_exhibition.' }
if($running.Count -gt 0) { $running | Stop-Process }
Push-Location $taskRoot
try {
 . (Join-Path $PSScriptRoot 'go-env.ps1')
 if($Build) {
  node web/node_modules/typescript/bin/tsc -b web
  if($LASTEXITCODE -ne 0) { throw 'TypeScript check failed.' }
  Push-Location web
  try { node node_modules/vite/bin/vite.js build --outDir ../.local/ai-import-web; if($LASTEXITCODE -ne 0) { throw 'Web build failed.' } } finally { Pop-Location }
  Push-Location server
  try { go build -o $candidatePath .; if($LASTEXITCODE -ne 0) { throw 'Backend build failed.' } } finally { Pop-Location }
 }
 if(!(Test-Path -LiteralPath $candidatePath)) { throw 'Run this helper with -Build first.' }
 $outputDir=Join-Path $taskRoot '.local/ai-import-inspection'
 New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
 $env:APP_ENV='development'
 $env:HTTP_ADDR="127.0.0.1:$Port"
 $env:WEB_DIR=Join-Path $taskRoot '.local/ai-import-web'
 $env:BACKGROUND_WORKERS_ENABLED='false'
 $env:AI_IMPORT_ENABLED='true'
 $env:AI_IMPORT_WORKER_ENABLED='true'
 $env:LOCAL_PREVIEW_SAME_ORIGIN='true'
 $env:SMS_PROVIDER='development'
 $process=Start-Process -FilePath $candidatePath -WorkingDirectory (Join-Path $taskRoot 'server') -WindowStyle Hidden -RedirectStandardOutput (Join-Path $outputDir 'server-final.stdout.log') -RedirectStandardError (Join-Path $outputDir 'server-final.stderr.log') -PassThru
 Write-Output "Local AI preview PID $($process.Id): http://127.0.0.1:$Port/merchant/ai-import"
} finally { Pop-Location }
