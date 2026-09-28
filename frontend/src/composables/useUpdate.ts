/**
 * 版本更新：问仓库上有没有新版本，以及安装离线包。
 *
 * 和后端的分工：
 *   - 后端负责联网（浏览器直连 api.github.com 会撞上跨域），并做缓存
 *   - 后端负责校验、解压离线包，然后退出、由替换脚本换文件
 *   - 这里只管把结果呈现出来，以及等新版服务重新起来
 */
import { onMounted, ref } from 'vue'
import { checkUpdate, getMeta, installUpdate } from '../api'
import { ApiError } from '../api/client'
import type { InstallResult, UpdateStatus } from '../api/types'

/** 等旧服务停下来的上限 */
const SHUTDOWN_TIMEOUT_MS = 30_000
/** 等新服务起来的上限 */
const RESTART_TIMEOUT_MS = 60_000
/** 轮询间隔 */
const POLL_MS = 500

const sleep = (ms: number): Promise<void> => new Promise((r) => window.setTimeout(r, ms))

export function useUpdate() {
  /** 后端版本号，显示在左侧面板底部 */
  const version = ref('')
  /** 当前这套是不是免安装版布局（能不能自己装离线包） */
  const canInstall = ref(false)
  const status = ref<UpdateStatus | null>(null)
  const checking = ref(false)

  async function loadMeta(): Promise<void> {
    try {
      const meta = await getMeta()
      version.value = meta.version
      canInstall.value = meta.canInstall
    } catch {
      // 版本号只是辅助信息，拿不到就留空，不影响主流程
      version.value = ''
    }
  }

  /**
   * 查一次有没有新版本。
   *
   * 查不到（没网、限流）时后端会返回一个带 error 的结果，这里什么都不显示——
   * 车间没网是常态，不该为这件事打扰正在干活的人。
   */
  async function check(fresh = false): Promise<void> {
    checking.value = true
    try {
      status.value = await checkUpdate(fresh)
    } catch {
      status.value = null
    } finally {
      checking.value = false
    }
  }

  onMounted(() => {
    void loadMeta()
    void check()
  })

  return { version, canInstall, status, checking, check, install: installPackage }
}

/**
 * 上传离线包并等新版服务起来。
 *
 * 服务会主动退出（Windows 上运行中的 exe 锁着，不退出就没法替换），
 * 所以中途必然有一段时间连不上——那是过程，不是失败，别急着报错。
 */
async function installPackage(file: File): Promise<InstallResult> {
  const result = await installUpdate(file)
  await waitForRestart(result.version)
  return result
}

async function waitForRestart(expected: string): Promise<void> {
  // 第一步：先等它真的停下来。
  // 安装响应发出后服务还会活一小会儿（要留时间把响应发完），
  // 这时候去问版本，拿到的当然是旧的，不能因此判定失败。
  const downDeadline = Date.now() + SHUTDOWN_TIMEOUT_MS
  let wentDown = false
  while (Date.now() < downDeadline) {
    await sleep(POLL_MS)
    try {
      await getMeta()
    } catch {
      wentDown = true
      break
    }
  }
  if (!wentDown) {
    throw new Error('服务没有按预期停下，安装可能没有执行。请查看安装目录下 .update\\apply-update.log')
  }

  // 第二步：等它带着新版本回来。
  const upDeadline = Date.now() + RESTART_TIMEOUT_MS
  while (Date.now() < upDeadline) {
    await sleep(POLL_MS)
    try {
      const meta = await getMeta()
      if (meta.version === expected) return
      // 起来了但版本没变：替换没生效，多半是中途回滚了。
      // 这时候要说清楚去哪看日志，否则用户只能对着界面干瞪眼。
      throw new Error(
        `服务已恢复，但版本仍是 ${meta.version}，安装没有生效。` +
          '请查看安装目录下 .update\\apply-update.log'
      )
    } catch (error) {
      if (error instanceof ApiError) continue // 还没起来，继续等
      throw error
    }
  }
  throw new Error('等待服务重启超时。请查看安装目录下 .update\\apply-update.log')
}
