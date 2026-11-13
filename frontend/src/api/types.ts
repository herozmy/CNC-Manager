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
