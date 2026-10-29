param([string]$ApiBaseUrl = 'https://zqzhg.com', [switch]$Preview, [switch]$BuildOnly)
if ($ApiBaseUrl -cne 'https://zqzhg.com') { throw 'Release APK backend must be https://zqzhg.com; use Mock tests for other environments.' }
. "$PSScriptRoot/flutter-env.ps1"
$existingApk = Join-Path $taskRoot 'delivery/android/dalunongji.apk'
$apkSigner = Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/apksigner.bat'
function Get-ApkCertificate([string]$Path) {
    $result = & $apkSigner verify --print-certs $Path 2>&1
    if ($LASTEXITCODE -ne 0) { throw "APK signature verification failed: $Path" }
    $match = [regex]::Match(($result -join "`n"), 'Signer #1 certificate SHA-256 digest: ([a-f0-9]{64})')
    if (-not $match.Success) { throw 'APK signing certificate was not reported.' }
    return $match.Groups[1].Value
}
$expectedCertificate = if (Test-Path -LiteralPath $existingApk) { Get-ApkCertificate $existingApk } else { $null }
& node (Join-Path $PSScriptRoot 'generate-android-icons.mjs')
if ($LASTEXITCODE -ne 0) { throw 'Android icon generation failed.' }
& node (Join-Path $PSScriptRoot 'sync-form-rules.mjs')
if ($LASTEXITCODE -ne 0) { throw 'Form rule synchronization failed.' }
& node (Join-Path $PSScriptRoot 'sync-auth-policy.mjs')
if ($LASTEXITCODE -ne 0) { throw 'Authentication policy synchronization failed.' }
& "$PSScriptRoot/prepare-android-workspace.ps1"
if ($LASTEXITCODE -ne 0) { throw 'Embedded workspace preparation failed.' }
$signingDirectory = Join-Path $taskRoot '.local/signing'
New-Item -ItemType Directory -Path $signingDirectory -Force | Out-Null
$signingKey = Join-Path $signingDirectory 'dalunongji-release.jks'
$signingInfo = Join-Path $signingDirectory 'signing.json'
if (-not (Test-Path -LiteralPath $signingKey)) {
    if ($expectedCertificate) { throw 'Existing signed APK found. Restore its original signing key before rebuilding.' }
    if (Test-Path -LiteralPath $signingInfo) { throw 'Signing metadata exists without its key. Restore the key before rebuilding.' }
    $passwordBytes = [byte[]]::new(24)
    [Security.Cryptography.RandomNumberGenerator]::Fill($passwordBytes)
    $signingPassword = [Convert]::ToHexString($passwordBytes)
    @{ password=$signingPassword; alias='dalunongji'; keyFile=$signingKey } | ConvertTo-Json | Set-Content -LiteralPath $signingInfo -Encoding utf8
    & "$env:JAVA_HOME/bin/keytool.exe" -genkeypair -v -keystore $signingKey -storepass $signingPassword -keypass $signingPassword -alias dalunongji -keyalg RSA -keysize 2048 -validity 10000 -dname 'CN=ZQZT Dalu Nongji, O=ZQZT, C=CN'
    if ($LASTEXITCODE -ne 0) { throw 'Release key generation failed.' }
}
$signing = Get-Content -LiteralPath $signingInfo -Raw | ConvertFrom-Json
if ($expectedCertificate) {
    $env:ZQZT_APK_SIGN_PASSWORD = $signing.password
    try {
        $keyResult = & "$env:JAVA_HOME/bin/keytool.exe" -list -v -keystore $signingKey -storepass:env ZQZT_APK_SIGN_PASSWORD -alias $signing.alias 2>&1
        if ($LASTEXITCODE -ne 0) { throw 'Could not verify the original release key.' }
        $keyMatch = [regex]::Match(($keyResult -join "`n"), 'SHA256:\s*([A-Fa-f0-9:]{95})')
        if (-not $keyMatch.Success -or $keyMatch.Groups[1].Value.Replace(':','').ToLowerInvariant() -ne $expectedCertificate) { throw 'Original APK and signing key certificate differ; build stopped.' }
    } finally { Remove-Item Env:ZQZT_APK_SIGN_PASSWORD -ErrorAction SilentlyContinue }
}
$keyProperties = "storePassword=$($signing.password)`nkeyPassword=$($signing.password)`nkeyAlias=$($signing.alias)`nstoreFile=$($signingKey.Replace('\','/'))`n"
[IO.File]::WriteAllText((Join-Path $taskRoot 'mobile/android/key.properties'), $keyProperties)
Push-Location (Join-Path $taskRoot 'mobile')
try {
    $taskBuildSources = @{}
    foreach ($taskSourceDirectory in @('lib','assets','android/app/src')) {
        Get-ChildItem -LiteralPath $taskSourceDirectory -Recurse -File | ForEach-Object { $taskBuildSources[$_.FullName] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
    }
    foreach ($taskSourceFile in @('pubspec.yaml','pubspec.lock','android/app/build.gradle.kts')) {
        $taskResolvedSource = (Resolve-Path -LiteralPath $taskSourceFile).Path
        $taskBuildSources[$taskResolvedSource] = (Get-FileHash -LiteralPath $taskResolvedSource -Algorithm SHA256).Hash
    }
    $buildStarted = Get-Date
    & $taskFlutter build apk --release --no-pub "--dart-define=API_BASE_URL=$ApiBaseUrl"
    if ($LASTEXITCODE -ne 0) {
        Write-Output 'Retrying with a fresh Gradle process.'
        Push-Location android
        try {
            & './gradlew.bat' --stop
            $apiDefine = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("API_BASE_URL=$ApiBaseUrl"))
            & './gradlew.bat' --no-daemon assembleRelease '-Ptarget=lib/main.dart' '-Ptarget-platform=android-arm,android-arm64,android-x64' "-Pdart-defines=$apiDefine"
            if ($LASTEXITCODE -ne 0) { throw 'APK build failed.' }
        } finally { Pop-Location }
    }
    $apkSource = @('mobile/build/app/outputs/flutter-apk/app-release.apk','mobile/build/app/outputs/apk/release/app-release.apk') |
        ForEach-Object { Get-Item -LiteralPath (Join-Path $taskRoot $_) -ErrorAction SilentlyContinue } |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1 -ExpandProperty FullName
    if (-not $apkSource -or (Get-Item -LiteralPath $apkSource).LastWriteTime -lt $buildStarted) { throw 'No newly built APK was found; refusing to publish an old artifact.' }
    foreach ($taskSource in $taskBuildSources.GetEnumerator()) {
        if ((Get-FileHash -LiteralPath $taskSource.Key -Algorithm SHA256).Hash -ne $taskSource.Value) { throw 'App sources changed while building. Rebuild before publishing the acceptance APK.' }
    }
    foreach ($taskSourceDirectory in @('lib','assets','android/app/src')) {
        Get-ChildItem -LiteralPath $taskSourceDirectory -Recurse -File | ForEach-Object { if (-not $taskBuildSources.ContainsKey($_.FullName)) { throw 'App sources were added while building; rebuild required.' } }
    }
    $certificate = Get-ApkCertificate $apkSource
    if ($expectedCertificate -and $certificate -ne $expectedCertificate) { throw 'Signing certificate changed; refusing to replace downloads.' }
    $version = [regex]::Match((Get-Content (Join-Path $taskRoot 'mobile/pubspec.yaml') -Raw), '(?m)^version:\s*(\S+)').Groups[1].Value
    if ($version.Contains('-') -and -not $Preview) { throw 'Prerelease APKs require -Preview; the stable download must not be replaced.' }
    $taskVersionParts = $version.Split('+')
    if ($taskVersionParts.Length -ne 2) { throw 'pubspec version must include a build number.' }
    $taskBadging = & (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/aapt.exe') dump badging $apkSource 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'APK metadata verification failed.' }
    $taskMetadata = $taskBadging -join "`n"
    if (-not $taskMetadata.Contains("name='com.zqzt.dalunongji'") -or -not $taskMetadata.Contains("versionName='$($taskVersionParts[0])'") -or -not $taskMetadata.Contains("versionCode='$($taskVersionParts[1])'")) { throw 'APK package or version differs from pubspec.' }
    if ($BuildOnly) {
        $output = Join-Path $taskRoot "delivery/android-v$($taskVersionParts[0])"
        New-Item -ItemType Directory -Force -Path $output | Out-Null
        $artifact = Join-Path $output "大陆农机-$version.apk"
        Copy-Item -LiteralPath $apkSource -Destination $artifact -Force
        @{version=$version;applicationId='com.zqzt.dalunongji';appName='大陆农机';sha256=(Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant();certificateSHA256=$certificate;signingMatchesPrevious=($certificate -eq $expectedCertificate);apiBaseUrl=$ApiBaseUrl;productionDeployed=$false;deviceValidation=$false} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $output 'manifest.json') -Encoding utf8
        Write-Output "Installable APK created: $artifact; existing downloads retained."
        return
    }
    if ($Preview) {
        $previewDirectories = @('delivery/android-preview','web/public/downloads/previews','web/dist/downloads/previews')
        foreach ($previewOutput in $previewDirectories) {
            $previewDirectory = Join-Path $taskRoot $previewOutput
            New-Item -ItemType Directory -Path $previewDirectory -Force | Out-Null
            Copy-Item -LiteralPath $apkSource -Destination (Join-Path $previewDirectory "dalunongji-$version.apk") -Force
        }
        $stableHash = (Get-FileHash -LiteralPath $existingApk -Algorithm SHA256).Hash.ToLowerInvariant()
        $receipt = @{version=$version;sha256=(Get-FileHash -LiteralPath $apkSource -Algorithm SHA256).Hash.ToLowerInvariant();certificateSHA256=$certificate;signingMatchesPrevious=($certificate -eq $expectedCertificate);stableSHA256=$stableHash;apiBaseUrl=$ApiBaseUrl;productionDeployed=$false;deviceValidation=$false}
        $receipt | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $taskRoot '.local/android-preview-verification.json') -Encoding utf8
        Write-Output "Acceptance APK $version built separately; stable APK retained; certificate SHA-256: $certificate"
        return
    }
    if ($expectedCertificate) {
        $oldHash = (Get-FileHash $existingApk -Algorithm SHA256).Hash.ToLowerInvariant()
        $archive = Join-Path $taskRoot 'delivery/android/archive'
        New-Item -ItemType Directory -Path $archive -Force | Out-Null
        $oldArchive = Join-Path $archive ("dalunongji-$oldHash.apk")
        if (-not (Test-Path -LiteralPath $oldArchive)) { Copy-Item -LiteralPath $existingApk -Destination $oldArchive }
    }
    Copy-Item -LiteralPath $apkSource -Destination (Join-Path $taskRoot "delivery/android/dalunongji-$version.apk") -Force
    foreach ($apkOutput in @('delivery/android','web/public/downloads','web/dist/downloads','delivery/web/dist/downloads')) {
        $apkDirectory = Join-Path $taskRoot $apkOutput
        New-Item -ItemType Directory -Path $apkDirectory -Force | Out-Null
        Copy-Item -LiteralPath $apkSource -Destination (Join-Path $apkDirectory 'dalunongji.apk') -Force
    }
    Get-FileHash -LiteralPath $apkSource -Algorithm SHA256 | Format-List
    Write-Output "Release $version; signing certificate SHA-256: $certificate; API: $ApiBaseUrl"
} finally { Pop-Location }
