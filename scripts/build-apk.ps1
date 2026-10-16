param([string]$ApiBaseUrl = 'http://39.97.250.244:8089')
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
    $certificate = Get-ApkCertificate $apkSource
    if ($expectedCertificate -and $certificate -ne $expectedCertificate) { throw 'Signing certificate changed; refusing to replace downloads.' }
    $version = [regex]::Match((Get-Content (Join-Path $taskRoot 'mobile/pubspec.yaml') -Raw), '(?m)^version:\s*(\S+)').Groups[1].Value
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
