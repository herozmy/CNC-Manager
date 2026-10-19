# 发布流程

根目录 `VERSION` 是唯一版本号来源。一次正式版本更新必须同步完成 Git 仓库、Git tag、GitHub Release、Windows/Linux 发布包和 Docker Hub 镜像，不能只更新其中一部分。

## 1. 更新版本

将 `VERSION` 更新为新的 `vX.Y.Z`，同步 README 中展示的当前版本和示例标签。已经发布过的版本号不得重新构建或覆盖；内容变化必须使用新版本号。

## 2. 完整验证

在仓库根目录运行：

```powershell
Push-Location backend
& ..\.tools\go\bin\go.exe test ./...
Pop-Location

Push-Location frontend
npm.cmd run typecheck
npm.cmd run build
Pop-Location

scripts\build-release.cmd
```

构建必须同时产生：

```text
.local/release/cnccool-vX.Y.Z-windows-amd64.zip
.local/release/cnccool-vX.Y.Z-linux-amd64.tar.gz
```

检查 Linux 包内 `CNC-Manager`、`install.sh` 和 `uninstall.sh` 具有执行权限。

## 3. 提交与 Git tag

确认工作区只包含本次版本变更，提交并推送默认分支：

```bash
git add <本次发布文件>
git commit -m "feat: 发布 vX.Y.Z ..."
git push origin main
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

## 4. GitHub Release

使用对应 tag 创建 GitHub Release，发布说明应概述功能、兼容性、升级方法和验证结果，并上传两个构建产物。Release 名称、tag、包内 `VERSION` 和二进制显示版本必须完全一致。

## 5. Docker Hub

从同一提交构建 Linux amd64 镜像，并同时推送不可变版本标签和滚动标签：

```bash
docker build -t herozmy/cnc-manager:vX.Y.Z -t herozmy/cnc-manager:latest .
docker push herozmy/cnc-manager:vX.Y.Z
docker push herozmy/cnc-manager:latest
```

推送后核对两个标签的远端 digest 一致，并用拉取后的镜像验证 `/api/meta` 返回正确版本。不得用新内容覆盖已发布的旧版本标签。

## 6. 发布完成标准

以下各项全部成功才算完成发布：

1. 默认分支代码已推送；
2. 对应 Git tag 已推送；
3. GitHub Release 已创建且包含 Windows/Linux 两个产物；
4. Docker Hub 已包含 `vX.Y.Z` 和更新后的 `latest`；
5. Release 资产、Docker 镜像和程序接口报告同一个版本号；
6. SHA256、镜像 digest、测试与构建结果已记录在发布说明或交付总结中。
