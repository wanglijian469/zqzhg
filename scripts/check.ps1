param([switch]$SkipMobile)
$ErrorActionPreference='Stop'
. "$PSScriptRoot/go-env.ps1"
$evidenceDirectory=Join-Path $taskRoot 'docs/test-output'
New-Item -ItemType Directory -Path $evidenceDirectory -Force | Out-Null
Push-Location (Join-Path $taskRoot 'server')
try {
    $env:RUN_MYSQL_TESTS='1'
    go test -v ./... 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'backend.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Backend tests failed.' }
    go vet ./... 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'go-vet.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Add-Content -LiteralPath (Join-Path $evidenceDirectory 'go-vet.txt') 'go vet ./...: exit 0'
} finally { Pop-Location }
Push-Location (Join-Path $taskRoot 'web')
try {
    npm test 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'web-test.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Web interaction tests failed.' }
    npm run build 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'web-build.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Web build failed.' }
} finally { Pop-Location }
if (-not $SkipMobile) {
    . "$PSScriptRoot/flutter-env.ps1"
    Push-Location (Join-Path $taskRoot 'mobile')
    try {
        & $taskFlutter analyze 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'flutter-analyze.txt')
        if ($LASTEXITCODE -ne 0) { throw 'Flutter analysis failed.' }
        & $taskFlutter test --concurrency=1 --reporter expanded 2>&1 | Tee-Object -FilePath (Join-Path $evidenceDirectory 'flutter-test.txt')
        if ($LASTEXITCODE -ne 0) { throw 'Flutter tests failed.' }
    } finally { Pop-Location }
}
