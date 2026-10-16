$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskTools = Join-Path $taskRoot '.tools'
foreach ($taskSubdirectory in @('tmp','profile/roaming','profile/local','pub-cache','gradle')) {
    New-Item -ItemType Directory -Path (Join-Path $taskTools $taskSubdirectory) -Force | Out-Null
}
$env:TEMP = Join-Path $taskTools 'tmp'
$env:TMP = $env:TEMP
$env:APPDATA = Join-Path $taskTools 'profile/roaming'
$env:LOCALAPPDATA = Join-Path $taskTools 'profile/local'
$env:PUB_CACHE = Join-Path $taskTools 'pub-cache'
$env:GRADLE_USER_HOME = Join-Path $taskTools 'gradle'
$env:FLUTTER_SUPPRESS_ANALYTICS = 'true'
$env:DART_SUPPRESS_ANALYTICS = 'true'
$env:NO_PROXY = 'localhost,127.0.0.1,::1'
$env:JAVA_TOOL_OPTIONS = "-Djava.io.tmpdir=$env:TEMP"
if (Test-Path -LiteralPath (Join-Path $taskTools 'android-sdk/platform-tools/adb.exe')) {
    $env:ANDROID_HOME = Join-Path $taskTools 'android-sdk'
} elseif (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = 'C:/Users/wang_ljian/AppData/Local/Android/Sdk' }
if (Test-Path -LiteralPath (Join-Path $taskTools 'android-ndk-r28c/source.properties')) { $env:ANDROID_NDK_HOME = Join-Path $taskTools 'android-ndk-r28c' }
if (Test-Path -LiteralPath 'D:/Program Files/Android/Android Studio/jbr') { $env:JAVA_HOME = 'D:/Program Files/Android/Android Studio/jbr' }
$taskFlutter = Join-Path $taskTools 'flutter/bin/flutter.bat'
if (-not (Test-Path -LiteralPath $taskFlutter)) { $taskFlutter = (Get-Command flutter).Source }
