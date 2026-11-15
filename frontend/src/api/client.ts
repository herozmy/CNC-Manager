// 统一的接口客户端：同源 Cookie 会话，错误统一成 ApiError。

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** 任何接口返回 401 时触发，由应用根节点接收并切回登录页。 */
export const UNAUTHORIZED_EVENT = 'cnc:unauthorized'
export const FORBIDDEN_EVENT = 'cnc:forbidden'
let businessEpoch = 0
const pendingBusiness = new Set<AbortController>()
let authEpoch = 0
const pendingAuth = new Set<AbortController>()

/** Abort old requests and reject late responses even if an adapter ignores AbortSignal. */
export function cancelBusinessRequests() {
  businessEpoch += 1
  for (const controller of pendingBusiness) controller.abort()
  pendingBusiness.clear()
}

/** Session transitions also invalidate auth responses, including delayed 401s. */
export function cancelAuthRequests() {
  authEpoch += 1
  for (const controller of pendingAuth) controller.abort()
  pendingAuth.clear()
}


async function request<T>(method: string, path: string, body?: unknown, binary = false): Promise<T> {
  // FormData 由浏览器自己带 multipart 边界，不能手动设置 Content-Type
  const isForm = body instanceof FormData
  const business = !path.startsWith('/auth/') && path !== '/meta'
  const auth = path.startsWith('/auth/')
  const epoch = business ? businessEpoch : authEpoch
  const controller = new AbortController()
  if (business) pendingBusiness.add(controller)
  if (auth) pendingAuth.add(controller)
  const checkScope = () => { if ((business && epoch !== businessEpoch) || (auth && epoch !== authEpoch)) throw new DOMException('会话权限已改变', 'AbortError') }
  try {
  const res = await fetch(`/api${path}`, {
    method,
    signal: business || auth ? controller.signal : undefined,
    headers: body === undefined || isForm ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
  })
  checkScope()
  if (res.status === 204) return undefined as T
  if (res.ok && binary) { const blob = await res.blob(); checkScope(); return blob as T }

  const text = await res.text()
  checkScope()
  let data: unknown = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    // 非 JSON 响应（如网关错误页）按下面的通用提示处理
  }
  if (!res.ok) {
    const message =
      data && typeof data === 'object' && 'error' in data && typeof data.error === 'string'
        ? data.error
        : `请求失败（${res.status}）`
    // 登录/初始化接口自己的 401 是"密码错误"，不应触发全局登出
    if (res.status === 401 && !path.startsWith('/auth/login') && !path.startsWith('/auth/setup')) {
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
    }
    if (res.status === 403 && business) window.dispatchEvent(new Event(FORBIDDEN_EVENT))
    throw new ApiError(res.status, message)
  }
  return data as T
  } finally { pendingBusiness.delete(controller); pendingAuth.delete(controller) }
}

export const http = {
  getBlob: (path: string) => request<Blob>('GET', path, undefined, true),
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body ?? {}),
  postForm: <T>(path: string, form: FormData) => request<T>('POST', path, form),
  del: <T>(path: string) => request<T>('DELETE', path),
}
