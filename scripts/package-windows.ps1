param(
    [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version = '0.5.0',
    [switch]$ReuseMedia
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
    $env:GOMODCACHE = Join-Path $projectRoot '.cache\go-mod'
    $env:GOBIN = Join-Path $projectRoot '.cache\tools'
    $wails = Join-Path $env:GOBIN 'wails.exe'
    if (-not (Test-Path -LiteralPath $wails)) {
        go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
        if ($LASTEXITCODE -ne 0) { throw 'Wails install failed' }
    }
    & $wails build -o FasterDM-package.exe -webview2 download -ldflags "-X github.com/KOMONG-Adventure/Faster_DM/internal/updates.Version=$Version"
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
    if (-not $ReuseMedia) { & (Join-Path $PSScriptRoot 'setup-media.ps1') }
    $stage = Join-Path $projectRoot ('.cache\package-' + [guid]::NewGuid().ToString('N'))
    $releaseDir = Join-Path $projectRoot "dist\$Version"
    New-Item -ItemType Directory -Force -Path "$stage\tools",$releaseDir | Out-Null
    Copy-Item -LiteralPath 'build\bin\FasterDM-package.exe' -Destination "$stage\FasterDM.exe"
    foreach ($name in @('yt-dlp.exe','deno.exe','ffmpeg.exe','ffprobe.exe','Deno-LICENSE.md','FFmpeg-LICENSE.txt','yt-dlp-LICENSE.txt','THIRD_PARTY.md')) {
        Copy-Item -LiteralPath (Join-Path 'build\bin\tools' $name) -Destination "$stage\tools\$name"
    }
    Copy-Item -LiteralPath 'THIRD_PARTY.md' -Destination $stage
    Copy-Item -LiteralPath 'licenses' -Destination $stage -Recurse
    $compiler = & (Join-Path $PSScriptRoot 'setup-installer.ps1')
    & $compiler "/DAppVersion=$Version" "/DPackageDir=$stage" "/DReleaseDir=$releaseDir" 'build\windows\installer.iss'
    if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'install.ps1') -Destination $releaseDir -Force
    $checksums = foreach ($name in @("FasterDM-Setup-$Version-x64.exe", 'install.ps1')) {
        $hash = (Get-FileHash -LiteralPath (Join-Path $releaseDir $name) -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $name"
    }
    [IO.File]::WriteAllLines((Join-Path $releaseDir 'SHA256SUMS.txt'), $checksums, [Text.Encoding]::ASCII)
    Write-Host "Installer ready: $releaseDir"
} finally { Pop-Location }
