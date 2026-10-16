param([string]$Database='zqzt_exhibition', [string]$OutputDirectory='', [string]$MediaDirectory='', [string]$MysqlBin='D:/InforGuardMC/InforGuardMC/mysql/bin')
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
if ($Database -notmatch '^zqzt_exhibition(?:_test_[0-9]+|_restore_[a-zA-Z0-9_]+)?$') { throw 'Only this application databases can be backed up with this script.' }
if (-not $OutputDirectory) { $OutputDirectory=Join-Path $taskRoot ('.local/backups/' + (Get-Date -Format 'yyyyMMdd-HHmmss')) }
if (-not $MediaDirectory) { $MediaDirectory=Join-Path $taskRoot 'server/data/media' }
if (-not (Test-Path -LiteralPath $MediaDirectory -PathType Container)) { throw 'Media directory does not exist.' }
if ((Test-Path -LiteralPath $OutputDirectory) -and @(Get-ChildItem -LiteralPath $OutputDirectory -Force).Count -gt 0) { throw 'Choose a new or empty backup directory to avoid replacing an existing backup.' }
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$priorPassword=$env:MYSQL_PWD
try {
    $env:MYSQL_PWD=if ($env:DB_PASSWORD) { $env:DB_PASSWORD } else { 'root' }
    $mysqlHost=if ($env:DB_HOST) { $env:DB_HOST } else { '127.0.0.1' }
    $mysqlPort=if ($env:DB_PORT) { $env:DB_PORT } else { '13306' }
    $mysqlUser=if ($env:DB_USER) { $env:DB_USER } else { 'root' }
    & "$MysqlBin/mysqldump.exe" --no-defaults "--host=$mysqlHost" "--port=$mysqlPort" "--user=$mysqlUser" --single-transaction --column-statistics=0 --set-gtid-purged=OFF --default-character-set=utf8mb4 "--result-file=$OutputDirectory/database.sql" $Database
    if ($LASTEXITCODE -ne 0) { throw 'Database backup failed.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::CreateFromDirectory([IO.Path]::GetFullPath($MediaDirectory), (Join-Path ([IO.Path]::GetFullPath($OutputDirectory)) 'media.zip'))
    @{database=$Database; createdAt=(Get-Date).ToUniversalTime().ToString('o');sqlHash=(Get-FileHash "$OutputDirectory/database.sql").Hash;mediaHash=(Get-FileHash "$OutputDirectory/media.zip").Hash} | ConvertTo-Json | Set-Content -LiteralPath "$OutputDirectory/manifest.json" -Encoding utf8
    Write-Output $OutputDirectory
} finally { $env:MYSQL_PWD=$priorPassword }
