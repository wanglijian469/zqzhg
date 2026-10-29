param([string]$ServerBinary)
$ErrorActionPreference = 'Stop'
# Read-only ECS preflight. No configuration change, restart or SMS send.
foreach ($key in @('ALIYUN_SMS_SIGN_NAME','ALIYUN_SMS_TEMPLATE_CODE','ALIYUN_SMS_RESET_TEMPLATE_CODE','ALIYUN_SMS_RESET_CODE_VARIABLE')) {
    if (-not [Environment]::GetEnvironmentVariable($key)) { throw "Missing server environment setting: $key" }
}
if (-not $ServerBinary -or -not (Test-Path -LiteralPath $ServerBinary -PathType Leaf)) { throw 'Specify the backend executable with -ServerBinary on the ECS; no AccessKey is required.' }
& $ServerBinary sms-preflight
if ($LASTEXITCODE -ne 0) { throw 'RAM role/template preflight failed; SMS remains unchanged.' }
Write-Output 'RAM role and approved templates verified. No SMS sent; no service configuration changed.'
