<script setup lang="ts">
/**
 * 版本历史弹窗（不占主页面）。
 *
 * 内容：上传新版本（上传后就地展示刀具识别结果）、版本列表
 * （版本号 / 文件名 / 大小 / 变更说明 / 时间）、下载、设为当前版本，
 * 以及选两个版本做逐行对比（add / del / change 三色左右分栏）。
 *
 * 程序号由人工输入记录，这里不识别也不对比；programNo 只用于标题文案。
 */
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { DiffResult, UploadVersionResult, Version } from '../api/types'
import { diffVersions, listVersions, setCurrentVersion, versionDownloadUrl } from '../api'
import { errorMessage } from '../api/client'
import { formatBytes, formatDateTime } from '../utils/format'
import NcParsePanel from './NcParsePanel.vue'
import { uploadNcVersion } from '../utils/ncUpload'

const props = defineProps<{
  modelValue: boolean
  programId: number
  programNo: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 当前版本被改动，父组件需要刷新详情 */
  changed: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

const versions = ref<Version[]>([])
const loading = ref(false)
const loadError = ref('')
const oldId = ref<number | null>(null)
const newId = ref<number | null>(null)
const diff = ref<DiffResult | null>(null)
const diffLoading = ref(false)
const switchingId = ref<number | null>(null)

// 注意：本组件由父组件的 v-if 控制挂载，首次渲染时 modelValue 已经是 true，
// 所以不能用 watch(visible) 来加载（首帧不会触发），必须挂 el-dialog 的 open 事件。

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const items = await listVersions(props.programId)
    // 后端按版本号倒序返回，这里再排一次，保证「最新」一定是第一条
    versions.value = [...items].sort((a, b) => b.versionNo - a.versionNo)
    newId.value = versions.value[0]?.id ?? null
    oldId.value = versions.value[1]?.id ?? null
    diff.value = null
  } catch (error) {
    versions.value = []
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}

const canCompare = computed(
  () => oldId.value !== null && newId.value !== null && oldId.value !== newId.value
)

async function compare(): Promise<void> {
  if (!canCompare.value || oldId.value === null || newId.value === null) return
  diffLoading.value = true
  try {
    // against 传旧版，路径里的 id 是新版
    diff.value = await diffVersions(newId.value, oldId.value)
  } catch (error) {
    diff.value = null
    ElMessage.error(errorMessage(error))
  } finally {
    diffLoading.value = false
  }
}

const diffSummary = computed(() => {
  const result = diff.value
  if (!result) return ''
  let add = 0
  let del = 0
  let change = 0
  for (const line of result.lines) {
    if (line.type === 'add') add += 1
    else if (line.type === 'del') del += 1
    else if (line.type === 'change') change += 1
  }
  return `第 ${result.leftVersionNo} 版 → 第 ${result.rightVersionNo} 版：新增 ${add} 行，删除 ${del} 行，修改 ${change} 行`
})

async function makeCurrent(version: Version): Promise<void> {
  try {
    await ElMessageBox.confirm(
      `把 ${props.programNo} 的当前生效版本切换为第 ${version.versionNo} 版（${version.fileName}）？`,
      '设为当前版本',
      { confirmButtonText: '设为当前', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  switchingId.value = version.id
  try {
    await setCurrentVersion(props.programId, version.id)
    ElMessage.success('已设为当前版本')
    await load()
    emit('changed')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    switchingId.value = null
  }
}

/* ------------------------------------------------------------ 上传新版本 */

const fileInputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

/** 最近一次上传的返回体（版本信息 + 正文识别结果），就地展示在下面 */
const uploadResult = ref<UploadVersionResult | null>(null)

function pickFile(): void {
  fileInputRef.value?.click()
}

/** 弹窗每次打开都清掉上一次的识别结果，避免和这次看到的版本对不上号 */
async function onOpen(): Promise<void> {
  uploadResult.value = null
  await load()
}

async function onFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 立刻清空，否则连续选同一个文件不会再触发 change
  input.value = ''
  if (!file) return

  uploading.value = true
  try {
    // 变更说明弹窗与上传统一在 uploadNcVersion 里处理
    const result = await uploadNcVersion(props.programId, file, props.programNo)
    // null = 用户在上传前取消了
    if (!result) return
    uploadResult.value = result
    await load()
    emit('changed')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    uploading.value = false
  }
}

/**
 * 识别结果里的刀具已确认加入，让父组件刷新详情。
 *
 * 提示由识别面板自己弹：只有它知道加进去几把、跳过几把，
 * 这边再弹一条就成两条了。
 */
function onToolsAdded(): void {
  emit('changed')
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="`版本历史 · ${programNo}`"
    width="920px"
    top="6vh"
    @open="onOpen"
  >
    <el-alert
      v-if="loadError"
      type="error"
      :closable="false"
      show-icon
      :title="loadError"
      class="dialog-error"
    >
      <el-button link type="primary" @click="load">重试</el-button>
    </el-alert>

    <!-- 上传区：上传成功后识别结果就贴在它下面 -->
    <div class="upload-bar">
      <el-button size="small" type="primary" plain :loading="uploading" @click="pickFile">
        上传新版本
      </el-button>
      <span class="upload-hint">上传后自动识别数控系统与刀具，识别结果可以先改再加入刀具补偿表</span>
      <input ref="fileInputRef" type="file" class="hidden-file" @change="onFileChange" />
    </div>

    <NcParsePanel
      v-if="uploadResult"
      :parse="uploadResult.parse ?? null"
      :program-id="programId"
      :title="`第 ${uploadResult.versionNo} 版上传成功`"
      closable
      @tools-added="onToolsAdded"
      @close="uploadResult = null"
    />

    <div v-loading="loading">
      <p v-if="!loading && versions.length === 0" class="empty">该程序还没有上传过 NC 文件。</p>

      <table v-else class="ver-table">
        <thead>
          <tr>
            <th class="col-ver">版本</th>
            <th>文件名</th>
            <th class="col-size">大小</th>
            <th>变更说明</th>
            <th class="col-time">时间</th>
            <th class="col-act">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in versions" :key="item.id">
            <td class="col-ver">
              第 {{ item.versionNo }} 版
              <span v-if="item.isCurrent" class="current-tag">当前</span>
            </td>
            <td class="cell-file">{{ item.fileName }}</td>
            <td class="col-size">{{ formatBytes(item.fileSize) }}</td>
            <td class="cell-note">{{ item.changeNote || '—' }}</td>
            <td class="col-time">{{ formatDateTime(item.createdAt) }}</td>
            <td class="col-act">
              <el-link type="primary" :href="versionDownloadUrl(item.id)" target="_blank">
                下载
              </el-link>
              <el-button
                v-if="!item.isCurrent"
                link
                type="primary"
                :loading="switchingId === item.id"
                @click="makeCurrent(item)"
              >
                设为当前
              </el-button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="versions.length > 1" class="compare">
      <div class="compare-head">
        <span class="compare-title">版本对比</span>
        <el-select v-model="oldId" size="small" class="ver-select" placeholder="旧版">
          <el-option
            v-for="item in versions"
            :key="item.id"
            :label="`第 ${item.versionNo} 版 · ${item.fileName}`"
            :value="item.id"
          />
        </el-select>
        <span class="arrow">→</span>
        <el-select v-model="newId" size="small" class="ver-select" placeholder="新版">
          <el-option
            v-for="item in versions"
            :key="item.id"
            :label="`第 ${item.versionNo} 版 · ${item.fileName}`"
            :value="item.id"
          />
        </el-select>
        <el-button size="small" type="primary" :disabled="!canCompare" :loading="diffLoading" @click="compare">
          对比
        </el-button>
      </div>

      <p v-if="diff" class="diff-summary">{{ diffSummary }}</p>

      <p v-if="diff && diff.identical" class="empty">两个版本内容完全相同。</p>

      <div v-else-if="diff" class="diff">
        <div class="diff-head">
          <span>第 {{ diff.leftVersionNo }} 版</span>
          <span>第 {{ diff.rightVersionNo }} 版</span>
        </div>
        <div
          v-for="(line, index) in diff.lines"
          :key="index"
          class="diff-row"
          :class="`diff-${line.type}`"
        >
          <div class="diff-cell">
            <span class="line-no">{{ line.leftNo ?? '' }}</span>
            <span class="line-text">{{ line.leftText || ' ' }}</span>
          </div>
          <div class="diff-cell">
            <span class="line-no">{{ line.rightNo ?? '' }}</span>
            <span class="line-text">{{ line.rightText || ' ' }}</span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.dialog-error {
  margin-bottom: 12px;
}

.upload-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.upload-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.hidden-file {
  display: none;
}

.empty {
  margin: 12px 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.ver-table {
  width: 100%;
  border-collapse: collapse;
}

.ver-table th,
.ver-table td {
  padding: 7px 8px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  font-size: 13px;
  text-align: left;
  vertical-align: middle;
}

.ver-table th {
  color: var(--el-text-color-regular);
  font-size: 12px;
  font-weight: 500;
}

.col-ver {
  width: 116px;
  white-space: nowrap;
}

.col-size {
  width: 84px;
  white-space: nowrap;
}

.col-time {
  width: 132px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.col-act {
  width: 150px;
  white-space: nowrap;
}

.col-act :deep(.el-button + .el-button) {
  margin-left: 10px;
}

.current-tag {
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 2px;
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
  font-size: 11px;
}

.cell-file {
  word-break: break-all;
}

.cell-note {
  color: var(--el-text-color-regular);
}

.compare {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.compare-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.compare-title {
  margin-right: 4px;
  font-size: 13px;
  font-weight: 600;
}

.ver-select {
  width: 250px;
}

.arrow {
  color: var(--el-text-color-secondary);
}

.diff-summary {
  margin: 12px 0 8px;
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.diff {
  max-height: 320px;
  overflow: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
}

.diff-head {
  display: flex;
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-regular);
  font-weight: 600;
}

.diff-head span {
  flex: 1;
  padding: 5px 8px;
}

.diff-head span + span {
  border-left: 1px solid var(--el-border-color-lighter);
}

.diff-row {
  display: flex;
}

.diff-cell {
  display: flex;
  flex: 1;
  min-width: 0;
  padding: 1px 0;
}

.diff-cell + .diff-cell {
  border-left: 1px solid var(--el-border-color-lighter);
}

.line-no {
  flex: 0 0 42px;
  padding: 0 6px;
  color: var(--el-text-color-placeholder);
  text-align: right;
  user-select: none;
}

.line-text {
  flex: 1;
  padding: 0 8px 0 6px;
  white-space: pre-wrap;
  word-break: break-all;
}

/* 三色：新增绿、删除红、修改左红右绿 */
.diff-add .diff-cell:last-child {
  background: var(--el-color-success-light-9);
}

.diff-del .diff-cell:first-child {
  background: var(--el-color-danger-light-9);
}

.diff-change .diff-cell:first-child {
  background: var(--el-color-danger-light-9);
}

.diff-change .diff-cell:last-child {
  background: var(--el-color-success-light-9);
}
</style>
