export interface Meta {
  version: string
}

export async function fetchMeta(): Promise<Meta> {
  const res = await fetch('/api/meta')
  if (!res.ok) throw new Error(`请求失败：${res.status}`)
  return res.json()
}
