$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
$configPath=Join-Path $taskRoot 'server/.env'
if(-not (Test-Path -LiteralPath $configPath)){throw 'Create server/.env first.'}
$configText=[IO.File]::ReadAllText($configPath)
$settings=@{}
foreach($line in ($configText -split "`n")){
    if($line.Trim() -match '^([^#=]+)=(.*)$'){$settings[$Matches[1].Trim()]=$Matches[2].Trim().Trim('"')}
}
$required=@('ALIYUN_ACCESS_KEY_ID','ALIYUN_ACCESS_KEY_SECRET','ALIYUN_SMS_SIGN_NAME','ALIYUN_SMS_TEMPLATE_CODE')
$missing=@($required | Where-Object {-not [Environment]::GetEnvironmentVariable($_) -and -not $settings[$_]})
if($missing.Count){throw ('Fill server/.env or environment variables before enabling SMS: '+($missing -join ', '))}
$updated=if($configText -match '(?m)^SMS_PROVIDER='){$configText -replace '(?m)^SMS_PROVIDER=[^\r\n]*','SMS_PROVIDER=aliyun'}else{$configText+"`nSMS_PROVIDER=aliyun`n"}
[IO.File]::WriteAllText($configPath,$updated,[Text.UTF8Encoding]::new($false))
$priorProvider=$env:SMS_PROVIDER
try {
    $env:SMS_PROVIDER='aliyun'
    & "$PSScriptRoot/stop.ps1"
    & "$PSScriptRoot/start.ps1"
} catch {
    [IO.File]::WriteAllText($configPath,$configText,[Text.UTF8Encoding]::new($false))
    throw
} finally {$env:SMS_PROVIDER=$priorProvider}
Write-Output 'Aliyun SMS selected. Check /api/v1/health; no test SMS was sent.'
