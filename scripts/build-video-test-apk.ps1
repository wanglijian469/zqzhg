param([string]$ApiBaseUrl='http://10.0.2.2:18086')
$ErrorActionPreference='Stop'
if($ApiBaseUrl -notmatch '^https?://(localhost|127\.0\.0\.1|10\.0\.2\.2)(:[0-9]+)?$'){throw 'Video test APK must use an isolated local service, never production.'}
$taskVideoRoot=Split-Path -Parent $PSScriptRoot
$taskProtected=@{}
foreach($taskRelative in @('delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk','mobile/pubspec.yaml')){
 $taskPath=Join-Path $taskVideoRoot $taskRelative
 if(Test-Path -LiteralPath $taskPath){$taskProtected[$taskPath]=(Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash}
}
& "$PSScriptRoot/prepare-android-workspace.ps1"
if($LASTEXITCODE -ne 0){throw 'Embedded web build failed.'}
. "$PSScriptRoot/flutter-env.ps1"
$taskBuildStarted=Get-Date
$taskSources=@{}
foreach($taskRelative in @('mobile/lib','mobile/android/app/src')){
 Get-ChildItem -LiteralPath (Join-Path $taskVideoRoot $taskRelative) -Recurse -File | ForEach-Object {$taskSources[$_.FullName]=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash}
}
$taskDefine=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("API_BASE_URL=$ApiBaseUrl"))
Push-Location (Join-Path $taskVideoRoot 'mobile/android')
try{
 & './gradlew.bat' --no-daemon assembleDebug '-Ptarget=lib/main.dart' '-Ptarget-platform=android-arm,android-arm64,android-x64' "-Pdart-defines=$taskDefine"
 if($LASTEXITCODE -ne 0){throw 'Video debug APK build failed.'}
}finally{Pop-Location}
$taskApk=Join-Path $taskVideoRoot 'mobile/build/app/outputs/apk/debug/app-debug.apk'
if((Get-Item -LiteralPath $taskApk).LastWriteTime -lt $taskBuildStarted){throw 'Refusing a stale test APK.'}
$taskBadging=& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/aapt.exe') dump badging $taskApk
if($LASTEXITCODE -ne 0 -or ($taskBadging -join "`n") -notmatch "package: name='com.zqzt.dalunongji.videodev'" -or ($taskBadging -join "`n") -notmatch '中企展馆视频测试'){throw 'Test package ID or label is incorrect.'}
& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/apksigner.bat') verify $taskApk
if($LASTEXITCODE -ne 0){throw 'Test APK signature verification failed.'}
foreach($taskSource in $taskSources.GetEnumerator()){if((Get-FileHash -LiteralPath $taskSource.Key -Algorithm SHA256).Hash -ne $taskSource.Value){throw 'App source changed during build. Rebuild the test APK.'}}
foreach($taskSource in $taskProtected.GetEnumerator()){if((Get-FileHash -LiteralPath $taskSource.Key -Algorithm SHA256).Hash -ne $taskSource.Value){throw 'A protected official artifact changed.'}}
$taskOutput=Join-Path $taskVideoRoot '.local/douyin-only/artifacts'
New-Item -ItemType Directory -Force -Path $taskOutput | Out-Null
$taskTarget=Join-Path $taskOutput 'zqzg-video-dev-debug.apk'
Copy-Item -LiteralPath $taskApk -Destination $taskTarget -Force
@{package='com.zqzt.dalunongji.videodev';label='中企展馆视频测试';apiBaseUrl=$ApiBaseUrl;debug=$true;sha256=(Get-FileHash -LiteralPath $taskTarget -Algorithm SHA256).Hash.ToLowerInvariant();bytes=(Get-Item -LiteralPath $taskTarget).Length;builtAt=(Get-Date).ToUniversalTime().ToString('o');deployed=$false;devicePlaybackValidated=$false;officialArtifactsUnchanged=$true}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $taskOutput 'apk-manifest.json') -Encoding utf8
Write-Output "Independent video test APK: $taskTarget"
