<#
  build-release.ps1 —— 打包 Windows 免安装版

  产出：.local\release\cnccool-<版本>-windows-amd64.zip
  解压后的目录结构：

      cnccool-server.exe   后端服务（接口和界面都由它提供）
      web\                 前端构建产物
      start.cmd            双击启动
      README.txt           使用说明

  打包进去的前端是纯静态文件，服务端通过 CNC_WEB_DIR 指过去，
  所以用户拿到手不需要装 Node，也不需要额外架 Web 服务器。

  注意这不影响开发：开发时不设 CNC_WEB_DIR，前端照旧由 Vite 提供、
  照旧热更新（见 scripts\run-backend.ps1 和 frontend 的 dev 脚本）。

  参数：
    -SkipFrontend   跳过前端构建。前端产物没动时可以省掉这一步。
#>
param(
    [switch]$SkipFrontend
)

$ErrorActionPreference = 'Stop'

$root     = Split-Path -Parent $PSScriptRoot
$backend  = Join-Path $root 'backend'
$frontend = Join-Path $root 'frontend'
$goExe    = Join-Path $root '.tools\go\bin\go.exe'

if (-not (Test-Path $goExe)) {
    Write-Host "找不到便携版 Go，改用系统 PATH 里的 go。" -ForegroundColor Yellow
    $goExe = 'go'
}

# ---- 版本号 -------------------------------------------------------------
# 版本号的真源只有 VERSION 一个文件，标签、二进制、界面三处都从它派生。
$versionFile = Join-Path $root 'VERSION'
if (-not (Test-Path $versionFile)) { throw "找不到版本号文件：$versionFile" }
$version = (Get-Content $versionFile -Raw).Trim()
if ($version -notmatch '^v\d+\.\d+') { throw "VERSION 的内容不像版本号：$version" }

Write-Host "打包 CNC 加工程序管理系统 $version" -ForegroundColor Green

# VERSION 和 git tag 对不上是很常见的失误，提前提示一句，
# 免得发出了一个"标签和内容对不上"的包。
$gitExe = Join-Path $root '.tools\git\cmd\git.exe'
if (Test-Path $gitExe) {
    $null = & $gitExe -C $root rev-parse -q --verify "refs/tags/$version" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "提示：仓库里还没有 $version 这个 tag，发布前记得先打上。" -ForegroundColor Yellow
    }
}

$name   = "cnccool-$version-windows-amd64"
$outDir = Join-Path $root '.local\release'
$stage  = Join-Path $outDir $name
$zip    = Join-Path $outDir "$name.zip"

# 每次从干净目录开始，避免上一次的残留文件混进包里。
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

# ---- 1. 后端 ------------------------------------------------------------
Write-Host "`n[1/4] 编译后端 ..." -ForegroundColor Cyan
$env:GOTOOLCHAIN = 'local'
$env:CGO_ENABLED = '0'
# -trimpath 去掉二进制里本机的绝对路径；-s -w 去掉调试信息，体积能小三分之一；
# 版本号用 ldflags 注入，保证界面显示的版本和 VERSION 文件一致。
$ldflags = "-s -w -X main.version=$version"
$exePath = Join-Path $stage 'cnccool-server.exe'
Push-Location $backend
try {
    & $goExe build -trimpath -ldflags $ldflags -o $exePath '.\cmd\server'
    if ($LASTEXITCODE -ne 0) { throw '后端编译失败' }
} finally {
    Pop-Location
}
$exeMB = [math]::Round((Get-Item $exePath).Length / 1MB, 1)
Write-Host "      cnccool-server.exe  $exeMB MB"

# ---- 2. 前端 ------------------------------------------------------------
if ($SkipFrontend) {
    Write-Host "`n[2/4] 跳过前端构建（-SkipFrontend）" -ForegroundColor DarkGray
} else {
    Write-Host "`n[2/4] 构建前端 ..." -ForegroundColor Cyan
    Push-Location $frontend
    try {
        & npm.cmd run build
        if ($LASTEXITCODE -ne 0) {
            throw '前端构建失败。若提示文件被占用，先停掉正在跑的 Vite dev server 再试。'
        }
    } finally {
        Pop-Location
    }
}

$dist = Join-Path $frontend 'dist'
if (-not (Test-Path (Join-Path $dist 'index.html'))) {
    throw "前端产物不完整：$dist 下面没有 index.html"
}

# ---- 3. 组装 ------------------------------------------------------------
Write-Host "`n[3/4] 组装免安装目录 ..." -ForegroundColor Cyan

$webStage = Join-Path $stage 'web'
New-Item -ItemType Directory -Force -Path $webStage | Out-Null
Copy-Item (Join-Path $dist '*') $webStage -Recurse -Force

# 包内的两个文本文件用 GBK 写：
#   - start.cmd 由 cmd.exe 按控制台代码页（简体中文 Windows 是 936）解释，
#     存成 UTF-8 会把中文显示成乱码；
#   - README.txt 存成 ANSI 同样能被记事本正确识别。
# 内容里因此不能出现 GBK 收录不了的字符（例如 Ø）。
$gbk = [System.Text.Encoding]::GetEncoding(936)

$startCmd = @'
@echo off
rem ---------------------------------------------------------------
rem  CNC Manager - portable start script
rem
rem  All data (SQLite database + NC file store) lives in the data\
rem  folder next to this file, so copying the whole folder is a
rem  complete backup.
rem
rem  To use another port:   start.cmd 8090
rem
rem  CNC_ADDR is only defaulted here, not forced: when the server restarts
rem  itself to install an update, the apply script passes the port it was
rem  running on through the environment, so a server started with
rem  "start.cmd 8090" comes back on 8090 instead of silently moving to 8080.
rem ---------------------------------------------------------------
setlocal
cd /d "%~dp0"

if not defined CNC_ADDR set "CNC_ADDR=127.0.0.1:8080"
if not "%~1"=="" set "CNC_ADDR=127.0.0.1:%~1"
set "CNC_DATA_DIR=%~dp0data"
set "CNC_WEB_DIR=%~dp0web"

if not exist "%~dp0cnccool-server.exe" goto no_exe
if not exist "%~dp0web\index.html" goto no_web

echo ============================================================
echo   CNC Manager
echo ============================================================
echo   URL    http://%CNC_ADDR%
echo   Data   %CNC_DATA_DIR%
echo.
echo   The browser opens automatically in a few seconds.
echo   Close this window to stop the server.
echo ============================================================
echo.

rem CNC_NO_BROWSER is set by the apply script while restarting after an
rem update: the tab the user already has open reconnects by itself, so
rem opening another one would just be noise.
if defined CNC_NO_BROWSER goto skip_browser
start "" powershell.exe -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 3; Start-Process 'http://%CNC_ADDR%'"
:skip_browser

"%~dp0cnccool-server.exe"
set "RC=%ERRORLEVEL%"

rem Exit code 99 means "restarting in order to install an update", not a crash.
rem The apply script is already waiting to swap the files and will open a fresh
rem window, so this one must close instead of sitting on a pause.
if "%RC%"=="99" exit /b 0

echo.
echo Server stopped (exit code %RC%).
pause
exit /b %RC%

:no_exe
echo [ERROR] cnccool-server.exe not found next to this script.
echo         Please unzip the package completely.
pause
exit /b 1

:no_web
echo [ERROR] web\index.html not found.
echo         The web folder must stay next to the exe.
pause
exit /b 1
'@

$readme = @"
CNC 加工程序管理系统 $version
============================================================

一句话说明
------------------------------------------------------------
把图纸号、工序、NC 程序串起来的本地小工具，数据全部存在本机，
不联网、不传云端，拷走整个文件夹就等于完成一次备份。


怎么用
------------------------------------------------------------
1. 把整个文件夹解压到任意位置，路径里尽量不要有特殊符号。
2. 双击 start.cmd。
3. 浏览器会自动打开 http://127.0.0.1:8080，没自动打开就手动输这个地址。
4. 想停止服务：关掉那个黑色命令行窗口即可。

使用流程：
    添加图纸  ->  添加工序（10# / 20# / 30# ...）  ->  添加程序号  ->  上传 NC 文件

上传 NC 文件后，系统会自动识别里面用到的刀具（M06 后面跟的 T 号），
单独出现的 T 是备刀，不计入加工刀具。


换端口
------------------------------------------------------------
默认 8080。如果提示端口被占用，就在命令行里执行：

    start.cmd 8090

然后访问 http://127.0.0.1:8090


数据放在哪
------------------------------------------------------------
data\ 目录，就在 exe 旁边：

    data\cnccool.db   数据库（图纸 / 工序 / 程序 / 刀具等）
    data\nc\          上传的 NC 原文件

备份 = 复制整个文件夹。
恢复 = 把整个文件夹复制回来。


更新到新版本
------------------------------------------------------------
方式一（推荐）：在界面里装
    界面左下角版本号旁边出现「有新版本 vX.Y.Z」时，点它打开对话框，
    按提示从发布页下载新的 zip，再在对话框里选中那个 zip，
    点「安装并重启」即可。服务会自己换文件、自己重启，data\ 不动。

    如果这台机器上不了外网，就在别的电脑上下载好，用 U 盘拷过来，
    同样是在这个对话框里选文件安装。

方式二：手工替换
    1. 关掉服务（关掉黑窗口）。
    2. 先备份 data\ 目录。
    3. 用新版本的 cnccool-server.exe 和 web\ 覆盖旧的，
       data\ 目录保持不动。
    4. 重新双击 start.cmd。数据库结构升级在启动时自动完成。


装完之后没反应 / 没装上
------------------------------------------------------------
    看安装目录下的 .update\apply-update.log，里面记了每一步的结果。
    更新失败会自动回滚，程序仍然能照常启动，只是版本没变。


出问题了怎么办
------------------------------------------------------------
窗口一闪就没了
    用命令行运行 start.cmd，这样错误信息不会消失。

提示端口被占用
    换端口：start.cmd 8090

浏览器打不开页面
    先确认黑窗口还在、没有报错，再手动访问 http://127.0.0.1:8080

杀毒软件报警
    这是自己编译的、没有数字签名的程序，加个信任即可。
    程序只监听本机 127.0.0.1。只有你点「检查更新」时才会去
    github.com 查一下最新版本号，不会上传任何数据。

页面显示不正常
    按 Ctrl+F5 强制刷新一次，清掉浏览器缓存。


说明
------------------------------------------------------------
本项目基于 Go + SQLite + Vue 构建，仅在本机运行，不对外提供服务。
源码地址：https://github.com/herozmy/CNC-Manager
"@

[System.IO.File]::WriteAllText((Join-Path $stage 'start.cmd'), $startCmd, $gbk)
[System.IO.File]::WriteAllText((Join-Path $stage 'README.txt'), $readme, $gbk)

# VERSION 必须放进包里：离线安装要先读它才知道装的是哪一版，
# 读到比较当前版本更新才允许装。少了它后端会直接拒绝这个包。
# 用 ASCII 写，不带行尾换行，跟仓库里那个文件保持一致。
[System.IO.File]::WriteAllText((Join-Path $stage 'VERSION'), $version, [System.Text.Encoding]::ASCII)

# ---- 4. 压缩 ------------------------------------------------------------
Write-Host "`n[4/4] 压缩 ..." -ForegroundColor Cyan
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zip -CompressionLevel Optimal

$zipMB  = [math]::Round((Get-Item $zip).Length / 1MB, 1)
$sha256 = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()

Write-Host ""
Write-Host "打包完成" -ForegroundColor Green
Write-Host "  文件    $zip"
Write-Host "  大小    $zipMB MB"
Write-Host "  版本    $version"
Write-Host "  SHA256  $sha256"
