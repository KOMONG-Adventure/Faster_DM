param(
    [ValidatePattern('^v?\d+\.\d+\.\d+$')][string]$Version,
    [switch]$Silent
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
if (-not [Environment]::Is64BitOperatingSystem) { throw 'Windows 10/11 x64 шаардлагатай.' }
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
$repo = 'KOMONG-Adventure/Faster_DM'
$headers = @{ 'User-Agent' = 'FasterDM-Installer'; 'Accept' = 'application/vnd.github+json' }
$token = if ($env:GH_TOKEN) { $env:GH_TOKEN } else { $env:GITHUB_TOKEN }
if ($token) { $headers.Authorization = 'Bearer ' + $token }
$endpoint = if ($Version) { 'tags/v' + $Version.TrimStart('v') } else { 'latest' }
try { $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/$endpoint" -Headers $headers -TimeoutSec 30 }
catch { throw 'GitHub release авч чадсангүй. Интернэт болон репозиторийн нийтэд нээлттэй эсэхийг шалгана уу.' }
if ($release.draft -or $release.prerelease -or $release.tag_name -notmatch '^v(\d+\.\d+\.\d+)$') { throw 'Тогтвортой release олдсонгүй.' }
$number = $Matches[1]
$name = "FasterDM-Setup-$number-x64.exe"
if ($name -notin $release.assets.name -or 'SHA256SUMS.txt' -notin $release.assets.name) { throw 'Release installer эсвэл checksum дутуу байна.' }
$downloadDir = Join-Path ([IO.Path]::GetTempPath()) ('FasterDM-install-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $downloadDir | Out-Null
$installer = Join-Path $downloadDir $name
$manifest = Join-Path $downloadDir 'SHA256SUMS.txt'
try {
    $baseURL = "https://github.com/$repo/releases/download/$($release.tag_name)"
    Write-Host "Faster DM $number татаж байна..."
    function Download-Asset([string]$assetName, [string]$destination, [int]$timeout) {
        if ($token) {
            $asset = @($release.assets | Where-Object name -eq $assetName)
            if ($asset.Count -ne 1) { throw 'Release asset буруу байна.' }
            $assetHeaders = @{ 'User-Agent' = 'FasterDM-Installer'; Accept = 'application/octet-stream'; Authorization = 'Bearer ' + $token }
            Invoke-WebRequest "https://api.github.com/repos/$repo/releases/assets/$($asset[0].id)" -Headers $assetHeaders -OutFile $destination -UseBasicParsing -TimeoutSec $timeout
        } else {
            Invoke-WebRequest "$baseURL/$assetName" -OutFile $destination -UseBasicParsing -TimeoutSec $timeout
        }
    }
    Download-Asset 'SHA256SUMS.txt' $manifest 60
    $pattern = '^([a-fA-F0-9]{64})\s+\*?' + [regex]::Escape($name) + '$'
    $lines = @(Get-Content -LiteralPath $manifest | Where-Object { $_ -match $pattern })
    if ($lines.Count -ne 1) { throw 'Checksum manifest буруу байна.' }
    $null = $lines[0] -match $pattern
    $expected = $Matches[1]
    Download-Asset $name $installer 900
    if ((Get-FileHash -LiteralPath $installer -Algorithm SHA256).Hash -ne $expected) { throw 'Checksum зөрсөн. Суулгалтыг зогсоолоо.' }
    $arguments = @('/NORESTART', '/NOCLOSEAPPLICATIONS')
    if ($Silent) { $arguments += @('/VERYSILENT', '/SUPPRESSMSGBOXES') }
    $process = Start-Process -FilePath $installer -ArgumentList $arguments -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "Суулгалт дуусаагүй. Код: $($process.ExitCode). Хуучин апп нээлттэй бол хаагаад дахин оролдоно уу." }
    Write-Host 'Суулгалаа. Start Menu > Faster DM нээнэ үү.'
} finally {
    Remove-Item -LiteralPath $installer,$manifest -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $downloadDir -ErrorAction SilentlyContinue
}
