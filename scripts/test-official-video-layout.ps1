$ErrorActionPreference='Stop'
$taskProject=Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/flutter-env.ps1"
Push-Location $taskProject
try {
    & "$PSScriptRoot/test-official-video-policy.ps1"
    if($LASTEXITCODE -ne 0){throw 'Official player native checks failed.'}
    & "$env:JAVA_HOME/bin/javac.exe" -encoding UTF-8 -cp .local/video-policy-classes -d .local/video-policy-classes scripts/tests/OfficialVideoDocumentProbe.java
    if($LASTEXITCODE -ne 0){throw 'Production document probe compilation failed.'}
    & node scripts/test-official-video-layout.cjs
    if($LASTEXITCODE -ne 0){throw 'Wide/portrait player layout checks failed.'}
} finally { Pop-Location }
