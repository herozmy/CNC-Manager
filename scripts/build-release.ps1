<#
  build-release.ps1 —— 打包 Windows 和 Linux amd64 发布版

  产出：
      .local\release\cnccool-<版本>-windows-amd64.zip
      .local\release\cnccool-<版本>-linux-amd64.tar.gz

  两个平台的包都包含后端程序、web\ 前端构建产物、README 和 VERSION。
  Linux 包使用 CNC-Manager 作为程序名，并附带 systemd 服务及安装、卸载脚本。

      CNC-Manager.exe      后端服务（接口和界面都由它提供）
      web\                 前端构建产物
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

$outDir = Join-Path $root '.local\release'
$windowsName = "cnccool-$version-windows-amd64"
$linuxName = "cnccool-$version-linux-amd64"
$windowsStage = Join-Path $outDir $windowsName
$linuxStage = Join-Path $outDir $linuxName
$zip = Join-Path $outDir "$windowsName.zip"
$tarGz = Join-Path $outDir "$linuxName.tar.gz"

# 每次从干净目录开始，避免上一次的残留文件混进包里。
foreach ($stage in @($windowsStage, $linuxStage)) {
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
}

# ---- 1. 后端 ------------------------------------------------------------
Write-Host "`n[1/4] 编译 Windows 和 Linux 后端 ..." -ForegroundColor Cyan
$env:GOTOOLCHAIN = 'local'
$env:CGO_ENABLED = '0'
# -trimpath 去掉二进制里本机的绝对路径；-s -w 去掉调试信息，体积能小三分之一；
# 版本号用 ldflags 注入，保证界面显示的版本和 VERSION 文件一致。
$ldflags = "-H windowsgui -s -w -X main.version=$version"
$exeName = 'CNC-Manager.exe'
$exePath = Join-Path $windowsStage $exeName
Push-Location $backend
try {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    & $goExe build -trimpath -ldflags $ldflags -o $exePath '.\cmd\server'
    if ($LASTEXITCODE -ne 0) { throw 'Windows 后端编译失败' }

    $linuxPath = Join-Path $linuxStage 'CNC-Manager'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    & $goExe build -trimpath -ldflags "-s -w -X main.version=$version" -o $linuxPath '.\cmd\server'
    if ($LASTEXITCODE -ne 0) { throw 'Linux 后端编译失败' }
} finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}
$exeMB = [math]::Round((Get-Item $exePath).Length / 1MB, 1)
Write-Host "      $exeName  $exeMB MB"
$linuxMB = [math]::Round((Get-Item $linuxPath).Length / 1MB, 1)
Write-Host "      CNC-Manager      $linuxMB MB"

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

foreach ($stage in @($windowsStage, $linuxStage)) {
    $webStage = Join-Path $stage 'web'
    New-Item -ItemType Directory -Force -Path $webStage | Out-Null
    Copy-Item (Join-Path $dist '*') $webStage -Recurse -Force
}

# 包内 README.txt 使用 GBK，保证简体中文 Windows 的记事本正确识别。
# 内容里因此不能出现 GBK 收录不了的字符（例如 Ø）。
$gbk = [System.Text.Encoding]::GetEncoding(936)

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
2. 双击 CNC-Manager.exe。
3. 浏览器会自动打开 http://127.0.0.1:8080，没自动打开就手动输这个地址。
4. 想停止服务：右键系统托盘里的 CNC 图标，选择“退出”。

使用流程：
    添加图纸  ->  添加工序（10# / 20# / 30# ...）  ->  添加程序号  ->  上传 NC 文件

上传 NC 文件后，系统会自动识别里面用到的刀具（M06 后面跟的 T 号），
单独出现的 T 是备刀，不计入加工刀具。


数据放在哪
------------------------------------------------------------
data\ 目录，就在 exe 旁边：

    data\cnccool.db   数据库（图纸 / 工序 / 程序 / 刀具等）
    data\nc\          上传的 NC 原文件

备份 = 复制整个文件夹。
恢复 = 把整个文件夹复制回来。

也可以登录系统后打开右上角「备份管理」，创建包含数据库快照和全部 NC 原文件的
完整迁移包。备份按 1、2、3…号保存在 data\backups\，可恢复任意历史编号；
在新电脑上点击「导入备份」即可加入历史并覆盖当前数据，然后自动重启恢复。


更新到新版本
------------------------------------------------------------
方式一（推荐）：在界面里装
    界面左下角版本号旁边出现「有新版本 vX.Y.Z」时，点它打开对话框，
    按提示从发布页下载新的 zip，再在对话框里选中那个 zip，
    点「安装并重启」即可。服务会自己换文件、自己重启，data\ 不动。

    如果这台机器上不了外网，就在别的电脑上下载好，用 U 盘拷过来，
    同样是在这个对话框里选文件安装。

方式二：手工替换
    1. 从系统托盘退出服务。
    2. 先备份 data\ 目录。
    3. 用新版本的 CNC-Manager.exe 和 web\ 覆盖旧的，
       data\ 目录保持不动。
    4. 重新双击 CNC-Manager.exe。数据库结构升级在启动时自动完成。


装完之后没反应 / 没装上
------------------------------------------------------------
    看安装目录下的 .update\apply-update.log，里面记了每一步的结果。
    更新失败会自动回滚，程序仍然能照常启动，只是版本没变。


出问题了怎么办
------------------------------------------------------------
服务突然连不上 / 页面报「无法连接后端服务」
    先看 data\logs\server.log 的最后几行，那里记着每一次请求。
    如果日志停在某个时间点之后就没动静了，说明服务中途停了
    （进程被结束或被杀毒软件拦截了），重新双击 CNC-Manager.exe 即可。
    日志超过 4MB 会自动滚动成 server.log.1，只留一代。

浏览器打不开页面
    先确认系统托盘中有 CNC 图标，再手动访问 http://127.0.0.1:8080；
    也可以右键托盘图标打开日志目录查看 server.log。

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

[System.IO.File]::WriteAllText((Join-Path $windowsStage 'README.txt'), $readme, $gbk)

$linuxReadme = @"
CNC 加工程序管理系统 $version（Linux amd64）

安装或升级（需要 root）：
    sudo ./install.sh

安装后浏览器访问 http://服务器IP:8080。

常用命令：
    sudo systemctl status cnc-manager
    sudo systemctl restart cnc-manager
    sudo journalctl -u cnc-manager -f
    sudo ./uninstall.sh

程序安装在 /opt/CNC-Manager，数据保存在 /var/lib/cnc-manager。
再次执行新版 install.sh 即可升级，安装脚本不会删除数据。升级前仍建议在界面中创建备份。
uninstall.sh 默认只卸载程序和服务、保留数据，并会明确打印数据目录位置。

Linux 版不支持 Windows 免安装版的界面内离线升级和自动恢复重启。
"@
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $linuxStage 'README.txt'), $linuxReadme, $utf8NoBom)

$installSh = @'
#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 sudo ./install.sh 运行" >&2
  exit 1
fi
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP_DIR=/opt/CNC-Manager
DATA_DIR=/var/lib/cnc-manager
if ! id cnc-manager >/dev/null 2>&1; then
  useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin cnc-manager
fi
systemctl stop cnc-manager.service 2>/dev/null || true
install -d -m 0755 "$APP_DIR" "$APP_DIR/web"
install -d -o cnc-manager -g cnc-manager -m 0750 "$DATA_DIR"
install -m 0755 "$SOURCE_DIR/CNC-Manager" "$APP_DIR/CNC-Manager"
rm -rf "$APP_DIR/web"
cp -R "$SOURCE_DIR/web" "$APP_DIR/web"
chown -R root:root "$APP_DIR"
find "$APP_DIR/web" -type d -exec chmod 0755 {} \;
find "$APP_DIR/web" -type f -exec chmod 0644 {} \;
install -m 0644 "$SOURCE_DIR/cnc-manager.service" /etc/systemd/system/cnc-manager.service
systemctl daemon-reload
systemctl enable --now cnc-manager.service
echo "安装完成：http://服务器IP:8080"
'@
[System.IO.File]::WriteAllText((Join-Path $linuxStage 'install.sh'), $installSh.Replace("`r`n", "`n") + "`n", $utf8NoBom)

$uninstallSh = @'
#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 sudo ./uninstall.sh 运行" >&2
  exit 1
fi
systemctl disable --now cnc-manager.service 2>/dev/null || true
rm -f /etc/systemd/system/cnc-manager.service
systemctl daemon-reload
rm -rf /opt/CNC-Manager
echo "程序和服务已卸载。数据仍保留在 /var/lib/cnc-manager。"
'@
[System.IO.File]::WriteAllText((Join-Path $linuxStage 'uninstall.sh'), $uninstallSh.Replace("`r`n", "`n") + "`n", $utf8NoBom)

$service = @'
[Unit]
Description=CNC Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=cnc-manager
Group=cnc-manager
WorkingDirectory=/var/lib/cnc-manager
Environment=TZ=Asia/Shanghai
Environment=CNC_ADDR=0.0.0.0:8080
Environment=CNC_DATA_DIR=/var/lib/cnc-manager
Environment=CNC_WEB_DIR=/opt/CNC-Manager/web
Environment=CNC_NO_BROWSER=1
Environment=CNC_UPDATE_REPO=
ExecStart=/opt/CNC-Manager/CNC-Manager
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/cnc-manager

[Install]
WantedBy=multi-user.target
'@
[System.IO.File]::WriteAllText((Join-Path $linuxStage 'cnc-manager.service'), $service.Replace("`r`n", "`n") + "`n", $utf8NoBom)

# VERSION 必须放进包里：离线安装要先读它才知道装的是哪一版，
# 读到比较当前版本更新才允许装。少了它后端会直接拒绝这个包。
# 用 ASCII 写，不带行尾换行，跟仓库里那个文件保持一致。
foreach ($stage in @($windowsStage, $linuxStage)) {
    [System.IO.File]::WriteAllText((Join-Path $stage 'VERSION'), $version, [System.Text.Encoding]::ASCII)
}

# ---- 4. 压缩 ------------------------------------------------------------
Write-Host "`n[4/4] 压缩 Windows zip 和 Linux tar.gz ..." -ForegroundColor Cyan
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $windowsStage '*') -DestinationPath $zip -CompressionLevel Optimal

if (Test-Path $tarGz) { Remove-Item $tarGz -Force }
& $goExe run (Join-Path $PSScriptRoot 'pack-linux-release.go') -source $linuxStage -output $tarGz -name $linuxName
if ($LASTEXITCODE -ne 0) { throw 'Linux tar.gz 压缩失败' }

$zipMB  = [math]::Round((Get-Item $zip).Length / 1MB, 1)
$sha256 = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
$tarMB = [math]::Round((Get-Item $tarGz).Length / 1MB, 1)
$tarSHA256 = (Get-FileHash $tarGz -Algorithm SHA256).Hash.ToLower()

Write-Host ""
Write-Host "打包完成" -ForegroundColor Green
Write-Host "  文件    $zip"
Write-Host "  大小    $zipMB MB"
Write-Host "  版本    $version"
Write-Host "  SHA256  $sha256"
Write-Host ""
Write-Host "  文件    $tarGz"
Write-Host "  大小    $tarMB MB"
Write-Host "  版本    $version"
Write-Host "  SHA256  $tarSHA256"
