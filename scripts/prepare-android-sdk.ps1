param([string]$SourceSdk='C:/Users/wang_ljian/AppData/Local/Android/Sdk')
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$projectSdk=Join-Path $taskRoot '.tools/android-sdk'
$projectNdk=Join-Path $taskRoot '.tools/android-ndk-r28c'
if (-not (Test-Path -LiteralPath (Join-Path $projectNdk 'source.properties'))) { throw 'Extract official Android NDK r28c into .tools first.' }
foreach ($component in @('platforms/android-36','build-tools/36.0.0','platform-tools','licenses')) {
    $sourceComponent=Join-Path $SourceSdk $component
    $targetComponent=Join-Path $projectSdk $component
    if (-not (Test-Path -LiteralPath $sourceComponent)) { throw "Missing source SDK component: $component" }
    if (-not (Test-Path -LiteralPath $targetComponent)) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $targetComponent) -Force | Out-Null
        Copy-Item -LiteralPath $sourceComponent -Destination $targetComponent -Recurse
    }
}
$ndkSlot=Join-Path $projectSdk 'ndk/28.2.13676358'
if (-not (Test-Path -LiteralPath $ndkSlot)) {
    New-Item -ItemType Directory -Path (Split-Path -Parent $ndkSlot) -Force | Out-Null
    New-Item -ItemType Junction -Path $ndkSlot -Target $projectNdk | Out-Null
}
Write-Output "Isolated build SDK ready: $projectSdk"
