export interface User {
  id: number
  username: string
  displayName: string
  role: 'admin' | 'editor'
  enabled: boolean
}

export interface Drawing {
  ownerUserId: number
  ownerName: string
  id: number
  drawingNo: string
  /** 产品类型；沿用 name 字段以兼容已有数据。 */
  name: string
  customer: string
  material: string
  /** 人民币元，固定两位小数；null 表示未填写。 */
  price: string | null
  drawingVersion: string
  remark: string
  createdAt: string
  updatedAt: string
  operationCount: number
  programCount: number
}

export interface DrawingInput {
  ownerUserId?: number
  drawingNo: string
  /** 产品类型。 */
  name: string
  customer: string
  material: string
  /** 空字符串清空价格；使用十进制字符串避免金额精度丢失。 */
  price: string
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
  machineId: number | null
  machineCode: string
  machineName: string
  revision: number
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
  machineCode?: string
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
  machineId?: number | null
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
  migration?: { skippedSessions: number; orphanFiles: number; skippedSettings: number }
  summary: { users: number; drawings: number; operations: number; programs: number; versions: number; tools: number; files: number; admins: string[] }
}

export interface BackupRecovery { name: string; createdAt: string; sizeBytes: number }

export type Permission = 'read' | 'download' | 'create' | 'edit' | 'delete'
export interface UserInput { username: string; displayName: string; password: string; role: 'admin' | 'editor' }
export type UserUpdate = Pick<User, 'displayName' | 'role' | 'enabled'>
export interface AuditEntry {
  id: number; action: string; entityType: string; entityId: number | null; entityName: string; detail: string
  actor: string; actorId: number | null; actorRoleId: number | null; actorRoleName: string
  roleId: number | null; roleName: string; ownerUserId: number | null; ownerName: string; result: string; createdAt: string
}
export interface AuditFilter { userId?: number; action?: string; result?: string; entityType?: string; from?: string; to?: string; page: number; size: number }

export interface MachineOption {
  id: number
  code: string
  name: string
  enabled: boolean
}
export interface Machine extends MachineOption {
  controller: string
  remark: string
  ownerUserId: number
  ownerName: string
  drawingCount: number
  operationCount: number
  programCount: number
  createdAt: string
  updatedAt: string
}
export interface MachineInput {
  code: string
  name: string
  controller: string
  remark: string
  ownerUserId?: number
}
export interface MachineFilter { q: string; page: number; size?: number; enabled?: boolean; ownerUserId?: number }

export interface ScheduleSettings {
  retention: number
  frequency: 'off' | 'daily' | 'weekly'
  weekday: number
  time: string
}

export interface BackupSchedule extends ScheduleSettings {
  next: string | null
  lastResult: string
}
