import { http } from './client'
import type {
  Drawing, DrawingDetail, DrawingInput, Meta, Operation, OperationInput, Page, Program, ProgramInput, UploadResult, User, Version,
} from './types'

export const api = {
  meta: () => http.get<Meta>('/meta'),

  authStatus: () => http.get<{ setupRequired: boolean }>('/auth/status'),
  me: () => http.get<User>('/auth/me'),
  login: (username: string, password: string) => http.post<User>('/auth/login', { username, password }),
  setup: (username: string, displayName: string, password: string) =>
    http.post<User>('/auth/setup', { username, displayName, password }),
  logout: () => http.post<void>('/auth/logout'),

  listDrawings: (q: string, page: number, size = 20) =>
    http.get<Page<Drawing>>(`/drawings?q=${encodeURIComponent(q)}&page=${page}&size=${size}`),
  drawingDetail: (id: number) => http.get<DrawingDetail>(`/drawings/${id}/detail`),
  createDrawing: (input: DrawingInput) => http.post<Drawing>('/drawings', input),

  createOperation: (drawingId: number, input: OperationInput) => http.post<Operation>(`/drawings/${drawingId}/operations`, input),
  updateOperation: (id: number, input: OperationInput) => http.put<Operation>(`/operations/${id}`, input),
  deleteOperation: (id: number) => http.del<void>(`/operations/${id}`),

  createProgram: (operationId: number, input: ProgramInput) => http.post<Program>(`/operations/${operationId}/programs`, input),

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
