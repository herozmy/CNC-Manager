# 项目协作说明

## 项目结构

- `backend/`：Go 后端服务、SQLite 数据库、REST API 和 NC 文件处理。
  - `cmd/server/`：服务入口。
  - `cmd/dbcheck/`：数据库巡检与在线备份工具。
  - `internal/domain/`：领域模型和请求模型。
  - `internal/httpapi/`：路由、参数校验和 HTTP 响应。
  - `internal/repo/`：手写 SQL 和数据访问。
  - `internal/store/`：SQLite 连接与数据库迁移。
  - `internal/ncparse/`：NC 程序和刀具调用解析。
  - `internal/ncstore/`：NC 文件内容寻址存储。
- `frontend/`：Vue 3 + TypeScript + Vite + Element Plus 前端。
  - `src/views/MainView.vue`：主界面。
  - `src/components/`：图纸、工序、程序、刀具、版本和升级组件。
  - `src/api/`：接口客户端和类型定义。
- `scripts/`：本地启动、演示数据和 Windows 发布包脚本。
- `VERSION`：版本号唯一真源。
- `backend/data/`：运行期数据库、日志和 NC 文件库，不纳入版本控制。

## 常用命令

在仓库根目录执行：

```powershell
# 前端类型检查
Push-Location frontend
npm.cmd run typecheck
Pop-Location

# 前端生产构建
Push-Location frontend
npm.cmd run build
Pop-Location

# 后端测试（使用项目自带 Go 1.27.1）
Push-Location backend
& ..\.tools\go\bin\go.exe test ./...
Pop-Location

# 构建 Windows 免安装发布包
scripts\build-release.cmd

# 构建仅供本机使用的测试项目（输出到被 Git 忽略的 dev/）
scripts\build-dev.cmd
```

发布前至少确认后端测试、前端构建和免安装包构建全部通过。发布包位于 `.local/release/`，托盘图标已嵌入服务程序；构建脚本只会将服务程序、前端静态文件、启动脚本、说明文件和 `VERSION` 放入压缩包，不包含本文件。

开发启动：后端运行 `scripts\run-backend.cmd`，另开窗口运行 `frontend\dev.cmd`。

## 代码修改与测试

- 修改业务代码时，必须在本地新增或更新与变更对应的自动化测试文件，不能只修改实现而不补测试。
- Go 后端测试文件使用 `*_test.go` 命名，并放在被测代码所在包内；优先覆盖正常流程、参数错误、未授权访问和数据边界。
- 测试需要文件或数据库时，使用 `t.TempDir()` 在本地生成隔离的测试文件，测试结束后由 Go 自动清理；禁止读写 `backend/data/` 中的真实运行数据。
- 修复缺陷时，应先增加能够复现问题的回归测试，再修改实现，并确认测试由失败变为通过。
- 前端修改至少运行类型检查和生产构建；已有可用测试框架的模块还必须新增或更新对应测试文件。
- 完成代码修改后，至少运行受影响模块的测试；提交或发布前运行后端全量测试、前端类型检查和生产构建。
- 需要运行完整编译产物时，使用 `scripts\build-dev.cmd` 生成 `dev/` 测试项目；`dev/` 及其中的数据库、日志和程序文件仅供本地测试，禁止强制加入 Git 或上传仓库。

## 发布注意事项

- GitHub Release 使用根目录 `VERSION`、Git tag 和构建产物保持一致。
- `AGENTS.md` 仅供 Codex/开发协作使用，不应进入 GitHub 源码归档或 Windows 发布包。
