# CNC 加工程序管理系统 · 前端

Vue 3 + TypeScript + Vite + Element Plus 单页应用。
**当前版本 v0.06**（版本号的真源是仓库根目录的 `VERSION`，界面左下角显示的是后端返回的值）。

界面是**单页线性表单**：左侧图纸平铺列表，右侧按现场的工艺顺序从上到下排开——
图纸 → 工序（装夹方式 / Z 轴垫高 / 备注）→ 程序 → 刀具补偿表。
没有左树、没有页签。

## 环境要求

- Node.js ≥ 20.19（本机验证版本 v24.21.0）
- npm ≥ 10（本机验证版本 11.19.0）

> **Windows 注意**：本机只有 Windows PowerShell 5.1，执行策略默认是 Restricted。
> 调用 npm 一律用 `npm.cmd`（`npm.ps1` 会被拦住报 SecurityError）。
> 项目根目录下 `frontend\dev.cmd` / `build.cmd` / `preview.cmd` 已经封装好，直接用它们最省事。

## 常用命令

```bash
npm.cmd install        # 安装依赖
npm.cmd run dev        # 开发服务器 http://127.0.0.1:5173（保存即热更新）
npm.cmd run typecheck  # 仅类型检查（vue-tsc，应零报错）
npm.cmd run build      # 类型检查 + 生产构建，产物在 dist/
npm.cmd run preview    # 预览 dist 构建产物
```

## 与后端联调

- 开发时 Vite 把 `/api` 代理到 `http://127.0.0.1:8080`（见 `vite.config.ts`），
  前端代码统一用相对路径，不写死主机名
- 生产构建产物是纯静态文件（文件名带内容 hash），**前端不打包进后端**。
  换前端只需替换 `dist`，后端和数据库完全不用动
- 接口契约见 `src/api/types.ts` 与 `src/api/index.ts`，与后端严格对应，**不要随意改字段名**

## 目录结构

```
src/
├─ api/
│  ├─ client.ts     # fetch 薄封装：统一 /api 前缀、JSON 解析、{error} 提取、上传与文本获取
│  ├─ types.ts      # 全部接口类型定义（严格照抄后端契约）
│  └─ index.ts      # 所有接口函数
├─ utils/format.ts  # 工序号 10# ↔ 10 的转换、数值/时间/字节格式化
├─ components/
│  ├─ DrawingList.vue          # 左侧平铺图纸列表 + 搜索 + 版本号
│  ├─ DrawingCard.vue          # 图纸卡片（图纸号/名称/材料/备注）
│  ├─ OperationCard.vue        # 工序卡片（工序号/装夹方式/Z轴垫高/备注）+ 其下程序
│  ├─ ToolTable.vue            # 刀具补偿表（序号/刀具号/刀补号/直径/补偿量）
│  ├─ VersionDialog.vue        # 版本历史弹窗（上传/下载/设为当前/逐行对比）
│  └─ ProgramEditorDialog.vue  # 查看程序弹窗（只读带行号，可编辑后另存或覆盖）
├─ views/MainView.vue          # 主界面：左列表 + 右线性表单
├─ App.vue
├─ main.ts
└─ styles.css
```

## 四条必须遵守的约定

改这个前端之前请先读这四条，都是踩过坑才定下来的。

**1. 所有 `PUT` 都是整体替换，隐藏字段必须原值回传**

界面上只显示一部分字段，但数据库里有更多（刀具名称、刀尖圆弧、转速、进给、冷却方式、
加工内容、备注、机加工艺名、机台…）。提交时如果把它们填成默认值而不是加载到的原值，
用户只是改个刀具号，其它数据就被**静默清空**了。

> 验证手段：`scripts\ui-save-check.cmd` 会在真实浏览器里改一个字段、点保存、
> 抓下 PUT 请求体逐字段比对。

**2. 程序文件的 `encoding` 必须原样回传**

现场很多 NC 程序的注释是中文，而且是 GBK 编码。读取接口返回的 `encoding`
是什么，保存时就传什么。**写死 `'utf-8'` 会把现场的 GBK 程序毁掉**，
机床可能直接不认，或者中文全变问号——这个损失不可逆。

**3. 弹窗数据加载不能用 `el-dialog` 的 `@open`**

弹窗由父组件 `v-if` 挂载时，`modelValue` 在首帧已经是 `true`，
`el-dialog` 只在 visible 由 false 变 true 时才 emit `open`，**不会补发**。
正确做法是 `watch(visible, ..., { immediate: true })`。
（`ProgramEditorDialog.vue` 里踩过这个坑：弹窗能打开，但数据从没加载。）

**4. 改完要跑真实浏览器检查**

`npm run build` 通过只说明能编译，说明不了界面能用。项目根目录 `scripts\` 下有三个检查：

```
scripts\ui-check.cmd          首屏渲染 / 控制台错误 / 失败请求
scripts\ui-editor-check.cmd   真的点开「查看程序」弹窗看里面
scripts\ui-save-check.cmd     改字段保存，验证隐藏列不被清空
```

后两个需要有数据才能跑；空库时会打印「跳过」并返回 0，不会误报失败。
