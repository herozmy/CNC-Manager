/**
 * NC 自动识别里用到的**纯逻辑**：只做字符串归一化与判断，不调接口、不弹窗。
 *
 * 放在这里集中一份，是为了让「上传后识别」和「只读模式识别」两处走同一套判断，
 * 也方便人工核对归一化规则。
 */
import type { ToolUse } from '../api/types'

/* -------------------------------------------------------------- 程序号 */

/**
 * 程序号归一化。
 *
 * 规则**照抄后端**（backend/internal/httpapi/handlers_content.go 的 normalizeProgramNo）：
 * 现场写法不统一，O1234 / o1234 / 1234 / O01234 可能指的是同一个程序。
 * 前端如果直接用字符串比较，会把后端认为一致的程序判成不一致，反而制造误报。
 */
export function normalizeProgramNo(value: string): string {
  let text = (value ?? '').trim().toUpperCase()
  if (text.startsWith('O')) text = text.slice(1)
  if (text.startsWith(':')) text = text.slice(1)
  const trimmed = text.replace(/^0+/, '')
  // 全是 0 的情况退回原样比较，避免归一化后变成空串
  return trimmed === '' ? text : trimmed
}

/** 两个程序号是否是同一个；两边都归一化成空串时判为「不相等」 */
export function programNoEqual(a: string, b: string): boolean {
  const left = normalizeProgramNo(a)
  const right = normalizeProgramNo(b)
  return left !== '' && left === right
}

/* -------------------------------------------------------------- 警告 */

/** 后端「程序号对不上」警告里的关键字 */
export const PROGRAM_NO_MISMATCH_KEYWORD = '程序号对不上'

export function isMismatchWarning(text: string): boolean {
  return text.includes(PROGRAM_NO_MISMATCH_KEYWORD)
}

/** 识别结果里是否有「程序号对不上」这条警告 */
export function hasProgramNoMismatch(warnings: string[]): boolean {
  return warnings.some(isMismatchWarning)
}

/* -------------------------------------------------------------- 刀具 */

/**
 * 刀具号的比较键：忽略大小写、忽略 T 前缀与前缀零，即 T1 和 T01 视为同一把。
 * 归一到空串（比如填了个 "T0"）时退回去掉 T 的写法，保证不会和别的刀撞键。
 */
export function normalizeToolNo(value: string): string {
  const text = (value ?? '').trim().toUpperCase()
  const body = text.startsWith('T') ? text.slice(1) : text
  const trimmed = body.replace(/^0+/, '')
  return trimmed === '' ? body : trimmed
}

/** 刀具展示文案：有刀补号时拼成 T01/D01，识别不到刀补号就只显示 T01 */
export function formatToolLabel(tool: ToolUse): string {
  return tool.offsetNo ? `${tool.toolNo}/${tool.offsetNo}` : tool.toolNo
}
