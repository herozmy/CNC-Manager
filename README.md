# CNC-Manager v2

面向数控加工场景的程序台账系统，按“图纸 → 工序 → 程序 → 版本”组织 NC 文件，支持手机、平板和桌面浏览器访问。Go 服务内嵌 React 前端，业务数据使用 SQLite 保存。

源码分支为 `v2`，源码基线标签为 `v2`。应用构建版本以根目录 [VERSION](VERSION) 为准；可从 [Releases](https://github.com/herozmy/CNC-Manager/releases) 下载 Linux amd64 二进制及 SHA-256 校验清单。

## 主要功能

- **图纸与工序**：维护图纸号、类型、材料、价格、备注，以及装夹方式、分中方式和 X/Y/Z 坐标；支持搜索和删除确认。图纸价格以人民币元填写，可留空，支持零及最多两位小数，范围 0～999999999.99 元；在图纸详情查看和编辑。
- **NC 程序与版本**：在程序详情编辑程序号、名称、数控系统和备注，保留原有 NC 文件、版本历史和刀具表；上传、查看、下载文件，保留版本历史，选择当前版本并比较版本差异。
- **在线编辑**：行号、语法高亮、查找替换、跳转；支持另存新版本及覆盖冲突检查，处理 UTF-8 / GBK 编码。
- **版本刀具表**：维护刀具号、名称与刀补，支持从 NC 内容识别、手动增删和排序，每个版本独立保存。
- **账号设置**：首次创建管理员、登录退出、修改显示名称和密码。
- **备份与恢复**：管理员创建整站 ZIP，保存在服务器并列出；按需下载、确认删除，导入前检查并自动保存恢复副本。

## 从源码运行

需要 Go 1.27.1 或更高版本，以及 Node.js 24 和 npm。以下命令适用于 macOS / Linux 终端；生成当前系统和架构的程序。

```bash
git clone --branch v2 --single-branch https://github.com/herozmy/CNC-Manager.git CNC-Manager-v2
cd CNC-Manager-v2
npm --prefix frontend ci
npm --prefix frontend run build
mkdir -p .local/bin
(cd backend && go build \
  -ldflags "-X main.version=$(tr -d '[:space:]' < ../VERSION)" \
  -o ../.local/bin/cnc-manager ./cmd/server)
CNC_ADDR=127.0.0.1:8080 CNC_DATA_DIR=./data ./.local/bin/cnc-manager
```

打开 [http://127.0.0.1:8080](http://127.0.0.1:8080)，首次访问按页面提示创建管理员。浏览器必须能连接运行服务的电脑；局域网部署时可将监听地址设为 `0.0.0.0:8080`，其他设备通过服务器地址访问。

## Linux 构建

在源码根目录执行：

```bash
bash scripts/build.sh
```

生成 Linux amd64 单文件程序 `.local/release/cnc-manager`。前端和密码重置功能均已内置，运行时不需要 Node.js 或 Python。使用 `cnc-manager --version` 查看版本。

## Docker 部署

在新克隆的干净源码目录构建镜像，使用命名卷保存数据：

```bash
docker build -t cnc-manager:v2 .
docker run -d --name cnc-manager \
  --restart unless-stopped \
  -p 8080:8080 \
  -e CNC_DATA_DIR=/data \
  -v cnc-manager-data:/data \
  cnc-manager:v2
```

通过服务器的 `8080` 端口访问。更新容器时复用原数据卷，升级前先创建并下载备份。

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CNC_ADDR` | `127.0.0.1:8080` | 监听地址；Docker 镜像默认 `0.0.0.0:8080` |
| `CNC_DATA_DIR` | `./data` | 数据目录，运行账号需有写权限 |
| `CNC_MAX_UPLOAD_MB` | `64` | 单个 NC 文件上传上限，范围 1～1024 MB |
| `CNC_SESSION_HOURS` | `168` | 登录会话有效时长，范围 1～8760 小时 |

`GET /api/meta` 可查看正在运行的程序版本。

数据目录包含 `cnc.db`（数据库）、`nc/`（加密 NC 文件）、`nc.key`（文件解密密钥）和 `backups/`（备份）。整体迁移时应停服后复制完整目录，保留原密钥和访问权限。

## 使用流程

1. 创建图纸，在图纸下添加工序，填写装夹和坐标等信息。
2. 在工序下创建程序，进入程序详情上传 NC 文件。
3. 查看版本和刀具表，按需编辑内容、维护刀补或选择当前版本。
4. 下载所需 NC 文件；加工前核对版本、刀具及机床参数。

NC 刀具识别和语法高亮用于整理与查看，不提供加工仿真或机床安全校验。当前为联网使用的响应式网页，尚不支持离线编辑和机床直连传输。备份操作已限制管理员，其他业务写操作尚未实现完整的角色权限区分。

## 备份与恢复

在“我的 → 备份与迁移”操作：

1. 点击“创建备份”。完成后 ZIP 保存在服务器，页面刷新列表，不自动下载。
2. 在“可恢复备份”中点击对应条目的“下载”，并在设备下载记录中确认保存；删除服务器备份需再次确认。
3. 恢复或迁移到同版本服务时选择“导入备份”，上传 ZIP 并检查摘要，确认覆盖后导入。
4. 导入前系统自动备份目标数据。导入完成后使用备份中的账号和密码重新登录。

ZIP 包含账号、已保存的业务数据、NC 文件及密钥；不包含有效登录会话、未保存编辑或其他历史备份。导入会覆盖目标服务器的账号和业务数据。备份包未额外加密，请妥善保管。

网页备份上限为 ZIP 512 MB、解压后 2 GB、100,000 个 NC 文件。超出范围时采用停服复制完整数据目录的方式迁移。

v2.0.2 增加图纸价格字段，首次启动会自动升级数据库，原图纸价格为空。升级前完整备份数据；旧版 ZIP 需先用匹配版本恢复，再升级程序，新版 ZIP 包含价格。不将升级后的数据库直接交给旧版程序使用。

## 忘记密码

从 `v2.0.1` 起，服务器管理员直接使用主程序的 `reset-password` 子命令：

```bash
sudo ./cnc-manager reset-password \
  --data-dir /opt/cnc-manager-v2/data \
  --service cnc-manager-v2.service
```

默认数据目录为 `/opt/cnc-manager-v2/data`（可通过 `CNC_DATA_DIR` 或 `--data-dir` 修改），默认服务为 `cnc-manager-v2.service`。服务必须显式配置相同的绝对路径 `CNC_DATA_DIR`；工具会核对目录与服务的对应关系。

只有一个启用账号时自动选择，多个账号时按编号选择；也可增加 `--user admin` 指定登录用户名。交互输入两次新密码（8～128 个字符），输入不显示，Ctrl+C 可取消；远程使用 `ssh -t` 分配终端。工具先停服，将完整数据复制到数据目录同级的 `backups/pre-password-reset-*/data/`，再事务修改密码并撤销该账号全部旧登录，最后恢复原本运行的服务。其他账号、业务数据和密钥保留。

```bash
./cnc-manager reset-password --help
./cnc-manager reset-password --data-dir /path/to/data --list
./cnc-manager reset-password --data-dir /path/to/offline/data --no-service
```

`--list` 只列出账号。`--no-service` 只用于已停止服务的离线数据目录，需由调用方先确认所有写入进程已停止；新版本有数据目录占用检查，旧版本不具备此检查。

备份或数据库操作失败不会提交密码变更，并尝试恢复服务；若提示“密码已重置，但服务恢复失败”，请检查服务状态后使用新密码登录。此功能依赖服务器文件访问/管理权限，无公开 HTTP 密码重置入口。

## 项目结构与检查

- `backend/`：Go 服务、接口、数据库、NC 解析与存储。
- `frontend/`：React + TypeScript 界面。
- `scripts/`：构建和服务器维护工具。
- `Dockerfile`：容器构建配置。

```bash
(cd frontend && npm ci && npm run typecheck && npm test && npm run lint && npm run build)
(cd backend && go vet ./... && go test ./...)
```
