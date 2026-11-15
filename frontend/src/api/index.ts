import { http } from './client'
import type {
  MachineOption, UserInput, UserUpdate, AuditEntry, AuditFilter, BackupPreview, BackupRecovery, ToolTable, ProgramToolInput, ParseResult, ContentInput, VersionContent, VersionSaveResult, VersionDiff, Drawing, DrawingDetail, DrawingInput, Meta, Operation, OperationInput, Page, Program, ProgramInput, UploadResult, User, Version,
} from './types'

export const api = {
  users: () => http.get<User[]>('/admin/users'),
  createUser: (input: UserInput) => http.post<User>('/admin/users', input),
  updateUser: (id: number, input: UserUpdate) => http.put<User>(`/admin/users/${id}`, input),
  resetPassword: (id: number, password: string) => http.post<void>(`/admin/users/${id}/reset-password`, { password }),
  assignDrawingOwner: (id: number, ownerUserId: number) => http.put<Drawing>(`/admin/drawings/${id}/owner`, { ownerUserId }),
  auditLogs: (filter: AuditFilter) => {
    const params = new URLSearchParams()
    Object.entries(filter).forEach(([key, value]) => { if (value !== undefined && value !== '') params.set(key, String(value)) })
    return http.get<Page<AuditEntry>>(`/admin/audit-logs?${params}`)
  },
  meta: () => http.get<Meta>('/meta'),

  authStatus: () => http.get<{ setupRequired: boolean }>('/auth/status'),
  me: () => http.get<User>('/auth/me'),
  updateMe: (displayName: string) => http.put<User>('/auth/me', { displayName }),
  changePassword: (currentPassword: string, newPassword: string) => http.put<void>('/auth/password', { currentPassword, newPassword }),
  login: (username: string, password: string) => http.post<User>('/auth/login', { username, password }),
  setup: (username: string, displayName: string, password: string) =>
    http.post<User>('/auth/setup', { username, displayName, password }),
  logout: () => http.post<void>('/auth/logout'),
  createBackup: () => http.post<{ name: string }>('/backup/create'),
  backupRecoveries: () => http.get<BackupRecovery[]>('/backup/recoveries'),
  deleteRecovery: (name: string) => http.del<void>(`/backup/recoveries/${encodeURIComponent(name)}`),
  downloadRecovery: (name: string) => http.getBlob(`/backup/recoveries/${encodeURIComponent(name)}`),
  inspectBackup: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.postForm<BackupPreview>('/backup/inspect', form)
  },
  importBackup: (file: File, sha256: string) => {
    const form = new FormData()
    form.append('file', file)
    form.append('sha256', sha256)
    form.append('confirmation', '覆盖导入')
    return http.postForm<{ recoveryName: string }>('/backup/import', form)
  },

  listDrawings: (q: string, page: number, size = 20, ownerUserId?: number) =>
    http.get<Page<Drawing>>(`/drawings?q=${encodeURIComponent(q)}&page=${page}&size=${size}${ownerUserId !== undefined ? `&ownerUserId=${ownerUserId}` : ''}`),
  drawingDetail: (id: number) => http.get<DrawingDetail>(`/drawings/${id}/detail`),
  deleteDrawing: (id: number) => http.del<void>(`/drawings/${id}`),
  createDrawing: (input: DrawingInput) => http.post<Drawing>('/drawings', input),
  updateDrawing: (id: number, input: DrawingInput) => http.put<Drawing>(`/drawings/${id}`, input),

  drawingMachines: (id: number) => http.get<MachineOption[]>(`/drawings/${id}/machines`),
  createDrawingMachine: (id: number, input: { code: string; name: string }) => http.post<MachineOption>(`/drawings/${id}/machines`, input),

  createOperation: (drawingId: number, input: OperationInput) => http.post<Operation>(`/drawings/${drawingId}/operations`, input),
  updateOperation: (id: number, input: OperationInput) => http.put<Operation>(`/operations/${id}`, input),
  deleteOperation: (id: number) => http.del<void>(`/operations/${id}`),

  getProgram: (id: number) => http.get<Program>(`/programs/${id}`),
  updateProgram: (id: number, input: ProgramInput & { expectedRevision: number }) => http.put<Program>(`/programs/${id}`, input),
  deleteProgram: (id: number) => http.del<void>(`/programs/${id}`),
  createProgram: (operationId: number, input: ProgramInput) => http.post<Program>(`/operations/${operationId}/programs`, input),

  versionTools: (id: number) => http.get<ToolTable>(`/versions/${id}/tools`),
  saveVersionTools: (id: number, items: ProgramToolInput[], expectedRevision: string) => http.put<ToolTable>(`/versions/${id}/tools`, { items, expectedRevision }),
  recognizeVersionTools: (id: number) => http.get<ParseResult>(`/versions/${id}/tools/recognize`),
  versionContent: (id: number) => http.get<VersionContent>(`/versions/${id}/content`),
  versionDiff: (left: number, right: number, offset: number, changesOnly: boolean) => http.get<VersionDiff>(`/versions/${left}/diff?with=${right}&offset=${offset}&limit=200&changesOnly=${changesOnly ? 1 : 0}`),
  saveContentAsNew: (programId: number, input: ContentInput) => http.post<VersionSaveResult>(`/programs/${programId}/versions/content`, input),
  overwriteContent: (id: number, input: ContentInput) => http.put<VersionSaveResult>(`/versions/${id}/content`, input),
  listVersions: (programId: number) => http.get<Version[]>(`/programs/${programId}/versions`),
  uploadVersion: (programId: number, file: File, changeNote: string) => {
    const form = new FormData()
    form.append('file', file)
    if (changeNote.trim()) form.append('changeNote', changeNote.trim())
    return http.postForm<UploadResult>(`/programs/${programId}/versions`, form)
  },
  setCurrentVersion: (programId: number, versionId: number) => http.put<Program>(`/programs/${programId}/current-version`, { versionId }),
  /** 下载链接：同源 Cookie 鉴权，手机浏览器可直接保存。 */
  downloadUrl: (versionId: number, encoding?: 'utf-8' | 'gbk') => `/api/versions/${versionId}/download${encoding ? `?encoding=${encoding}` : ''}`,
}
