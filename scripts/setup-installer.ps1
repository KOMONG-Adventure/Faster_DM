$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$compilerDir = Join-Path (Split-Path -Parent $PSScriptRoot) '.cache\inno'
$iscc = Join-Path $compilerDir 'compiler\ISCC.exe'
if (Test-Path -LiteralPath $iscc) { return $iscc }
New-Item -ItemType Directory -Force -Path $compilerDir | Out-Null
$setup = Join-Path $compilerDir 'innosetup-6.7.3.exe'
Invoke-WebRequest 'https://github.com/jrsoftware/issrc/releases/download/is-6_7_3/innosetup-6.7.3.exe' -OutFile $setup -UseBasicParsing -TimeoutSec 180
if ((Get-FileHash -LiteralPath $setup -Algorithm SHA256).Hash -ne '9c73c3bae7ed48d44112a0f48e66742c00090bdb5bef71d9d3c056c66e97b732') { throw 'Inno Setup checksum mismatch' }
$process = Start-Process -FilePath $setup -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/CURRENTUSER','/NOICONS','/TASKS=',('/DIR="' + $compilerDir + '\compiler"')) -WindowStyle Hidden -Wait -PassThru
if ($process.ExitCode -ne 0 -or -not (Test-Path -LiteralPath $iscc)) { throw 'Inno Setup compiler installation failed' }
return $iscc
