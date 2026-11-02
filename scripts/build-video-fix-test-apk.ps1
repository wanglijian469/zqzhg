$ErrorActionPreference='Stop'
$taskProjectRoot=Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/flutter-env.ps1"
$taskProtected=@{}
foreach($taskRelative in @('delivery/android/dalunongji.apk','web/public/downloads/dalunongji.apk','web/dist/downloads/dalunongji.apk')){
 $taskPath=Join-Path $taskProjectRoot $taskRelative
 if(Test-Path -LiteralPath $taskPath){$taskProtected[$taskPath]=(Get-FileHash -LiteralPath $taskPath).Hash}
}
& "$PSScriptRoot/test-official-video-policy.ps1"
& "$PSScriptRoot/prepare-android-workspace.ps1"
if($LASTEXITCODE -ne 0){throw 'Workspace preparation failed.'}
# A direct Gradle build does not refresh the version values normally written by Flutter.
$taskLocalProperties=Join-Path $taskProjectRoot 'mobile/android/local.properties'
$taskLocalText=Get-Content -LiteralPath $taskLocalProperties -Raw
$taskLocalText=[regex]::Replace($taskLocalText,'(?m)^flutter.versionName=.*$','flutter.versionName=1.10.9')
$taskLocalText=[regex]::Replace($taskLocalText,'(?m)^flutter.versionCode=.*$','flutter.versionCode=47')
[IO.File]::WriteAllText($taskLocalProperties,$taskLocalText)
$taskDefine=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('API_BASE_URL=https://zqzhg.com'))
$taskStarted=Get-Date
Push-Location (Join-Path $taskProjectRoot 'mobile/android')
try{
 & './gradlew.bat' --no-daemon assembleEditdev '-Ptarget=lib/main.dart' '-Ptarget-platform=android-arm,android-arm64,android-x64' "-Pdart-defines=$taskDefine"
 if($LASTEXITCODE -ne 0){throw 'Video fix test APK build failed.'}
}finally{Pop-Location}
$taskApk=Join-Path $taskProjectRoot 'mobile/build/app/outputs/apk/editdev/app-editdev.apk'
if((Get-Item -LiteralPath $taskApk).LastWriteTime -lt $taskStarted){throw 'Stale test APK.'}
$taskMetadata=(& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/aapt.exe') dump badging $taskApk) -join "`n"
if($LASTEXITCODE -ne 0 -or $taskMetadata -notmatch "name='com.zqzt.dalunongji.editdev'" -or $taskMetadata -notmatch "versionCode='47'" -or $taskMetadata -notmatch "versionName='1.10.9'"){throw 'Incorrect test identity/version.'}
& (Join-Path $env:ANDROID_HOME 'build-tools/36.0.0/apksigner.bat') verify $taskApk
if($LASTEXITCODE -ne 0){throw 'Test signature verification failed.'}
foreach($taskEntry in $taskProtected.GetEnumerator()){if((Get-FileHash -LiteralPath $taskEntry.Key).Hash -ne $taskEntry.Value){throw 'Stable artifact changed.'}}
$taskOut=Join-Path $taskProjectRoot 'delivery/video-fix-test'
New-Item -ItemType Directory -Force -Path $taskOut | Out-Null
$taskName='zqzhg-video-fix-1.10.9-47.apk'
$taskTarget=Join-Path $taskOut $taskName
Copy-Item -LiteralPath $taskApk -Destination $taskTarget -Force
@{version='1.10.9+47';versionCode=47;versionName='1.10.9';applicationId='com.zqzt.dalunongji.editdev';appName='中企展馆发布测试';apiBaseUrl='https://zqzhg.com';asset=$taskName;bytes=(Get-Item -LiteralPath $taskTarget).Length;sha256=(Get-FileHash -LiteralPath $taskTarget).Hash.ToLowerInvariant();debug=$true;independentPackage=$true;deviceValidation=$false;stableDownloadUpdated=$false;schemaVersion=30;candidateDownloadURL="https://zqzhg.com/downloads/candidates/$taskName"}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $taskOut 'manifest.json') -Encoding utf8
Write-Output "Independent video test APK: $taskTarget"
