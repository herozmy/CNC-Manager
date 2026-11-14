export interface User {
  id: number
  username: string
  displayName: string
  role: 'admin' | 'editor' | 'viewer'
  enabled: boolean
}

export interface Drawing {
  id: number
  drawingNo: string
  /** 产品类型；沿用 name 字段以兼容已有数据。 */
  name: string
  customer: string
  material: string
  drawingVersion: string
  remark: string
  createdAt: string
  updatedAt: string
  operationCount: number
  programCount: number
}

export interface DrawingInput {
  drawingNo: string
  /** 产品类型。 */
  name: string
  customer: string
  material: string
  drawingVersion: string
  remark: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  size: number
}

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
}

export interface Operation {
  id: number
  drawingId: number
  opNo: number
  opName: string
  machineId: number | null
  machineName: string
  fixture: string
  centeringMethod: string
  xCoordinate: number
  yCoordinate: number
  /** Z 坐标，沿用旧 API 字段名。 */
  zHeight: number
  remark: string
  programs: Program[]
}

export interface DrawingDetail extends Drawing {
  operations: Operation[]
}

export interface Meta {
  version: string
}

export interface OperationInput {
  opNo: number
  opName: string
  machineId: number | null
  fixture: string
  centeringMethod: string
  xCoordinate: number
  yCoordinate: number
  zHeight: number
  remark: string
}

export interface ProgramInput {
  programNo: string
  programName: string
  controller: string
  remark: string
}

export interface Version {
  id: number
  programId: number
  versionNo: number
  sha256: string
  fileName: string
  fileSize: number
  encoding: 'utf-8' | 'gbk'
  changeNote: string
  createdBy: string
  createdAt: string
  isCurrent: boolean
}

export interface ToolUse {
  seq: number
  toolNo: string
  offsetNo: string
  lineNo: number
  raw: string
}

export interface ParseResult {
  controller: string
  tools: ToolUse[]
  lineCount: number
  warnings: string[]
}

export interface UploadResult extends Version {
  parse?: ParseResult
}

export interface VersionContent {
	sha256: string
  versionId: number
  programId: number
  versionNo: number
  fileName: string
  encoding: 'utf-8' | 'gbk'
  content: string
  lineCount: number
  sizeBytes: number
  isCurrent: boolean
}

export interface ContentInput {
	expectedSHA256?: string
  content: string
  encoding: 'utf-8' | 'gbk'
  fileName: string
  changeNote: string
}

export interface VersionSaveResult extends Version {
  warning?: string
}

export interface DiffLine {
  type: 'same' | 'add' | 'del' | 'change'
  leftNo: number | null
  rightNo: number | null
  leftText: string
  rightText: string
}

export interface VersionDiff {
  leftVersionNo: number
  rightVersionNo: number
  identical: boolean
  truncated: boolean
  totalLines: number
  totalChanges: number
  lines: DiffLine[]
}

export interface ToolCustomParam { name: string; value: string }

export interface ProgramToolInput {
  seq: number
  toolId: number | null
  toolNo: string
  offsetNo: string
  toolName: string
  toolDia: number
  cornerRadius: number
  compAmount: number
  spindleSpeed: number
  speedMode: number
  feed: number
  feedMode: number
  cutDepth: number
  coolant: number
  machiningContent: string
  remark: string
  customParams: ToolCustomParam[]
}

export interface ProgramTool extends ProgramToolInput { id: number; programId: number; versionId: number }
export interface ToolTable { items: ProgramTool[]; revision: string }
export interface BackupPreview {
  format: string
  version: string
  createdAt: string
  sha256: string
  summary: { users: number; drawings: number; operations: number; programs: number; versions: number; tools: number; files: number; admins: string[] }
}

export interface BackupRecovery { name: string; createdAt: string; sizeBytes: number }
