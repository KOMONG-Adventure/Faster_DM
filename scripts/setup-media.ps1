param([string]$Destination = '')
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (-not $Destination) { $Destination = Join-Path $projectRoot 'build\bin\tools' }
$mediaCache = Join-Path $projectRoot '.cache\media-downloads'
New-Item -ItemType Directory -Force -Path $Destination,$mediaCache | Out-Null

function Get-CheckedFile([string]$Uri, [string]$Name, [string]$Hash) {
    $target = Join-Path $mediaCache $Name
    if (-not (Test-Path -LiteralPath $target) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $Hash) {
        Invoke-WebRequest -Uri $Uri -OutFile $target -UseBasicParsing -TimeoutSec 180
    }
    if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $Hash) { throw "Checksum mismatch: $Name. Do not run this file." }
    return $target
}

$yt = Get-CheckedFile 'https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp.exe' 'yt-dlp-2026.08.19.exe' '66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a'
Copy-Item -LiteralPath $yt -Destination (Join-Path $Destination 'yt-dlp.exe') -Force
$deno = Get-CheckedFile 'https://github.com/denoland/deno/releases/download/v2.9.7/deno-x86_64-pc-windows-msvc.zip' 'deno-2.9.7.zip' 'a0c3101b4158d1dfb7d6a78a7bf0f3de80c96bb423c152beec8beb22786f2238'
Expand-Archive -LiteralPath $deno -DestinationPath (Join-Path $mediaCache 'deno') -Force
Copy-Item -LiteralPath (Join-Path $mediaCache 'deno\deno.exe') -Destination $Destination -Force
$ffmpeg = Get-CheckedFile 'https://github.com/GyanD/codexffmpeg/releases/download/9.0.2/ffmpeg-9.0.2-essentials_build.zip' 'ffmpeg-9.0.2.zip' '60f467265b1e312373dbcd92200c2618a74850f98d3d078e94296bb3fa2047ba'
Expand-Archive -LiteralPath $ffmpeg -DestinationPath (Join-Path $mediaCache 'ffmpeg') -Force
foreach ($name in @('ffmpeg.exe','ffprobe.exe','LICENSE')) {
    $match = Get-ChildItem -LiteralPath (Join-Path $mediaCache 'ffmpeg') -Recurse -File | Where-Object { $_.Name -eq $name } | Select-Object -First 1
    if ($match) {
        $outputName = if ($name -eq 'LICENSE') { 'FFmpeg-LICENSE.txt' } else { $name }
        Copy-Item -LiteralPath $match.FullName -Destination (Join-Path $Destination $outputName) -Force
    }
}
Invoke-WebRequest 'https://raw.githubusercontent.com/yt-dlp/yt-dlp/2026.08.19/LICENSE' -OutFile (Join-Path $Destination 'yt-dlp-LICENSE.txt') -UseBasicParsing
Invoke-WebRequest 'https://raw.githubusercontent.com/denoland/deno/v2.9.7/LICENSE.md' -OutFile (Join-Path $Destination 'Deno-LICENSE.md') -UseBasicParsing
Copy-Item -LiteralPath (Join-Path $projectRoot 'THIRD_PARTY.md') -Destination $Destination -Force
Write-Host "YouTube tools ready: $Destination"
