$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$demoRoot=Join-Path $taskRoot '.local/procurement-demo'
$demoPidFile=Join-Path $demoRoot 'server.pid'
if (Test-Path -LiteralPath $demoPidFile) {
 $demoProcessID=[int](Get-Content -LiteralPath $demoPidFile)
 $demoProcess=Get-Process -Id $demoProcessID -ErrorAction SilentlyContinue
 $demoExecutable=[IO.Path]::GetFullPath((Join-Path $demoRoot 'exhibition.exe'))
 if ($demoProcess) {
  if ([IO.Path]::GetFullPath($demoProcess.Path) -ne $demoExecutable) { throw 'Demo PID belongs to another executable; no process stopped.' }
  Stop-Process -Id $demoProcessID
  $demoProcess.WaitForExit(10000) | Out-Null
 }
 Remove-Item -LiteralPath $demoPidFile
}
Write-Output 'Procurement demo stopped; primary service unchanged.'
