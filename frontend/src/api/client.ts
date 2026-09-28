/**
 * fetch 薄封装。
 *
 * 约定（与后端契约一致）：
 *   - 所有接口以 /api 为前缀
 *   - 出错时 HTTP 状态码非 2xx，响应体为 { "error": "错误信息" }
 *   - 204 无响应体
 *
 * 这里刻意不引入 axios，只用原生 fetch + 统一的错误处理。
 */

/** 接口前缀，生产环境由后端同源提供静态文件时同样适用 */
export const API_BASE = '/api'

/** 统一的接口错误对象：带上 HTTP 状态码与请求路径，方便页面提示与排查 */
export class ApiError extends Error {
  /** HTTP 状态码；0 表示请求根本没发出去（后端未启动 / 网络不可达） */
  readonly status: number
  /** 出错的接口路径（不含 /api 前缀） */
  readonly path: string

  constructor(message: string, status: number, path: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.path = path
  }
}

/** 把任意异常转换成可直接展示给用户的中文文案 */
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message || '未知错误'
  return '未知错误'
}

export type QueryValue = string | number | boolean | null | undefined
export type QueryParams = Record<string, QueryValue>

/** 拼接完整请求地址；空字符串 / null / undefined 的查询参数会被忽略 */
export function buildUrl(path: string, query?: QueryParams): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const url = `${API_BASE}${normalizedPath}`
  if (!query) return url
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === null || value === undefined || value === '') continue
    search.append(key, String(value))
  }
  const qs = search.toString()
  return qs ? `${url}?${qs}` : url
}

type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE'

interface RequestOptions {
  method?: HttpMethod
  query?: QueryParams
  /** JSON 请求体 */
  json?: unknown
  /** multipart/form-data 请求体（文件上传），与 json 互斥 */
  form?: FormData
  signal?: AbortSignal
}

/** 用户主动取消（AbortController）时不应该提示「无法连接后端」 */
function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

/** 解析后端错误响应体 { error: string } */
async function toApiError(response: Response, path: string): Promise<ApiError> {
  const fallback = `请求失败（HTTP ${response.status} ${response.statusText || ''}）`.trim()
  let text = ''
  try {
    text = await response.text()
  } catch {
    return new ApiError(fallback, response.status, path)
  }
  if (!text.trim()) return new ApiError(fallback, response.status, path)
  try {
    const parsed: unknown = JSON.parse(text)
    if (parsed && typeof parsed === 'object' && 'error' in parsed) {
      const raw = (parsed as { error?: unknown }).error
      if (typeof raw === 'string' && raw.trim()) {
        return new ApiError(raw.trim(), response.status, path)
      }
    }
  } catch {
    // 不是 JSON，直接把原始文本当错误信息（截断，避免弹出超长内容）
  }
  return new ApiError(text.slice(0, 500), response.status, path)
}

/** 核心请求方法：返回已解析的 JSON */
async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', query, json, form, signal } = options
  const url = buildUrl(path, query)

  const headers = new Headers({ Accept: 'application/json' })
  let body: BodyInit | undefined
  if (form) {
    // 交给浏览器自动带 boundary，绝对不能手写 Content-Type
    body = form
  } else if (json !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(json)
  }

  let response: Response
  try {
    response = await fetch(url, { method, headers, body, signal })
  } catch (error) {
    if (isAbortError(error)) throw error
    throw new ApiError(
      '无法连接后端服务，请确认后端已启动（http://127.0.0.1:8080）。',
      0,
      path
    )
  }

  if (!response.ok) throw await toApiError(response, path)

  // 204 / 空响应体
  if (response.status === 204) return undefined as unknown as T
  const text = await response.text()
  if (!text.trim()) return undefined as unknown as T

  try {
    return JSON.parse(text) as T
  } catch {
    throw new ApiError('后端返回的内容不是合法 JSON。', response.status, path)
  }
}

/** 获取纯文本响应（用于 GET /api/versions/{id}/download?inline=1 的 NC 代码预览） */
export async function apiGetText(path: string, query?: QueryParams): Promise<string> {
  const url = buildUrl(path, query)
  let response: Response
  try {
    response = await fetch(url, { method: 'GET', headers: { Accept: 'text/plain, */*' } })
  } catch {
    throw new ApiError(
      '无法连接后端服务，请确认后端已启动（http://127.0.0.1:8080）。',
      0,
      path
    )
  }
  if (!response.ok) throw await toApiError(response, path)
  return await response.text()
}

export function apiGet<T>(path: string, query?: QueryParams): Promise<T> {
  return request<T>(path, { method: 'GET', query })
}

export function apiPost<T>(path: string, json?: unknown): Promise<T> {
  return request<T>(path, { method: 'POST', json })
}

export function apiPut<T>(path: string, json?: unknown): Promise<T> {
  return request<T>(path, { method: 'PUT', json })
}

export function apiDelete(path: string): Promise<void> {
  return request<void>(path, { method: 'DELETE' })
}

/** 文件上传（multipart/form-data） */
export function apiUpload<T>(path: string, form: FormData): Promise<T> {
  return request<T>(path, { method: 'POST', form })
}
