import { http } from './client'
import type { Drawing, DrawingDetail, DrawingInput, Meta, Page, User } from './types'

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
}
