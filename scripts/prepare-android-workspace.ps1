$ErrorActionPreference = 'Stop'
$workspaceRoot = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/go-env.ps1"
$webOutput = Join-Path $workspaceRoot '.local/android-web-dist'
$expectedWebOutput = [IO.Path]::GetFullPath((Join-Path $workspaceRoot '.local/android-web-dist'))
if ([IO.Path]::GetFullPath($webOutput) -ne $expectedWebOutput) { throw 'Invalid generated Web output directory.' }
Push-Location (Join-Path $workspaceRoot 'web')
try {
    & node node_modules/typescript/bin/tsc --noEmit
    if ($LASTEXITCODE -ne 0) { throw 'Web type check failed.' }
    & node node_modules/vite/bin/vite.js build --outDir $webOutput --manifest --emptyOutDir
    if ($LASTEXITCODE -ne 0) { throw 'Embedded workspace build failed.' }
} finally { Pop-Location }
$assetRoot = [IO.Path]::GetFullPath((Join-Path $workspaceRoot 'mobile/android/app/src/main/assets/workspace'))
$expectedRoot = [IO.Path]::GetFullPath((Join-Path $workspaceRoot 'mobile/android/app/src/main/assets')) + [IO.Path]::DirectorySeparatorChar
if (-not $assetRoot.StartsWith($expectedRoot,[StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid generated asset directory.' }
if (Test-Path -LiteralPath $assetRoot) {
    if ((Get-Item -LiteralPath $assetRoot).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Generated assets must not be a link.' }
    if (-not (Test-Path -LiteralPath (Join-Path $assetRoot 'bundle-manifest.json'))) { throw 'Refusing to replace unrecognized Android assets.' }
    Remove-Item -LiteralPath $assetRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $assetRoot | Out-Null
# Explicit allowlist: never include downloads/APKs, secrets or databases.
foreach ($entry in @('index.html','brand.svg','assets','pdfjs','maps')) {
    $source = Join-Path $webOutput $entry
    if (Test-Path -LiteralPath $source) { Copy-Item -LiteralPath $source -Destination $assetRoot -Recurse }
}
# Android normalizes .gz asset names to their uncompressed counterpart. The
# embedded server reads plain files; HTTP precompressed variants belong on Web only.
Get-ChildItem -LiteralPath $assetRoot -Recurse -File -Filter '*.gz' | ForEach-Object { Remove-Item -LiteralPath $_.FullName }
$files = @(Get-ChildItem -LiteralPath $assetRoot -Recurse -File | ForEach-Object {
    @{ path=[IO.Path]::GetRelativePath($assetRoot,$_.FullName).Replace('\','/'); bytes=$_.Length; sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
})
if ($files | Where-Object { $_.path -match '\.(apk|jks|keystore|env)$' }) { throw 'Forbidden generated asset.' }
@{version=1;createdAt=(Get-Date).ToUniversalTime().ToString('o');files=$files} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $assetRoot 'bundle-manifest.json') -Encoding utf8
Write-Output "Bundled $($files.Count) Web assets; main website outputs remain unchanged."
