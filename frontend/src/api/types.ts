/**
 * 后端 API 契约类型定义。
 *
 * 字段名与后端真实返回严格一致（已用 curl.exe 逐个接口核对过），禁止自行改名。
 *
 * 单位约定（现场最容易出错的地方，务必看清）：
 *   - toolDia / cornerRadius / cutDepth / zHeight 单位 mm
 *   - spindleSpeed 按 speedMode 解释：0 = G97 恒转速(r/min)，1 = G96 恒线速(m/min)
 *   - feed        按 feedMode  解释：0 = G94 每分钟(mm/min)，1 = G95 每转(mm/r)
 */

/** 服务元信息 */
export interface Meta {
  version: string
  serverTime: string
  dataDir: string
  /**
   * 能不能自己安装离线包。
   *
   * 只有「exe 旁边就是 web 目录」的免安装版布局才为 true；
   * 开发模式下为 false，界面据此把安装入口藏起来，
   * 而不是让用户点了才发现不支持。
   */
  canInstall: boolean
}

export interface AuthStatus {
  setupRequired: boolean
}

export interface AuthUser {
  id: number
  username: string
  displayName: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface LoginInput {
  username: string
  password: string
}

export interface SetupInput extends LoginInput {
  displayName: string
}

/* ---------------------------------------------------------------- 版本更新 */

/** GET /api/update/check 的结果 */
export interface UpdateStatus {
  current: string
  latest: string
  hasUpdate: boolean
  releaseUrl: string
  publishedAt: string
  assetName: string
  assetUrl: string
  assetSize: number
  checkedAt: string
  /**
   * 查不到时的原因（没网、限流、仓库不存在等）。
   *
   * 这不是「错误」：车间没网是常态，界面上安静处理即可，不该弹红叉吓人。
   */
  error: string
}

/** POST /api/update/install 的结果 */
export interface InstallResult {
  /** 装上去的新版本 */
  version: string
  /** 装之前的版本 */
  previous: string
  restarting: boolean
}

/** 通用分页结构 */
export interface Page<T> {
  items: T[]
  total: number
  page: number
  size: number
}

/* ------------------------------------------------------------------ 图纸 */

export interface Drawing {
  id: number
  drawingNo: string
  name: string
  customer: string
  material: string
  drawingVersion: string
  remark: string
  createdAt: string
  updatedAt: string
  operationCount?: number
}

/**
 * 图纸提交体。
 *
 * 注意：后端 PUT 是**整体替换**语义，没带的字段会被写成零值。
 * 界面上不显示的 customer / drawingVersion 必须用「加载到的原值」带回，
 * 不能填死空串，否则会把后端已有数据覆盖掉（详见各卡片组件里构造提交体的地方）。
 */
export interface DrawingInput {
  drawingNo: string
  name: string
  customer: string
  material: string
  drawingVersion: string
  remark: string
}

/* ------------------------------------------------------------------ 工序 */

export interface Operation {
  id: number
  drawingId: number
  opNo: number
  opName: string
  machineId: number | null
  machineName: string
  fixture: string
  /** Z 轴高度垫高，单位 mm */
  zHeight: number
  remark: string
  createdAt: string
  updatedAt: string
  programCount?: number
}

/** 工序提交体；opName / machineId 界面上不显示，提交时必须带回原值 */
export interface OperationInput {
  opNo: number
  opName: string
  machineId: number | null
  fixture: string
  zHeight: number
  remark: string
}

/* ------------------------------------------------------------------ 程序 */

export interface Program {
  id: number
  operationId: number
  programNo: string
  programName: string
  controller: string
  currentVersionId: number | null
  currentVersionNo: number | null
  versionCount: number
  remark: string
  createdAt: string
  updatedAt: string
}

/** 程序提交体；programName / controller / remark 界面上不显示，提交时必须带回原值 */
export interface ProgramInput {
  programNo: string
  programName: string
  controller: string
  remark: string
}

/* -------------------------------------------------------------- 刀具刀补 */

/**
 * 刀具补偿行（后端返回体）。
 *
 * 后端的行对象里还有 id / programId，界面上用不到（整表提交会重建这些行），
 * 因此这里不声明，避免误用。
 */
export interface ProgramTool {
  seq: number
  toolId: number | null
  toolNo: string
  offsetNo: string
  toolName: string
  toolDia: number
  cornerRadius: number
  /** 刀具补偿量，单位 mm */
  compAmount: number
  spindleSpeed: number
  /** 0 = G97 恒转速(r/min)，1 = G96 恒线速(m/min) */
  speedMode: number
  feed: number
  /** 0 = G94 每分钟(mm/min)，1 = G95 每转(mm/r) */
  feedMode: number
  cutDepth: number
  /** 0=无，1=冷却液，2=吹气，3=喷雾 */
  coolant: number
  machiningContent: string
  remark: string
}

/**
 * 刀具补偿行（提交体）。字段与 ProgramTool 相同。
 *
 * 界面上只编辑 seq / toolNo / offsetNo / toolDia / compAmount，
 * 其余列（toolName、cornerRadius、spindleSpeed、speedMode、feed、feedMode、
 * cutDepth、coolant、machiningContent、remark、toolId）
 * 在整表提交时**必须按加载到的原值带回**，不能填死默认值，否则会丢数据。
 */
export interface ProgramToolInput {
  seq: number
  toolId: number | null
  toolNo: string
  offsetNo: string
  toolName: string
  toolDia: number
  cornerRadius: number
  /** 刀具补偿量，单位 mm */
  compAmount: number
  spindleSpeed: number
  speedMode: number
  feed: number
  feedMode: number
  cutDepth: number
  coolant: number
  machiningContent: string
  remark: string
}

/* ------------------------------------------------------------------ 版本 */

export interface Version {
  id: number
  programId: number
  versionNo: number
  fileId: number
  fileName: string
  fileSize: number
  sha256: string
  changeNote: string
  createdBy: string
  createdAt: string
  isCurrent: boolean
}

/**
 * 某个版本的程序文本（GET /api/versions/{id}/content）。
 *
 * content 已由后端按源文件编码解码成 UTF-8，界面直接显示即可。
 */
export interface VersionContent {
  versionId: number
  programId: number
  versionNo: number
  fileName: string
  /** 'utf-8' 或 'gbk' */
  encoding: string
  /** 已经解码成 UTF-8 的完整文本 */
  content: string
  lineCount: number
  sizeBytes: number
  isCurrent: boolean
}

/**
 * 保存程序文本的请求体（PUT 覆盖当前版本 / POST 另存为新版本）。
 *
 * encoding 必须把读到的 VersionContent.encoding **原样带回**：
 * 写死 'utf-8' 会把现场的 GBK 程序毁掉，机床可能直接不认。
 */
export interface ContentInput {
  content: string
  encoding: string
  changeNote: string
}

/* ------------------------------------------------------------ 程序自动识别 */

/** 从 NC 程序正文里识别到的一次刀具调用 */
export interface ToolUse {
  seq: number
  /** 刀具号，如 "T01" */
  toolNo: string
  /** 刀补号，如 "D01" / "H01"；识别不到时是**空串** */
  offsetNo: string
  /** 出现在第几行，便于人工核对 */
  lineNo: number
  /** 该行原文，供人工核对 */
  raw: string
}

/**
 * 一段 NC 文本的识别结果（POST /api/nc/parse 与上传接口的 parse 字段共用）。
 *
 * 只做**刀具识别**：程序号一律由人工输入记录，后端不再返回、前端也不做对比。
 */
export interface ParseResult {
  /** 推测的数控系统："FANUC" / "SIEMENS" / 空串 */
  controller: string
  /** 识别到的刀具调用，可能是空数组 */
  tools: ToolUse[]
  lineCount: number
  /** 需要人工看一眼的提醒，如「没有识别到刀具调用」「同一把刀出现多个刀补号」 */
  warnings: string[]
}

/**
 * 上传 NC 新版本的返回体：版本信息 + 正文识别结果。
 *
 * 注意：后端该字段带 `omitempty` 标签，读取/解码文件失败时**整个 parse 字段都不会出现**
 * （不是 null）。所以这里声明成可选，取用时一律写成 `result.parse ?? null`。
 */
export interface UploadVersionResult extends Version {
  parse?: ParseResult | null
}

/* ------------------------------------------------------------------ 对比 */

export interface DiffLine {
  type: 'same' | 'add' | 'del' | 'change'
  leftNo: number | null
  rightNo: number | null
  leftText: string
  rightText: string
}

export interface DiffResult {
  leftVersionNo: number
  rightVersionNo: number
  identical: boolean
  lines: DiffLine[]
}

/* ------------------------------------------------------------------ 机台 */

/** 机台字典。界面上刻意不展示机台选择，仅在提交工序时原样带回 machineId。 */
export interface Machine {
  id: number
  code: string
  name: string
  controller: string
  remark: string
}

/* ------------------------------------------------- 图纸详情（整页一次拉全） */

/** 图纸详情里的程序节点，直接带上它的刀具补偿表 */
export interface DetailProgram {
  id: number
  programNo: string
  programName: string
  controller: string
  currentVersionId: number | null
  currentVersionNo: number | null
  versionCount: number
  remark: string
  tools: ProgramTool[]
}

/** 图纸详情里的工序节点 */
export interface DetailOperation {
  id: number
  opNo: number
  opName: string
  machineId: number | null
  machineName: string
  fixture: string
  zHeight: number
  remark: string
  programs: DetailProgram[]
}

/** 一整张图纸：图纸 → 工序 → 程序 → 刀具补偿表 */
export interface DrawingDetail {
  drawing: Drawing
  operations: DetailOperation[]
}
