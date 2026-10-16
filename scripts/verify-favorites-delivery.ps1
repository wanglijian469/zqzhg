$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskRoot 'delivery/manifest-favorites-1.5.16.json') -Raw | ConvertFrom-Json
foreach ($taskArtifact in $taskManifest.files) {
    $taskPath = Join-Path $taskRoot $taskArtifact.file
    if ((Get-Item -LiteralPath $taskPath).Length -ne $taskArtifact.bytes) { throw "Wrong artifact size: $($taskArtifact.file)" }
    if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskArtifact.sha256) { throw "Artifact checksum mismatch: $($taskArtifact.file)" }
}
foreach ($taskApkCopy in $taskManifest.apkCopies) {
    if ((Get-FileHash -LiteralPath (Join-Path $taskRoot $taskApkCopy) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskManifest.apkSHA256) { throw "APK copy mismatch: $taskApkCopy" }
}
foreach ($taskOldVersion in @('1.5.12+20','1.5.13+21','1.5.14+22','1.5.15+23')) {
    if (-not (Test-Path -LiteralPath (Join-Path $taskRoot "delivery/android/dalunongji-$taskOldVersion.apk"))) { throw "Previous APK missing: $taskOldVersion" }
}
Add-Type -AssemblyName System.IO.Compression.FileSystem
$taskArchive = [IO.Compression.ZipFile]::OpenRead((Join-Path $taskRoot 'delivery/zqzt-favorites-v1.5.16-backend.zip'))
$taskChecked = 0
try {
    $taskChecksumEntry = $taskArchive.GetEntry('SHA256SUMS.txt')
    if (-not $taskChecksumEntry) { throw 'Backend package checksum list missing' }
    $taskReader = [IO.StreamReader]::new($taskChecksumEntry.Open())
    try { $taskChecksumLines = $taskReader.ReadToEnd() -split "`n" } finally { $taskReader.Dispose() }
    foreach ($taskChecksumLine in $taskChecksumLines) {
        if ([string]::IsNullOrWhiteSpace($taskChecksumLine)) { continue }
        if ($taskChecksumLine -notmatch '^([0-9a-f]{64})  (.+)$') { throw 'Invalid checksum line' }
        $taskExpected = $Matches[1]
        $taskEntryName = $Matches[2].TrimEnd("`r")
        $taskEntry = $taskArchive.GetEntry($taskEntryName)
        if (-not $taskEntry) { throw "Missing archive entry: $taskEntryName" }
        $taskStream = $taskEntry.Open()
        $taskHasher = [Security.Cryptography.SHA256]::Create()
        try { $taskActual = [Convert]::ToHexString($taskHasher.ComputeHash($taskStream)).ToLowerInvariant() }
        finally { $taskStream.Dispose(); $taskHasher.Dispose() }
        if ($taskActual -ne $taskExpected) { throw "Archive checksum mismatch: $taskEntryName" }
        $taskChecked++
    }
    foreach ($taskEntry in $taskArchive.Entries) {
        if ($taskEntry.FullName -match '(^|/)(\.env|key\.properties|signing\.json)$|\.(jks|keystore)$') { throw 'Private configuration or signing material included' }
    }
    if ($taskChecked -ne $taskArchive.Entries.Count - 1) { throw 'Archive has unlisted entries' }
} finally { $taskArchive.Dispose() }
$taskResult = [ordered]@{
    passed = $true
    androidVersion = $taskManifest.androidVersion
    backendVersion = $taskManifest.backendVersion
    artifactHashesChecked = $taskManifest.files.Count
    apkCopiesChecked = $taskManifest.apkCopies.Count
    archiveEntriesChecked = $taskChecked
    previousApksPreserved = $true
    forbiddenFilesIncluded = @()
    remoteDeployment = $false
}
$taskResult | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $taskRoot 'docs/test-output/favorites-package-verification.json') -Encoding utf8
$taskResult | ConvertTo-Json -Depth 5
