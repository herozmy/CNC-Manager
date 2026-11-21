const repo = 'herozmy/CNC-Manager'
const stableVersion = /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/

export async function checkRelease(current: string): Promise<{ message: string; link: string | null; target?: string }> {
  const response = await fetch(`https://api.github.com/repos/${repo}/releases/latest`, {
    signal: AbortSignal.timeout(10000), credentials: 'omit', cache: 'no-store',
  })
  if (!response.ok) throw new Error('GitHub 发布信息暂时不可用')
  const { tag_name: tag, draft, prerelease } = await response.json()
  const latest = typeof tag === 'string' && stableVersion.exec(tag)
  if (!latest || draft !== false || prerelease !== false) throw new Error('无法识别正式版本')
  const installed = stableVersion.exec(current)
  const link = `https://github.com/${repo}/releases/tag/${tag}`
  if (!installed) return { message: `当前为非正式版本，最新正式版为 ${tag}`, link }
  for (let i = 1; i <= 3; i++) {
    const diff = BigInt(latest[i]) - BigInt(installed[i])
    if (diff > 0n) return { message: `发现新版本 ${tag}`, link, target: tag }
    if (diff < 0n) return { message: `当前版本高于最新正式版 ${tag}`, link }
  }
  return { message: '已是最新正式版', link: null }
}
