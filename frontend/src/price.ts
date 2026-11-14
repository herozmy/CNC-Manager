/** 价格以十进制字符串传输，后端按分保存。 */
export function priceError(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (!/^[0-9]+(?:\.[0-9]{1,2})?$/.test(trimmed) || Number(trimmed) > 999999999.99) {
    return '价格应为 0~999999999.99 元，最多两位小数'
  }
  return ''
}
