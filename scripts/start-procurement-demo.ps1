param([switch]$Reset)
$ErrorActionPreference='Stop'
. "$PSScriptRoot/go-env.ps1"
$taskRoot=Split-Path -Parent $PSScriptRoot
$demoRoot=[IO.Path]::GetFullPath((Join-Path $taskRoot '.local/procurement-demo'))
if (-not $demoRoot.StartsWith([IO.Path]::GetFullPath($taskRoot)+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected demo path' }
New-Item -ItemType Directory -Path $demoRoot -Force | Out-Null
if ($Reset) { & "$PSScriptRoot/stop-procurement-demo.ps1" }
$demoPidFile=Join-Path $demoRoot 'server.pid'
if (Test-Path -LiteralPath $demoPidFile) {
 $demoProcess=Get-Process -Id ([int](Get-Content -LiteralPath $demoPidFile)) -ErrorAction SilentlyContinue
 if ($demoProcess -and [IO.Path]::GetFullPath($demoProcess.Path) -eq (Join-Path $demoRoot 'exhibition.exe')) { Write-Output 'Demo already running: http://localhost:8081/buyer'; return }
}
if (Get-NetTCPConnection -State Listen -LocalPort 8081 -ErrorAction SilentlyContinue) { throw 'Port 8081 is occupied; no process changed.' }
if (-not (Test-Path -LiteralPath (Join-Path $taskRoot 'web/dist/index.html'))) { throw 'Build web/dist before starting the demo.' }
Push-Location (Join-Path $taskRoot 'server')
try { go build -o (Join-Path $demoRoot 'exhibition.exe') .; if ($LASTEXITCODE -ne 0) { throw 'Demo backend build failed' } } finally { Pop-Location }
$demoSecretFile=Join-Path $demoRoot 'local-secrets.json'
if (-not (Test-Path -LiteralPath $demoSecretFile)) {
 $demoBytes=New-Object byte[] 32
 $demoRandom=[Security.Cryptography.RandomNumberGenerator]::Create()
 try { $demoRandom.GetBytes($demoBytes) } finally { $demoRandom.Dispose() }
 @{admin_password=('Demo-'+[guid]::NewGuid().ToString('N'));encryption_key=[Convert]::ToBase64String($demoBytes)} | ConvertTo-Json | Set-Content -LiteralPath $demoSecretFile -Encoding utf8
}
$demoSecrets=Get-Content -LiteralPath $demoSecretFile -Raw | ConvertFrom-Json
@("URL: http://localhost:8081/admin/procurement/reviews",'Username: demo-admin',("Password: "+$demoSecrets.admin_password),'Local isolated demo only. Do not use this password for another environment.') | Set-Content -LiteralPath (Join-Path $demoRoot 'admin-login.txt') -Encoding utf8
$demoEnvironment=@{
 APP_ENV='development';PROCUREMENT_DEMO='true';PROCUREMENT_ENABLED='true';SMS_PROVIDER='development';HTTP_ADDR='127.0.0.1:8081';DB_HOST='127.0.0.1';DB_PORT='13306';DB_NAME='zqzt_procurement_demo';
 WEB_DIR=(Join-Path $taskRoot 'web/dist');MEDIA_DIR=(Join-Path $demoRoot 'media');MEDIA_TEMP_DIR=(Join-Path $demoRoot 'temp');OPERATIONS_DIR=(Join-Path $demoRoot 'operations');BACKUP_DIR=(Join-Path $demoRoot 'backups');
 PUBLIC_BASE_URL='http://localhost:8081';CORS_ORIGIN='http://localhost:8081';MAP_PREVIEW_ENABLED='false';LOCAL_STORAGE_ROOTS_JSON='[]';ADMIN_USERNAME='demo-admin';ADMIN_PASSWORD=$demoSecrets.admin_password;CONFIG_ENCRYPTION_KEY=$demoSecrets.encryption_key
}
$demoPrior=@{}
try {
 foreach ($demoName in $demoEnvironment.Keys) { $demoPrior[$demoName]=[Environment]::GetEnvironmentVariable($demoName,'Process');[Environment]::SetEnvironmentVariable($demoName,$demoEnvironment[$demoName],'Process') }
 $demoExecutable=Join-Path $demoRoot 'exhibition.exe'
 Push-Location (Join-Path $taskRoot 'server')
 try {
  if ($Reset) {
   & $demoExecutable reset-procurement-demo
   if ($LASTEXITCODE -ne 0) { throw 'Demo database guard/reset failed; files were not removed.' }
   foreach ($demoLeaf in @('media','temp','operations','backups')) {
    $demoTarget=[IO.Path]::GetFullPath((Join-Path $demoRoot $demoLeaf))
    if (-not $demoTarget.StartsWith($demoRoot+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected reset path' }
    if (Test-Path -LiteralPath $demoTarget) { Remove-Item -LiteralPath $demoTarget -Recurse -Force }
   }
  }
  foreach ($demoDirectory in @('media','temp','operations','backups')) { New-Item -ItemType Directory -Path (Join-Path $demoRoot $demoDirectory) -Force | Out-Null }
  & $demoExecutable seed-procurement-demo
  if ($LASTEXITCODE -ne 0) { throw 'Demo seed failed' }
  $demoProcess=Start-Process -FilePath $demoExecutable -WorkingDirectory (Join-Path $taskRoot 'server') -WindowStyle Hidden -RedirectStandardOutput (Join-Path $demoRoot 'server.stdout.log') -RedirectStandardError (Join-Path $demoRoot 'server.stderr.log') -PassThru
  $demoProcess.Id | Set-Content -LiteralPath $demoPidFile
 } finally { Pop-Location }
} finally { foreach ($demoName in $demoPrior.Keys) { [Environment]::SetEnvironmentVariable($demoName,$demoPrior[$demoName],'Process') } }
for ($demoAttempt=0;$demoAttempt -lt 75;$demoAttempt++) {
 try { $demoHealth=Invoke-RestMethod 'http://127.0.0.1:8081/api/v1/health' -TimeoutSec 2;if ($demoHealth.status -eq 'ok') { Write-Output 'Procurement demo: http://localhost:8081/buyer; select an account and request the dynamic code. No real SMS is sent.';return } } catch { }
 if ($demoProcess.HasExited) { throw 'Demo process exited; inspect .local/procurement-demo/server.stderr.log' }
 Start-Sleep -Milliseconds 200
}
throw 'Demo did not become healthy; inspect its isolated logs.'
