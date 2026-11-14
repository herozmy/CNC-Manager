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
