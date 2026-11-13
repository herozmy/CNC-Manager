import { useEffect, useState } from 'react'

/** 输入停顿 delay 毫秒后才更新，避免每敲一个字就发一次请求。 */
export function useDebounced<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return debounced
}
