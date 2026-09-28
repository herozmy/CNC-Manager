/**
 * 格式化工具 + 界面用到的少量业务常量。
 */

/* -------------------------------------------------------------- 工序号 */

/**
 * 工序号展示：后端存 10 / 20，界面上补成 10# / 20#。
 * 后端不认 `#`，所以提交前必须用 parseOpNo 剥掉。
 */
export function formatOpNo(opNo: number | null | undefined): string {
  if (typeof opNo !== 'number' || !Number.isFinite(opNo) || opNo <= 0) return ''
  return `${Math.round(opNo)}#`
}

/**
 * 把用户填的工序号解析成后端要的整数。
 * 允许 `10#`、`10`、`10 #`、`#10` 这些写法，只取其中的数字。
 * 解析不出来（或超出 1~9999）返回 null，由调用方提示并回滚。
 */
export function parseOpNo(text: string | null | undefined): number | null {
  const digits = (text ?? '').replace(/[^\d]/g, '')
  if (!digits) return null
  const value = Number.parseInt(digits, 10)
  if (!Number.isFinite(value) || value <= 0 || value > 9999) return null
  return value
}

/* ------------------------------------------------------------------ 数值 */

/** 空值安全的数值判断：el-input-number 清空后可能是 null，不能让它进到提交体里 */
export function toNumber(value: number | null | undefined, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

/* ------------------------------------------------------------------ 时间 */

function pad(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

/** 匹配 "2025-01-02 08:30" / "2025-01-02T08:30:00" 这类后端时间格式 */
const PLAIN_DATE_TIME = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?/

/** 格式化为 "YYYY-MM-DD HH:mm"；后端给的是 UTC，这里按字面展示不做时区换算 */
export function formatDateTime(value: string | null | undefined, placeholder = '-'): string {
  const text = (value ?? '').trim()
  if (!text) return placeholder

  const matched = PLAIN_DATE_TIME.exec(text)
  if (matched) {
    const [, y, mo, d, h, mi] = matched
    return `${y}-${mo}-${d} ${h}:${mi}`
  }

  const date = new Date(text)
  if (Number.isNaN(date.getTime())) return text
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}`
  )
}

/* ------------------------------------------------------------------ 字节 */

/** 文件大小格式化：B / KB / MB / GB */
export function formatBytes(bytes: number | null | undefined, placeholder = '-'): string {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes < 0) return placeholder
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = bytes / 1024
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(size >= 100 ? 0 : 1)} ${units[index]}`
}
