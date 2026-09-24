$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
$env:GOCACHE = Join-Path $PSScriptRoot '.cache\go-build'
$env:GOMODCACHE = Join-Path $PSScriptRoot '.cache\go-mod'
$env:GOBIN = Join-Path $PSScriptRoot '.cache\tools'

Push-Location frontend
try {
    npm.cmd ci
    if ($LASTEXITCODE -ne 0) { throw 'Frontend install failed' }
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
} finally { Pop-Location }

go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
if ($LASTEXITCODE -ne 0) { throw 'Wails install failed' }
& (Join-Path $env:GOBIN 'wails.exe') build -s -webview2 download
if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
New-Item -ItemType Directory -Force -Path bin | Out-Null
Copy-Item -LiteralPath 'build\bin\FasterDM.exe' -Destination 'bin\fasterdm.exe' -Force
& (Join-Path $PSScriptRoot 'scripts\setup-media.ps1')
Write-Host 'Ready: build\bin\FasterDM.exe'
