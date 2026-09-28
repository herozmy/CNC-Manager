<script setup lang="ts">
/**
 * 版本更新对话框：显示仓库上的最新版本，并提供离线安装入口。
 *
 * 为什么是「离线安装」而不是「在线升级」：车间里的机器多半上不了外网。
 * 能联网的机器（或者你自己）从发布页下载好 zip，拿到这台机器上选一下就能装。
 *
 * 这个组件不直接调接口：安装动作由 MainView 统一处理，
 * 因为装完要让整个页面刷新，那件事不该由一个对话框来做。
 */
import { computed, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import type { UpdateStatus } from '../api/types'

const props = defineProps<{
  modelValue: boolean
  /** 最近一次检查的结果；没查到为 null */
  status: UpdateStatus | null
  /** 当前这套能不能自己装离线包（免安装版布局才行） */
  canInstall: boolean
  /** 当前运行的版本号 */
  currentVersion: string
  /** 正在检查 */
  checking: boolean
  /** 正在安装 */
  installing: boolean
  /** 安装过程中的提示，装完或失败后留在这里 */
  installMessage: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 请求重新检查（fresh = 绕过缓存） */
  check: [fresh: boolean]
  /** 用户选定了一个离线包，请求安装 */
  install: [file: File]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const selected = ref<File | null>(null)

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v)
})

const hasUpdate = computed(() => props.status?.hasUpdate === true)
/** 查不到时给一句人话，而不是把后端的原始错误直接甩出来 */
const checkFailed = computed(() => {
  const s = props.status
  if (!s) return ''
  if (!s.error) return ''
  return s.error
})

/** 上次检查时间，显示成「HH:MM:SS」就够用了 */
const checkedAtText = computed(() => {
  const raw = props.status?.checkedAt
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString()
})

const publishedText = computed(() => {
  const raw = props.status?.publishedAt
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString()
})

const assetSizeText = computed(() => {
  const n = props.status?.assetSize ?? 0
  if (!n) return ''
  return `${(n / 1024 / 1024).toFixed(1)} MB`
})

// 每次重新打开都清掉上一次选的文件，避免"看着是空的、其实装的是旧包"
watch(visible, (open) => {
  if (open) selected.value = null
})

function pickFile(): void {
  fileInput.value?.click()
}

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  selected.value = input.files?.[0] ?? null
}

async function confirmInstall(): Promise<void> {
  const file = selected.value
  if (!file) return

  try {
    await ElMessageBox.confirm(
      `即将安装 ${file.name}。\n\n` +
        '安装过程中服务会退出并自动重启，当前页面会短暂无法访问。\n' +
        '数据（data 目录）不会被改动。',
      '确认安装',
      { type: 'warning', confirmButtonText: '开始安装', cancelButtonText: '取消' }
    )
  } catch {
    return // 用户取消
  }
  emit('install', file)
}
</script>

<template>
  <el-dialog v-model="visible" title="版本更新" width="560px">
    <!-- 当前版本 / 最新版本 -->
    <div class="ver-grid">
      <span class="ver-label">当前版本</span>
      <span class="ver-value">{{ currentVersion || '未知' }}</span>

      <span class="ver-label">最新版本</span>
      <span class="ver-value" :class="{ highlight: hasUpdate }">
        {{ status?.latest || '—' }}
        <span v-if="hasUpdate" class="ver-tag">有新版本</span>
      </span>

      <template v-if="publishedText">
        <span class="ver-label">发布时间</span>
        <span class="ver-value muted">{{ publishedText }}</span>
      </template>
    </div>

    <p v-if="checkFailed" class="hint muted">
      没能查到最新版本：{{ checkFailed }}
    </p>
    <p v-else-if="checkedAtText" class="hint muted">上次检查：{{ checkedAtText }}</p>

    <div class="actions">
      <el-button size="small" :loading="checking" @click="emit('check', true)">
        重新检查
      </el-button>
      <el-link
        v-if="status?.releaseUrl"
        type="primary"
        :href="status.releaseUrl"
        target="_blank"
        rel="noopener"
      >
        打开发布页
      </el-link>
    </div>

    <el-divider />

    <!-- 离线安装 -->
    <template v-if="canInstall">
      <div class="section-title">离线安装</div>
      <p class="hint">
        在能联网的机器上从发布页下载
        <code>{{ status?.assetName || 'cnccool-vX.Y.Z-windows-amd64.zip' }}</code>
        （{{ assetSizeText || '约 6 MB' }}），拷到这台机器后在这里选中它即可安装。
        安装包会先校验，再替换程序和前端文件；<strong>数据不会被改动</strong>。
      </p>

      <div class="file-row">
        <el-button @click="pickFile">选择离线包…</el-button>
        <span class="file-name" :class="{ muted: !selected }">
          {{ selected ? selected.name : '还没有选择文件' }}
        </span>
        <input
          ref="fileInput"
          class="hidden-input"
          type="file"
          accept=".zip,application/zip"
          @change="onFileChange"
        />
      </div>

      <el-button
        type="primary"
        :disabled="!selected"
        :loading="installing"
        @click="confirmInstall"
      >
        安装并重启
      </el-button>
    </template>

    <p v-else class="hint muted">
      当前不是免安装版布局（可执行文件旁边没有 web 目录），不支持在这里安装离线包。
      开发模式下请用 <code>scripts\build-release.cmd</code> 重新打包后再试。
    </p>

    <p v-if="installMessage" class="install-msg">{{ installMessage }}</p>
  </el-dialog>
</template>

<style scoped>
.ver-grid {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 8px 12px;
  align-items: center;
  font-size: 13px;
}

.ver-label {
  color: var(--el-text-color-secondary);
}

.ver-value {
  color: var(--el-text-color-primary);
  font-weight: 600;
}

.ver-value.highlight {
  color: var(--el-color-primary);
}

.ver-value.muted {
  color: var(--el-text-color-regular);
  font-weight: 400;
}

.ver-tag {
  margin-left: 8px;
  padding: 1px 8px;
  border-radius: 9px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-size: 12px;
  font-weight: 400;
}

.actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 14px;
}

.hint {
  margin: 10px 0;
  color: var(--el-text-color-regular);
  font-size: 13px;
  line-height: 1.8;
}

.hint.muted {
  color: var(--el-text-color-secondary);
}

.section-title {
  margin-bottom: 4px;
  font-size: 14px;
  font-weight: 600;
}

code {
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--el-fill-color-light);
  font-size: 12px;
}

.file-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 12px 0;
}

.file-name {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-name.muted {
  color: var(--el-text-color-placeholder);
}

.hidden-input {
  display: none;
}

.install-msg {
  margin: 14px 0 0;
  color: var(--el-color-primary);
  font-size: 13px;
  line-height: 1.8;
}
</style>
