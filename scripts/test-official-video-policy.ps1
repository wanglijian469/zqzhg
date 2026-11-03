$ErrorActionPreference='Stop'
. "$PSScriptRoot/flutter-env.ps1"
$taskClasses=Join-Path $taskRoot '.local/video-policy-classes'
New-Item -ItemType Directory -Force -Path $taskClasses | Out-Null
& "$env:JAVA_HOME/bin/javac.exe" -encoding UTF-8 -d $taskClasses (Join-Path $taskRoot 'mobile/android/app/src/main/java/com/zqzt/dalunongji/OfficialVideoPolicy.java') (Join-Path $taskRoot 'mobile/android/app/src/main/java/com/zqzt/dalunongji/OfficialVideoLoadState.java') (Join-Path $PSScriptRoot 'tests/OfficialVideoPolicyCheck.java')
if($LASTEXITCODE -ne 0){throw 'Player policy compilation failed.'}
& "$env:JAVA_HOME/bin/java.exe" -cp $taskClasses com.zqzt.dalunongji.OfficialVideoPolicyCheck
if($LASTEXITCODE -ne 0){throw 'Player policy checks failed.'}
