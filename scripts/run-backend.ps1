<#
  run-backend.ps1 —— 启动后端服务

  会自动找到项目自带的便携版 Go，不需要系统里装 Go。
  数据目录默认是 backend\data，数据库和 NC 文件库都在里面。

  版本号的真源是仓库根目录的 VERSION 文件，这里读出来注入到二进制里。
  这样界面上显示的版本号、启动日志里的版本号、和 VERSION 文件永远一致，
  不会出现"代码里写一个版本、文档里写另一个"这种对不上的情况。
#>
param(
    [string]$Addr = '127.0.0.1:8080',
    [switch]$Rebuild
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root 'backend'
$goExe = Join-Path $root '.tools\go\bin\go.exe'

if (-not (Test-Path $goExe)) {
    Write-Host "找不到便携版 Go：$goExe" -ForegroundColor Red
    Write-Host "请先下载 go1.27.1.windows-amd64.zip 解压到 .tools\ 目录，或用系统安装的 Go。" -ForegroundColor Yellow
    $goExe = 'go'
}

# 读取版本号；VERSION 文件缺失时用 v0.00 占位，但不静默忽略
$versionFile = Join-Path $root 'VERSION'
$version = 'v0.00'
if (Test-Path $versionFile) {
    $version = (Get-Content $versionFile -Raw).Trim()
} else {
    Write-Host "警告：找不到 $versionFile，版本号将显示为 $version" -ForegroundColor Yellow
}
$ldflags = "-X main.version=$version"

$env:CNC_ADDR = $Addr
$env:GOTOOLCHAIN = 'local'
$env:CGO_ENABLED = '0'

Write-Host "CNC 加工程序管理系统  $version" -ForegroundColor Green
Write-Host "监听地址  http://$Addr" -ForegroundColor Cyan
Write-Host "工作目录  $backend" -ForegroundColor DarkGray
Write-Host "数据目录  $backend\data" -ForegroundColor DarkGray
Write-Host "按 Ctrl+C 停止`n" -ForegroundColor DarkGray

Push-Location $backend
try {
    if ($Rebuild) {
        & $goExe build -ldflags $ldflags -o 'cnccool-server.exe' '.\cmd\server'
        if ($LASTEXITCODE -ne 0) { throw '编译失败' }
        & '.\cnccool-server.exe'
    } else {
        & $goExe run -ldflags $ldflags '.\cmd\server'
    }
} finally {
    Pop-Location
}
