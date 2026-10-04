$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$devDir = Join-Path $root 'dev'
$webDir = Join-Path $devDir 'web'
$goExe = Join-Path $root '.tools\go\bin\go.exe'
$version = (Get-Content (Join-Path $root 'VERSION') -Raw).Trim()

if (-not (Test-Path $goExe -PathType Leaf)) {
    throw "Bundled Go was not found: $goExe"
}

New-Item -ItemType Directory -Force -Path $devDir | Out-Null
$legacyIcon = Join-Path $devDir 'cnccool.ico'
if (Test-Path $legacyIcon) {
    Remove-Item -LiteralPath $legacyIcon -Force
}

Write-Host '[1/3] Building backend...'
Push-Location (Join-Path $root 'backend')
try {
    & $goExe build -ldflags "-H windowsgui -X main.version=$version" -o (Join-Path $devDir 'cnccool-server.exe') ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed.' }
}
finally {
    Pop-Location
}

Write-Host '[2/3] Building frontend...'
Push-Location (Join-Path $root 'frontend')
try {
    & npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
}
finally {
    Pop-Location
}

Write-Host '[3/3] Assembling local test project...'
if (Test-Path $webDir) {
    Remove-Item -LiteralPath $webDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $webDir | Out-Null
Copy-Item (Join-Path $root 'frontend\dist\*') $webDir -Recurse -Force
Set-Content -LiteralPath (Join-Path $devDir 'VERSION') -Value $version -Encoding ASCII

$startScript = @'
@echo off
setlocal
cd /d "%~dp0"
if not defined CNC_ADDR set "CNC_ADDR=127.0.0.1:8080"
if not "%~1"=="" set "CNC_ADDR=127.0.0.1:%~1"
set "CNC_DATA_DIR=%~dp0data"
set "CNC_WEB_DIR=%~dp0web"
set "CNC_NO_BROWSER=1"
echo Local test URL: http://%CNC_ADDR%
echo Local test data: %CNC_DATA_DIR%
start "" powershell.exe -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 2; Start-Process 'http://%CNC_ADDR%'"
"%~dp0cnccool-server.exe"
pause
'@
Set-Content -LiteralPath (Join-Path $devDir 'start.cmd') -Value $startScript -Encoding ASCII

Write-Host "Local test project created: $devDir"
Write-Host 'Run dev\start.cmd to start it. The dev\ directory is ignored by Git.'
