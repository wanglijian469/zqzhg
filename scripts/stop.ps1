$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$pidFile = Join-Path $taskRoot '.local/server.pid'
if (Test-Path -LiteralPath $pidFile) {
    $taskProcessID = [int](Get-Content -LiteralPath $pidFile)
    $taskProcess = Get-Process -Id $taskProcessID -ErrorAction SilentlyContinue
    $expectedExecutable = Join-Path $taskRoot 'server/exhibition.exe'
    if ($taskProcess -and [IO.Path]::GetFullPath($taskProcess.Path) -eq [IO.Path]::GetFullPath($expectedExecutable)) { Stop-Process -Id $taskProcessID }
}
