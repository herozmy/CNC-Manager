/**
 * NC 自动识别里用到的**纯逻辑**：只做字符串归一化与判断，不调接口、不弹窗。
 *
 * 现在只剩**刀具识别**相关的部分（程序号一律由人工输入，不做识别也不做对比）。
 */
import type { ToolUse } from '../api/types'

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
