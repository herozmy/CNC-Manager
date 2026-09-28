# CNC 加工程序管理系统

**v0.06**

数控加工程序的结构化管理工具：以**图纸号**为主线，挂**工序**（一序、二序…），
工序下挂**程序**，每个程序带 **NC 文件版本历史**、**刀具补偿表**和**备注**。

后端 Go 单文件可执行程序，前端 Vue 单页应用，本地运行，可打包进 Docker。

版本号的真源是仓库根目录的 `VERSION` 文件，构建时注入后端；服务启动日志、
界面左下角和 `GET /api/meta` 都会显示。改版本只需改那一个文件。

---

## 功能

| 能力 | 说明 |
|---|---|
| 图纸管理 | 图纸号（唯一）、名称、材料、备注 |
| 工序管理 | 工序号用 10# / 20# / 30# 留插入余量；装夹方式、Z 轴高度垫高、备注 |
| 程序管理 | 程序号（如 O1234）；一道工序可挂多个程序 |
| NC 文件托管 | 上传原始 NC 文件，按内容 sha256 寻址去重，相同内容秒传；自动识别 GBK / UTF-8 |
| **刀具自动识别** | 上传时自动读程序正文，识别出刀具调用（T 号与刀补号）。识别结果**可以先手工改**（改号、删行、补行）再加入刀具补偿表；上传时没加，之后在「查看程序 → 识别程序」里也能补 |
| 版本管理 | 每次上传产生新版本，可回滚、可两版逐行对比、可下载 |
| 程序查看与编辑 | 在软件里直接打开 NC 程序查看；可在线修改，保存时可选「另存为新版本」（留历史）或「覆盖当前版本」（不留历史） |
| 刀具补偿表 | 序号 / 刀具号 / 刀补号 / 直径 / 补偿量 |
| 搜索 | 图纸号、名称、客户、材料；也支持用程序号反查图纸 |
| **版本检查** | 启动时问一次 GitHub 有没有新版本，有就在左下角版本号旁标出「有新版本 vX.Y.Z」 |
| **离线安装** | 在界面里选中下载好的离线包即可升级：校验 → 换 exe 和前端 → 自动重启。数据不动 |
| 级联删除 | 删图纸连带清掉其下工序、程序、版本引用 |
| 操作日志 | 记录新增、修改、上传、切版、删除（数据库与 API 已提供，界面暂未展示） |
| 字典表 | 刀具字典、机台字典（表结构与 API 已提供，当前界面未使用） |

一期**刻意没有做登录**，打开即用。数据库里已经留好用户表与操作日志的位置。

### 界面结构

单页线性表单，没有左树、没有页签，从上到下就是现场的工艺顺序：

```
左侧：图纸平铺列表（搜索 + 点选）
右侧：
  图纸        图纸号 / 名称 / 材料 / 备注
    └ 工序        工序号 10# 起，装夹方式 / Z 轴高度垫高 / 备注
        └ 程序        程序号（如 O1234），一道工序可挂多个
            └ 刀具补偿    序号 / 刀具号 / 刀补号 / 直径 / 补偿量
```

- 装夹方式、Z 轴垫高、备注挂在**工序**上——它们描述「这道工序怎么装夹」，
  同一装夹下的多个程序共享
- 工序号显示成 `10#`，输入时带不带 `#` 都接受
- NC 上传、查看程序、版本历史都收在弹窗里，不占主页面

---

## 目录结构

```
backend\                          Go 后端（默认只提供 JSON 接口；设了 CNC_WEB_DIR 才顺带托管前端）
├─ cmd\server\main.go             程序入口
├─ cmd\dbcheck\main.go            数据库巡检与在线备份工具
└─ internal\
   ├─ config\                     环境变量配置
   ├─ domain\                     实体、请求模型、领域错误（不依赖任何框架）
   ├─ store\                      SQLite 连接、PRAGMA、内嵌迁移执行器
   │  └─ migrations\              0001_init.sql 等，编译进二进制
   ├─ repo\                       所有 SQL 都在这里，手写，无 ORM
   ├─ ncstore\                    NC 文件库：sha256 内容寻址、去重、编码转换
   ├─ ncparse\                    从 NC 正文识别数控系统与刀具调用
   ├─ diff\                       NC 程序逐行对比
   ├─ update\                     版本检查（问 GitHub）与离线安装（校验、替换脚本）
   └─ httpapi\                    路由、参数校验、错误映射、响应编码
                                  web.go 是可选的静态托管（含 SPA 回落与缓存头）

frontend\                         Vue 3 + TypeScript + Vite + Element Plus
├─ src\api\                      接口封装与类型定义
├─ src\composables\              组合式逻辑（useUpdate：查新版本、装离线包）
├─ src\components\               DrawingList 图纸列表 / DrawingCard 图纸卡片
│                                OperationCard 工序卡片 / ToolTable 刀具补偿表
│                                VersionDialog 版本历史 / ProgramEditorDialog 查看程序
│                                UpdateDialog 版本更新与离线安装
└─ src\views\MainView.vue        主界面

scripts\                          运行项目所需的脚本
├─ run-backend.cmd               启动后端
├─ build-release.cmd             打包 Windows 免安装版（产物在 .local\release\）
└─ seed-demo.cmd                 灌入演示数据（仅试用，正式使用不要跑）

VERSION                          版本号真源
```

运行期数据全部在 `backend\data\`（不纳入版本控制）：

```
backend\data\
├─ cnccool.db        SQLite 数据库
├─ cnccool.db-wal    预写日志（WAL 模式）
└─ nc\               NC 文件库
   └─ a3\f9\a3f9....nc    按内容哈希分层存放
```

---

## 快速开始

两种方式，任选一种。

### 方式一：免安装包（现场推荐）

从 [Releases](https://github.com/herozmy/CNC-Manager/releases) 下载
`cnccool-v0.06-windows-amd64.zip`，解压到任意目录，双击 `start.cmd`。
浏览器会自动打开 <http://127.0.0.1:8080>。

换端口：`start.cmd 8090`。

包里只有这些东西，**不需要装 Go、Node，也不需要单独装数据库**：

```
cnccool-server.exe   后端服务，同时提供接口和前端页面
web\                 前端静态文件
start.cmd            启动脚本
README.txt           随包使用说明
```

数据全部落在包内 `data\` 目录（`cnccool.db` + `nc\`），
**拷走整个文件夹就是一份完整备份**。

左下角版本号旁边出现「有新版本 vX.Y.Z」时，点它就能在界面里装新的离线包，
不用手工替换文件——详见[版本升级](#版本升级)。

自己打包：

```
scripts\build-release.cmd
```

产物在 `.local\release\`。脚本读取 `VERSION` 注入版本号，编译后端、构建前端，
拼出免安装目录后压成 zip，结束时打印 SHA256。

> 免安装包之所以一个进程就够了，靠的是环境变量 `CNC_WEB_DIR`：
> 设上它，后端会顺带把前端静态文件托管出去（`/api/**` 仍然是接口，
> 其余路径走前端，静态资源长缓存、`index.html` 不缓存）。
> 开发时不设这个变量，前后端保持分离，前端照旧由 Vite 提供、照旧热更新。
>
> 包内的 `VERSION` 文件不是摆设：离线安装要先读它才知道装的是哪一版，
> 读到比当前版本旧就直接拒绝。少了它的包会被后端挡下来。

### 方式二：从源码运行

#### 环境要求

- Windows 10 / 11
- 后端：Go 1.22 或更高；若使用项目自带的便携版 Go（`.tools\go\`）则无需安装
- 前端：Node.js 18+（验证版本 v24.21.0）

#### 1. 启动后端

```
scripts\run-backend.cmd
```

看到下面这行就成功了：

```
CNC 加工程序管理服务已启动 地址=http://127.0.0.1:8080 ...
```

首次启动会自动建库、建表，**不需要任何手工步骤**。

#### 2. 启动前端

**另开一个窗口**：

```
frontend\dev.cmd
```

浏览器打开 **http://127.0.0.1:5173**

前端也可以单独构建成静态文件部署：

```
frontend\build.cmd      产物在 frontend\dist
frontend\preview.cmd    本地预览构建产物
```

#### 3. 开始录入

首次启动数据库是空的，界面上会提示「还没有图纸，先新建图纸」。
直接点「+ 新建图纸」按现场流程录入即可，**不需要任何演示数据**。

如果想先看看软件长什么样：

```
scripts\seed-demo.cmd
```

它会创建 2 个图纸 / 3 道工序 / 4 个程序，带刀具补偿表和 NC 版本。
正式使用前请停掉后端、删掉 `backend\data\` 整个目录再重启，即可回到空库。

---

## 配置项

后端全部通过环境变量配置，代码中无硬编码路径。

| 变量 | 默认值 | 说明 |
|---|---|---|
| `CNC_ADDR` | `127.0.0.1:8080` | 监听地址。局域网访问改成 `0.0.0.0:8080` |
| `CNC_DATA_DIR` | `data` | 数据根目录 |
| `CNC_DB_PATH` | `<data>/cnccool.db` | 数据库文件 |
| `CNC_NC_DIR` | `<data>/nc` | NC 文件库 |
| `CNC_WEB_DIR` | 空 | 前端静态文件目录。设上则同一端口顺带托管前端（免安装包模式）；留空则只提供接口，前端交给 Vite / nginx |
| `CNC_MAX_UPLOAD_MB` | `64` | 单个 NC 文件上传上限 |
| `CNC_CORS_ORIGINS` | `http://127.0.0.1:5173,...` | 允许跨域的前端地址 |
| `CNC_UPDATE_REPO` | `herozmy/CNC-Manager` | 查新版本用的 GitHub 仓库 `owner/name`。**留空则不检查更新**，服务完全不去连外网 |
| `CNC_UPDATE_API` | GitHub 官方地址 | 仓库 API 地址。内网自建 GitHub 或测试时改它 |
| `CNC_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

> `CNC_ADDR` 改成 `0.0.0.0:8080` 之后局域网上的机器就能访问了。但要注意
> `POST /api/update/install` 能把 exe 换掉，等于一个"随意执行代码"的口子，
> 所以那个接口**只接受来自本机的请求**，局域网上的机器调用会拿到 403。
> 这是刻意的：数据可以共享，程序只能在本机换。

---

## 技术选型

| 层 | 选择 | 说明 |
|---|---|---|
| 后端语言 | Go 1.27 | 编译成单文件可执行程序，无运行时依赖 |
| HTTP 路由 | `go-chi/chi` v5 | 标准库 `net/http` 兼容 |
| 数据库 | SQLite | 零安装、单文件、免运维 |
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go 实现，**不需要 cgo / gcc** |
| SQL 层 | `jmoiron/sqlx` + 手写 SQL | 不用 ORM，SQL 全部可见可改 |
| 数据库迁移 | 自研内嵌迁移器 | 迁移脚本用 `//go:embed` 编进二进制，启动时自动执行 |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus | 与后端完全解耦 |
| 通信 | REST + JSON | 无状态 |

**前端不打包进后端**：后端只提供 `/api/**`，前端由 Vite（开发）或 nginx（生产）
单独提供。这样改界面只需替换静态文件，Go 服务和数据库完全不动；
换数据库只需改 `repo` 层。

---

## 数据模型

```
drawing 图纸          drawing_no(唯一) name customer material drawing_version remark
  └ operation 工序    drawing_id op_no(10/20/30) op_name machine_id fixture
                      z_height_milli(Z轴垫高) remark    UNIQUE(drawing_id, op_no)
      └ nc_program 程序   operation_id program_no program_name controller
                          current_version_id remark
          ├ nc_version 版本   program_id version_no file_id file_name change_note created_by
          └ program_tool 刀具补偿
                              program_id seq tool_no(T) offset_no(D/H) tool_name
                              tool_dia_milli corner_radius_milli comp_amount_milli
                              spindle_speed speed_mode feed_milli feed_mode
                              cut_depth_milli coolant machining_content remark
                              UNIQUE(program_id, seq)

nc_file 文件库        sha256(唯一) size_bytes original_name rel_path file_encoding
tool 刀具字典          tool_no name spec tool_type
machine 机台字典       code name controller
audit_log 操作日志     action entity_type entity_id detail actor
app_setting 通用配置    key_name value_text
```

**约定**：所有带小数的尺寸、进给、切深以「实际值 × 1000」的整数落库
（字段名以 `_milli` 结尾），避免 SQLite 浮点误差；
时间统一存 UTC + RFC3339 文本。

---

## 接口一览

所有接口以 `/api` 为前缀。出错时返回非 2xx，响应体固定为 `{"error":"中文提示"}`。

```
GET    /api/meta                              服务元信息（版本、数据目录）
GET    /api/tree?q=                           图纸→工序→程序 三级一次性返回

GET    /api/drawings?q=&page=&size=           分页搜索图纸
POST   /api/drawings                          新建图纸
GET    /api/drawings/{id}
PUT    /api/drawings/{id}
DELETE /api/drawings/{id}                     级联删除下级
GET    /api/drawings/{id}/detail              界面主视图：一次拿到 图纸→工序→程序→刀具补偿表
GET    /api/drawings/{id}/operations
POST   /api/drawings/{id}/operations

PUT    /api/operations/{id}
DELETE /api/operations/{id}
GET    /api/operations/{id}/programs
POST   /api/operations/{id}/programs

GET    /api/programs/{id}                     详情：基本信息 + 图纸 + 工序 + 刀具表 + 版本列表
PUT    /api/programs/{id}
DELETE /api/programs/{id}
GET    /api/programs/{id}/tools
PUT    /api/programs/{id}/tools               整表替换
GET    /api/programs/{id}/versions
POST   /api/programs/{id}/versions            multipart：file + changeNote
                                              响应里附带 parse：识别到的刀具调用
POST   /api/programs/{id}/versions/content    编辑后「另存为新版本」
PUT    /api/programs/{id}/current-version     切换 / 回滚当前版本
GET    /api/programs/{id}/logs

GET    /api/versions/{id}/download            下载（?inline=1 内联预览）
GET    /api/versions/{id}/diff?against={旧版id}  逐行对比
GET    /api/versions/{id}/content             查看程序：返回解码成 UTF-8 的文本
PUT    /api/versions/{id}/content             覆盖这一版的内容（版本号不变）

GET    /api/tools?q=      POST /api/tools      PUT/DELETE /api/tools/{id}
GET    /api/machines      POST /api/machines   PUT/DELETE /api/machines/{id}

POST   /api/nc/parse                          解析一段 NC 文本，识别刀具调用

GET    /api/update/check?fresh=1              问 GitHub 上有没有新版本（fresh 绕过缓存）
POST   /api/update/install                    multipart：file，安装离线包
                                              只接受来自本机的请求（见下）
```

`GET /api/drawings/{id}/detail` 是界面主视图用的接口：
选中一张图纸只发这一个请求，就能拿到整页要渲染的全部数据。

### 刀具识别

上传 NC 后，`ncparse` 会从程序正文里读出数控系统与刀具调用。

**换刀的含义在车床和加工中心上不一样**，所以先看程序里有没有 `M06` 来分流：

| 程序 | 规则 |
|---|---|
| **有 `M06`**（加工中心） | 只有**和 `M06` 同行**的 `T` 才算真正换刀。单独一行的 `T` 是「备刀」——把刀转到待换位置、还没换上去，不算。跳过了哪些备刀会明确提示，免得以为漏识别 |
| **没有 `M06`**（车床） | `T` 命令本身就是换刀（`T0101` 直接换到 1 号刀），所有 `T` 都算 |

> 为什么要分流：加工中心里 `T1 M06` 才算换到 T1，而单独一行 `T2` 只是备刀；
> 但车床程序根本没有 `M06`，若也按这条规则处理，车床程序会一把刀都识别不出来。

刀补号的取法：

- `T0101` → 刀号 01、刀补 D01（FANUC 车床的四位写法）
- `T01 D01` → 同一行给出刀补
- 单独一行的 `G43 H01` / `G41 D01` → 补到当前这把刀上（铣床几乎都这么写）
- 同一把刀出现多个不同刀补号（如 `G43 H01` 与 `G41 D01`）时明确提示，
  取第一个并让人工确认，而不是悄悄丢掉一个

识别结果随上传响应一起返回，在界面上是一张**可编辑的清单**：
识别到什么只是初稿，确认之前可以改刀具号、改刀补号、删掉多出来的、补上漏掉的，
点「加入刀具补偿表」时才落库。省掉手工敲一遍，也避免敲错——
现场的程序写法五花八门，识别偶尔看走眼很正常，所以必须留一个能改的口子。

上传那一次没顺手加也没关系：「查看程序」里的「识别程序」用的是同一个面板，
随时可以回去补。加入时按刀具号去重（`T1` 与 `T01` 视为同一把），
已存在的会跳过并告知，不会重复堆。

解析偏保守：**宁可少识别，不要乱识别**。注释里出现的 `T` 号会被正确忽略
（FANUC 的圆括号注释、Siemens 的分号注释都处理了）。识别不到就返回空，
由界面提示人工填写，绝不猜。

**程序号不参与识别**，由用户手工填写。现场的程序号写法五花八门
（`O1234` / `1234` / `O01234` / Siemens 的 `%_N_名称_MPF`），
机器判断不如人一眼看得准，误报还会打断正常的上传流程。

---

## 运维

### 数据库巡检

```
cd backend
go run ./cmd/dbcheck
```

打印库文件体积、已应用的迁移版本、各业务表记录数、NC 文件库占用，
以及无人引用的「孤儿文件」数量。

也可以直接查数据：

```
go run ./cmd/dbcheck -sql "SELECT id, op_no, fixture, z_height_milli FROM operation"
```

### 备份

数据库开了 WAL 模式，**最新提交的数据可能还在 `cnccool.db-wal` 里**，
所以服务运行中只拷 `cnccool.db` 会丢数据。正确做法二选一：

**停服务再拷**：停掉后端（Ctrl+C 会触发优雅关闭并把 WAL 合并回主库），
然后整个拷走 `backend\data\`。

**在线备份**（服务不用停）：

```
cd backend
go run ./cmd/dbcheck -backup "D:\backup\cnccool-20260928.db"
```

底层用 SQLite 的 `VACUUM INTO`，由数据库保证一致性。

> 两种方式都记得把 `backend\data\nc\` 一并拷走——那是 NC 程序文件本体。

### 版本升级

**在界面里升级（免安装版）**

左下角版本号旁边出现「有新版本 vX.Y.Z」时，点它打开对话框，
从发布页下载新的 `cnccool-vX.Y.Z-windows-amd64.zip`，再在对话框里选中它，
点「安装并重启」即可。服务会自己换掉 exe 和 `web\`、自己重启，`data\` 一动不动。

上不了外网的机器就在别的电脑上下载好，用 U 盘拷过来，同样是在这个对话框里选文件。

升级过程中服务必然要退出一次——Windows 上运行中的 exe 是锁着的，不退出就换不了。
所以后端会：

1. 校验上传的包（必须是完整免安装包、带 `VERSION`、版本不低于当前）
2. 解压到安装目录下的 `.update\stage\`
3. 生成 `.update\apply-update.cmd` 并以**脱离控制台**的方式拉起它
4. 自己退出，退出码 `99`（`start.cmd` 认这个码：说明是升级重启，不停在 pause 上）

替换脚本等旧进程释放 exe 后，用**改名**（而不是逐个覆盖）把 `web\` 换掉——
改名快且基本不会中途失败，失败时旧的还在 `web.old`，可以退回去。
替换完它会用 `start.cmd` 重新拉起服务，并沿用原来的端口。

> 那个脚本里**一条管道都没有**（不用 `tasklist | find` 判断进程是否退出）。
> 它是被"无控制台"方式拉起来的，实测管道会卡死：`find` 迟迟等不到管道关闭，
> 安装就停在那里不动，用户看到的现象是"装完打不开了"。
> 所以判断方式改成直接重试真正要做的那个操作——反正真正关心的只是"文件能不能换"。

升级没生效时看安装目录下的 `.update\apply-update.log`，里面记了每一步。

**从源码升级**

1. 先备份
2. 停掉后端
3. 改 `VERSION` 文件里的版本号（发新版本时才需要）
4. 重新构建并启动

启动时会**自动应用新增的数据库迁移**（迁移脚本编译在二进制里），
原有数据全部保留。迁移只增不改，执行过的记进 `schema_migration` 表，
且整体在事务里执行，失败会回滚。

前端也可以单独升级：跑 `frontend\build.cmd`，替换 `dist` 即可，后端和数据库不动。

### 检查更新会不会联网

会，但只在两处：启动时查一次 GitHub 的 `releases/latest`，以及你在对话框里点
「重新检查」时。查的是公开接口，不发任何本地数据出去。结果缓存 30 分钟，
避免多台机器共用出口 IP 撞上 GitHub 的限流。

**离线车间可以把 `CNC_UPDATE_REPO` 设成空**，服务就完全不去连外网，
界面上的更新入口自然也不会出现。查不到时不报错、不弹窗——
没网是常态，不该为这件事打扰正在干活的人。

---

## 部署到 Docker

架构已经为此准备好：无 cgo、配置全走环境变量、无硬编码绝对路径、
数据全在单一目录下、数据库只存相对路径。

```yaml
# docker-compose.yml（示意）
services:
  db:
    image: postgres:16-alpine
    environment: { POSTGRES_DB: cnccool, POSTGRES_PASSWORD: ${DB_PASSWORD} }
    volumes: [pg-data:/var/lib/postgresql/data]

  api:
    image: cnccool/api:0.01           # Go 静态二进制，CGO_ENABLED=0
    environment:
      CNC_ADDR: 0.0.0.0:8080
      CNC_DATA_DIR: /data
    volumes: [nc-data:/data/nc]        # NC 文件库必须挂卷
    depends_on: [db]

  web:
    image: nginx:alpine                # 只放 frontend/dist，可独立热更新
    ports: ["80:80"]
    volumes: [./frontend/dist:/usr/share/nginx/html:ro]
volumes: { pg-data:, nc-data: }
```

如果不需要「前端独立热更新」，上面这个 `web` 服务可以直接不要：
把构建好的 `dist` 放进 api 镜像，再设 `CNC_WEB_DIR=/app/web`，一个容器就够了。
缓存策略后端已经内置（`index.html` 走 `no-store`、`/assets/*` 走 `immutable`），
不用再写 nginx 规则。

迁移时要做的三件事：

1. **换数据库**：`repo` 层把 SQLite 方言改成 PostgreSQL
   （`INTEGER PRIMARY KEY` → `bigint generated by default as identity`、
   时间文本 → `timestamptz`、`_milli` 整数 → `numeric`），表结构与业务代码不动
2. **挂卷**：数据库和 `nc-data` 必须挂出来，容器本身无状态
3. **前端交付方式**：要么用 nginx 单独放 `dist`（`index.html` 设 `no-store`、
   `/assets/*` 设 `immutable`，替换 `dist` 即可热更新），
   要么设 `CNC_WEB_DIR` 让后端一起托管（缓存头已经带好）

---

## 后续计划

按现场价值排序：

1. **导出到机床** —— 浏览器下载已可用；直连 U 盘 / 网络共享 / 串口 DNC 需要额外的本地小程序
2. **Excel 台账导入** —— 把现有台账一次性搬进来
3. **登录与权限** —— 表已就位，加用户表与鉴权即可
4. **孤儿文件回收** —— 定期清理 `nc_file` 里没人引用的物理文件
5. **按机台筛选程序** —— 车间想知道「这台机床今天要跑哪些程序」
6. **程序内容校验** —— 在已识别的刀具基础上，检查程序里调用的刀具是否都在刀具补偿表里
7. **操作日志界面** —— 数据库与接口都已就绪，只差一个展示面板
8. **刀具字典接入界面** —— 接口齐全，目前刀具靠手输或从识别结果生成
