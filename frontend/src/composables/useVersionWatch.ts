/**
 * 盯着服务器上的前端有没有被换掉。
 *
 * 要解决的问题很具体：用户开着页面干活，我们把新版本的 `web\` 目录覆盖上去
 * （免安装版就是这么升级的），他页面里跑的还是旧的 JS，自己不会知道。
 *
 * 判据有两条，任一成立就提示刷新：
 *   1. **入口脚本地址变了** —— `web\` 被覆盖过
 *   2. **后端版本号变了** —— 整个包换过
 *
 * 为什么用入口脚本文件名当指纹：Vite 构建产物的文件名带内容哈希，前端一改
 * 名字就变，所以这个名字等价于「这一版前端」的身份。前端从 DOM 里读自己正在跑的
 * 那一个，后端从 `index.html` 里读当前要发的哪一个，两边取的是同一个值，
 * 不需要在构建时把版本号塞进代码里。
 *
 * 开发模式下后端不返回 `webEntry`（前端由 Vite 提供），只比对版本号，不会误报。
 */
import { onMounted, onUnmounted, ref, type Ref } from 'vue'
import { getMeta } from '../api'

/** 每隔多久主动问一次 */
const POLL_INTERVAL_MS = 60_000

/**
 * 本页正在跑的前端入口脚本地址，例如 `/assets/index-CpuQPsoE.js`。
 *
 * 用 DOM 取而不是用 `import.meta.url`：代码分割之后 `import.meta.url` 会指向
 * 某一个子 chunk，不一定是入口，拿它当指纹会误判。
 */
function runningEntry(): string {
  const el = document.querySelector<HTMLScriptElement>('script[type="module"][src]')
  const src = el?.getAttribute('src')
  if (!src) return ''
  try {
    return new URL(src, window.location.href).pathname
  } catch {
    return ''
  }
}

export interface VersionWatch {
  /** 后端版本号，显示在左侧面板底部 */
  version: Ref<string>
  /** 非空表示已经发现有新版在等着，内容是给用户看的提示 */
  notice: Ref<string>
  /** 立刻查一次（测试和调试用） */
  check: () => Promise<void>
  /** 刷新页面 */
  reload: () => void
}

export function useVersionWatch(): VersionWatch {
  const version = ref('')
  const notice = ref('')

  /** 页面打开时后端的版本号，之后拿它做对比 */
  let loadedVersion = ''
  let timer: number | undefined

  async function load(): Promise<void> {
    try {
      const meta = await getMeta()
      version.value = meta.version
      loadedVersion = meta.version
    } catch {
      // 版本号只是辅助信息，拿不到就留空，不影响主流程
      version.value = ''
    }
  }

  async function check(): Promise<void> {
    if (notice.value) return // 已经提示过了，不用再问
    try {
      const meta = await getMeta()

      const entry = runningEntry()
      if (meta.webEntry && entry && meta.webEntry !== entry) {
        notice.value = '系统前端文件已更新，当前页面还是旧版本。刷新后才能看到新界面。'
        return
      }
      if (loadedVersion && meta.version && meta.version !== loadedVersion) {
        notice.value = `服务已升级到 ${meta.version}，当前页面还是旧版本。刷新后才能对上。`
      }
    } catch {
      // 版本比对是锦上添花的功能，连不上后端时不要打扰用户
    }
  }

  function onFocus(): void {
    void check()
  }

  function onVisibilityChange(): void {
    if (document.visibilityState === 'visible') void check()
  }

  function reload(): void {
    window.location.reload()
  }

  onMounted(() => {
    void load()
    // 开着页面干活的人不会主动刷新，所以隔一会儿主动问一次；
    // 切回这个标签页时再立刻问一次，多数情况下提示是即时出现的。
    timer = window.setInterval(() => {
      void check()
    }, POLL_INTERVAL_MS)
    window.addEventListener('focus', onFocus)
    document.addEventListener('visibilitychange', onVisibilityChange)
  })

  onUnmounted(() => {
    if (timer !== undefined) {
      window.clearInterval(timer)
      timer = undefined
    }
    window.removeEventListener('focus', onFocus)
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return { version, notice, check, reload }
}
