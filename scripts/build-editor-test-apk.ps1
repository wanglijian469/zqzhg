param([string]$ApiBaseUrl='http://172.27.35.2:18088')
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$taskFixturePath=Join-Path $taskRoot '.local/appflow/phone-fixture.json'
$taskFixture=Get-Content -LiteralPath $taskFixturePath -Raw | ConvertFrom-Json
if($taskFixture.database -notlike 'zqzt_exhibition_test_appflow_*' -or $taskFixture.api_base_url -ne $ApiBaseUrl){throw 'Test APK must use the matching isolated phone fixture.'}
$taskUri=[Uri]$ApiBaseUrl
if($taskUri.Scheme -ne 'http' -or $taskUri.Port -ne 18088 -or $taskUri.Host -notmatch '^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.|127\.0\.0\.1$)'){throw 'Only a private LAN test address on port 18088 is allowed.'}
$taskProtected=@{}
foreach($taskRelative in @('delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','mobile/pubspec.yaml')){
 $taskPath=Join-Path $taskRoot $taskRelative
 if(Test-Path -LiteralPath $taskPath){$taskProtected[$taskPath]=(Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash}
}
& "$PSScriptRoot/prepare-android-workspace.ps1"
. "$PSScriptRoot/flutter-env.ps1"
$taskStarted=Get-Date
$taskDefine=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("API_BASE_URL=$ApiBaseUrl"))
Push-Location (Join-Path $taskRoot 'mobile/android')
try{
 & './gradlew.bat' --no-daemon assembleEditdev '-Ptarget=lib/main.dart' '-Ptarget-platform=android-arm,android-arm64,android-x64' "-Pdart-defines=$taskDefine"
 if($LASTEXITCODE -ne 0){throw 'Phone test APK build failed.'}
}finally{Pop-Location}
$taskApk=Join-Path $taskRoot 'mobile/build/app/outputs/apk/editdev/app-editdev.apk'
if((Get-Item -LiteralPath $taskApk).LastWriteTime -lt $taskStarted){throw 'Stale test APK.'}
$taskBadging=& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/aapt.exe') dump badging $taskApk
if($LASTEXITCODE -ne 0 -or ($taskBadging -join "`n") -notmatch "package: name='com.zqzt.dalunongji.editdev'" -or ($taskBadging -join "`n") -notmatch '中企展馆发布测试'){throw 'Incorrect test package or label.'}
& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/apksigner.bat') verify $taskApk
if($LASTEXITCODE -ne 0){throw 'APK signature verification failed.'}
foreach($taskEntry in $taskProtected.GetEnumerator()){if((Get-FileHash -LiteralPath $taskEntry.Key -Algorithm SHA256).Hash -ne $taskEntry.Value){throw 'A protected official artifact changed.'}}
$taskOut=Join-Path $taskRoot '.local/appflow/artifacts'
New-Item -ItemType Directory -Force -Path $taskOut | Out-Null
$taskTarget=Join-Path $taskOut 'zqzg-editor-phone-test.apk'
Copy-Item -LiteralPath $taskApk -Destination $taskTarget -Force
@{package='com.zqzt.dalunongji.editdev';label='中企展馆发布测试';apiBaseUrl=$ApiBaseUrl;debug=$true;sha256=(Get-FileHash -LiteralPath $taskTarget -Algorithm SHA256).Hash.ToLowerInvariant();bytes=(Get-Item -LiteralPath $taskTarget).Length;builtAt=(Get-Date).ToUniversalTime().ToString('o');devicePlaybackValidated=$false;officialArtifactsUnchanged=$true}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $taskOut 'apk-manifest.json') -Encoding utf8
Write-Output "Phone test APK: $taskTarget"
