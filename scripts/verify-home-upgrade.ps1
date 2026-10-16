param([string]$BaseUrl = 'http://39.97.250.244:8089', [string]$EvidencePrefix = 'home-upgrade')
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
function Read-Public([string]$Path) {
    Invoke-RestMethod -Uri "$BaseUrl/api/v1$Path" -NoProxy -TimeoutSec 25
}
$taskData = @{}
foreach ($taskName in @('home', 'taxonomy', 'vendors', 'products')) {
    $taskData[$taskName] = Read-Public "/public/$taskName"
}
$taskChecks = [Collections.Generic.List[object]]::new()
foreach ($taskVendor in @($taskData.home.vendors | Select-Object -First 6)) {
    $taskDetail = Read-Public "/public/vendors/$($taskVendor.id)"
    $taskSlug = if ($taskVendor.showroom_slug) { $taskVendor.showroom_slug } else { "v-$($taskVendor.id)" }
    $taskShowroom = Read-Public "/public/showrooms/$([Uri]::EscapeDataString($taskSlug))"
    $taskEqual = ($taskDetail.item.data | ConvertTo-Json -Depth 25 -Compress) -ceq ($taskVendor.data | ConvertTo-Json -Depth 25 -Compress)
    $taskChecks.Add([pscustomobject]@{Kind='vendor';ID=$taskVendor.id;Name=$taskVendor.data.name;Slug=$taskSlug;HomeMatchesDetail=$taskEqual;ShowroomMatchesDetail=(($taskShowroom.item.data | ConvertTo-Json -Depth 25 -Compress) -ceq ($taskDetail.item.data | ConvertTo-Json -Depth 25 -Compress))})
    if (-not $taskEqual -or $taskShowroom.item.id -ne $taskVendor.id) { throw 'Vendor public data mismatch' }
}
foreach ($taskProduct in @($taskData.home.products | Select-Object -First 6)) {
    $taskDetail = Read-Public "/public/products/$($taskProduct.id)"
    $taskEqual = ($taskDetail.item.data | ConvertTo-Json -Depth 25 -Compress) -ceq ($taskProduct.data | ConvertTo-Json -Depth 25 -Compress)
    $taskChecks.Add([pscustomobject]@{Kind='product';ID=$taskProduct.id;Name=$taskProduct.data.name;HomeMatchesDetail=$taskEqual;VendorID=$taskDetail.vendor.id;HasContact=[bool]($taskDetail.vendor.data.phone -or $taskDetail.vendor.data.wechat -or $taskDetail.vendor.data.company_profile.contacts)})
    if (-not $taskEqual) { throw 'Product public data mismatch' }
}
$taskImages = [Collections.Generic.HashSet[int]]::new()
if ($taskData.home.hero.media_id -gt 0) { [void]$taskImages.Add([int]$taskData.home.hero.media_id) }
foreach ($taskItem in @($taskData.home.vendors) + @($taskData.home.products) + @($taskData.vendors.items) + @($taskData.products.items)) {
    foreach ($taskMediaID in @($taskItem.data.media_ids) + @($taskItem.data.company_profile.cover_media_id) + @($taskItem.data.company_profile.logo_media_id)) {
        if ($taskMediaID -gt 0) { [void]$taskImages.Add([int]$taskMediaID) }
    }
}
$taskImageDirectory = Join-Path $taskRoot '.local/home-upgrade-images'
New-Item -ItemType Directory -Path $taskImageDirectory -Force | Out-Null
foreach ($taskMediaID in $taskImages) {
    $taskSuffix = if ($taskMediaID -eq $taskData.home.hero.media_id) { '' } else { '?thumb=1' }
    Invoke-WebRequest -Uri "$BaseUrl/api/v1/media/$taskMediaID$taskSuffix" -NoProxy -TimeoutSec 25 -OutFile (Join-Path $taskImageDirectory "$taskMediaID.bin")
}
[IO.File]::WriteAllText((Join-Path $taskRoot '.local/home-upgrade-live-fixture.json'), ($taskData | ConvertTo-Json -Depth 30))
$taskReport = [ordered]@{CheckedAt=(Get-Date).ToUniversalTime().ToString('o');BaseUrl=$BaseUrl;ReadOnly=$true;HeroMediaID=$taskData.home.hero.media_id;FeaturedVendors=$taskData.home.vendors.Count;FeaturedProducts=$taskData.home.products.Count;CategoryNames=@($taskData.taxonomy.categories | ForEach-Object {$_.name});MediaVerified=$taskImages.Count;Checks=$taskChecks}
$taskReport | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $taskRoot "docs/test-output/$EvidencePrefix-live-data.json") -Encoding utf8
$taskReport | ConvertTo-Json -Depth 8
