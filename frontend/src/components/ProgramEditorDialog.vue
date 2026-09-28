<script setup lang="ts">
/**
 * 程序查看 / 编辑弹窗。
 *
 * 用「查看程序」按钮打开，默认显示**当前版本**，也可以在顶部下拉里切换任意版本。
 *
 * 两种模式：
 *   - 只读（默认）：等宽字体 + 行号，保留换行与缩进，右侧可横向滚动，行号列不跟着滚。
 *   - 编辑：点「编辑」切成 textarea；「取消编辑」丢弃改动（会先确认一次）。
 *
 * 两种保存方式（内容与原文相同时按钮禁用）：
 *   - 另存为新版本：POST /api/programs/{id}/versions/content，原版本保留，可回溯。
 *   - 覆盖当前版本：PUT /api/versions/{id}/content，版本号不变，原内容不再保留（二次确认）。
 *
 * 关键约束：encoding 必须把读到的 VersionContent.encoding **原样带回**，
 * 写死 'utf-8' 会把现场的 GBK 程序毁掉，机床可能直接不认。
 */
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { ParseResult, Version, VersionContent } from '../api/types'
import {
  getVersionContent,
  listVersions,
  overwriteVersionContent,
  parseNcText,
  saveVersionContentAsNew
} from '../api'
import { errorMessage } from '../api/client'
import { formatBytes } from '../utils/format'
import NcParsePanel from './NcParsePanel.vue'

const props = defineProps<{
  modelValue: boolean
  programId: number
  programNo: string
  /** 父组件加载到的当前版本 id；作为默认打开的那一版 */
  currentVersionId: number | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 保存成功（版本数 / 当前版本号可能变了），父组件需要重新拉详情 */
  saved: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value)
})

/* ------------------------------------------------------------ 版本列表 */

const versions = ref<Version[]>([])
const selectedVersionId = ref<number | null>(null)
const listLoading = ref(false)

const selectedVersion = computed(
  () => versions.value.find((item) => item.id === selectedVersionId.value) ?? null
)

/* ------------------------------------------------------------ 程序文本 */

const current = ref<VersionContent | null>(null)
const contentLoading = ref(false)
const loadError = ref('')

const editing = ref(false)
/** 编辑中的草稿，只在编辑模式下有意义 */
const draft = ref('')
const changeNote = ref('')

const savingNew = ref(false)
const savingOverwrite = ref(false)

const busy = computed(() => savingNew.value || savingOverwrite.value)

/** 内容与加载到的原文是否有差异；两个保存按钮都以此为准禁用 */
const isDirty = computed(() => current.value !== null && draft.value !== current.value.content)

/**
 * 行号列的文本。
 *
 * 与 <pre> 的换行规则保持一致：末尾的换行不额外算一行，
 * 所以这里的行数和后端的 lineCount 是对得上的，也与右侧代码逐行对齐。
 */
const lineNoText = computed(() => {
  const text = current.value?.content ?? ''
  if (!text) return ''
  const lines = text.split('\n')
  if (lines.length > 1 && lines[lines.length - 1] === '') lines.pop()
  return lines.map((_, index) => index + 1).join('\n')
})

/* ------------------------------------------------------------ 识别程序 */

/**
 * 只读模式下的识别结果（**只有刀具识别**：数控系统 / 行数 / 刀具 / 警告）。
 * 程序号由人工输入记录，这里不显示也不对比。
 * 编辑模式下按钮与面板都不出现——内容正在改，识别出来的东西没意义。
 */
const parseVisible = ref(false)
const parsed = ref<ParseResult | null>(null)
const parsing = ref(false)

function clearParse(): void {
  parseVisible.value = false
  parsed.value = null
}

/** 「识别程序」/「收起」：拿当前已加载的正文去 POST /api/nc/parse */
async function toggleRecognize(): Promise<void> {
  if (parseVisible.value) {
    parseVisible.value = false
    return
  }
  const snapshot = current.value
  if (!snapshot) return

  parsing.value = true
  try {
    parsed.value = await parseNcText(snapshot.content)
    parseVisible.value = true
  } catch (error) {
    parsed.value = null
    parseVisible.value = false
    ElMessage.error(errorMessage(error))
  } finally {
    parsing.value = false
  }
}

/** 拉取程序的版本列表 */
async function loadVersions(): Promise<void> {
  listLoading.value = true
  try {
    const items = await listVersions(props.programId)
    // 后端按版本号倒序返回，这里再排一次，保证「最新」一定是第一条
    versions.value = [...items].sort((a, b) => b.versionNo - a.versionNo)
  } catch (error) {
    versions.value = []
    loadError.value = errorMessage(error)
  } finally {
    listLoading.value = false
  }
}

/** 拉取当前选中版本的文本 */
async function loadContent(): Promise<void> {
  const id = selectedVersionId.value
  if (id === null) {
    current.value = null
    return
  }
  contentLoading.value = true
  loadError.value = ''
  // 换了版本，上一次的识别结果就作废了
  clearParse()
  try {
    const data = await getVersionContent(id)
    current.value = data
    draft.value = data.content
    changeNote.value = ''
    editing.value = false
  } catch (error) {
    current.value = null
    loadError.value = errorMessage(error)
  } finally {
    contentLoading.value = false
  }
}

/**
 * 弹窗打开时加载数据。
 *
 * 这里必须用 watch + immediate，**不能**用 el-dialog 的 @open 事件。
 * 原因：本组件由父组件的 v-if 控制挂载，父组件在点击「查看程序」时
 * 先设置 program 再设置 visible=true，两者在同一轮同步更新里完成，
 * 于是组件挂载时 modelValue **已经是 true**。el-dialog 只在
 * visible 由 false 变 true 时才 emit('open')，首帧就已经是 true 的情况
 * 它不会补发——结果就是弹窗打开了但数据从没加载过，
 * 界面显示"该程序还没有上传过 NC 文件"（实测踩过这个坑）。
 *
 * immediate: true 保证挂载即执行一次；组件被 v-if 反复创建，
 * 所以每次打开都会重新加载。
 */
watch(visible, (value) => {
  if (value) void open()
}, { immediate: true })

async function open(): Promise<void> {
  loadError.value = ''
  editing.value = false
  draft.value = ''
  changeNote.value = ''
  current.value = null
  selectedVersionId.value = null
  versions.value = []
  clearParse()

  await loadVersions()

  // 默认打开当前生效版本；没有当前版本就退回最新的一版
  const target =
    versions.value.find((item) => item.id === props.currentVersionId) ??
    versions.value.find((item) => item.isCurrent) ??
    versions.value[0]
  selectedVersionId.value = target?.id ?? null

  if (selectedVersionId.value !== null) await loadContent()
}

/** 切换版本：正在编辑且有改动时先确认一次，取消则把下拉框选回原来那一版 */
async function onVersionChange(): Promise<void> {
  if (editing.value && isDirty.value) {
    try {
      await ElMessageBox.confirm('切换版本会丢弃当前未保存的编辑内容，确定继续？', '切换版本', {
        confirmButtonText: '丢弃并切换',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      selectedVersionId.value = current.value?.versionId ?? null
      return
    }
  }
  await loadContent()
}

/* ------------------------------------------------------------ 编辑模式 */

function startEdit(): void {
  if (!current.value || busy.value) return
  draft.value = current.value.content
  changeNote.value = ''
  editing.value = true
  // 编辑模式下不展示识别结果，收起面板免得和正在改的内容混淆
  clearParse()
}

async function cancelEdit(): Promise<void> {
  if (isDirty.value) {
    try {
      await ElMessageBox.confirm('放弃本次编辑的改动？改动不会被保存。', '取消编辑', {
        confirmButtonText: '放弃改动',
        cancelButtonText: '继续编辑',
        type: 'warning'
      })
    } catch {
      return
    }
  }
  draft.value = current.value?.content ?? ''
  changeNote.value = ''
  editing.value = false
}

/* ------------------------------------------------------------ 保存 */

/** 另存为新版本：原版本原封不动保留，可以回溯、对比、回滚 */
async function saveAsNew(): Promise<void> {
  const snapshot = current.value
  if (!snapshot || !isDirty.value || busy.value) return

  savingNew.value = true
  try {
    const created = await saveVersionContentAsNew(props.programId, {
      content: draft.value,
      // 编码原样带回，绝不写死 'utf-8'
      encoding: snapshot.encoding,
      changeNote: changeNote.value.trim()
    })
    await loadVersions()
    // 就地切到新版本并重新拉一次内容
    selectedVersionId.value = created.id
    await loadContent()
    ElMessage.success(`已另存为第 ${created.versionNo} 版`)
    emit('saved')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingNew.value = false
  }
}

/** 覆盖当前版本：版本号不变，原内容不再保留 */
async function overwriteCurrent(): Promise<void> {
  const snapshot = current.value
  if (!snapshot || !isDirty.value || busy.value) return

  try {
    await ElMessageBox.confirm(
      `覆盖后会直接替换第 ${snapshot.versionNo} 版的文件内容，版本号不变，原内容不再保留。` +
        '确定继续？',
      '覆盖当前版本',
      { confirmButtonText: '覆盖', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  savingOverwrite.value = true
  try {
    await overwriteVersionContent(snapshot.versionId, {
      content: draft.value,
      // 编码原样带回，绝不写死 'utf-8'
      encoding: snapshot.encoding,
      // 用户没填变更说明就沿用这一版原有的，避免把原来的说明清空
      changeNote: changeNote.value.trim() || selectedVersion.value?.changeNote || ''
    })
    await loadVersions()
    await loadContent()
    ElMessage.success(`第 ${snapshot.versionNo} 版已覆盖保存`)
    emit('saved')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingOverwrite.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="`查看程序 · ${programNo}`"
    width="70%"
    top="6vh"
  >
    <el-alert
      v-if="loadError"
      type="error"
      :closable="false"
      show-icon
      :title="loadError"
      class="dialog-error"
    >
      <el-button link type="primary" @click="loadContent">重试</el-button>
    </el-alert>

    <!-- 顶部信息条 -->
    <div class="info-bar">
      <el-select
        v-model="selectedVersionId"
        class="ver-select"
        size="small"
        placeholder="选择版本"
        :loading="listLoading"
        :disabled="busy"
        @change="onVersionChange"
      >
        <el-option
          v-for="item in versions"
          :key="item.id"
          :label="`第 ${item.versionNo} 版 · ${item.fileName}`"
          :value="item.id"
        />
      </el-select>

      <template v-if="current">
        <span class="info-text">{{ current.fileName }}</span>
        <span class="info-sep">·</span>
        <span class="info-text">第 {{ current.versionNo }} 版</span>
        <span class="info-sep">·</span>
        <span class="info-text">{{ current.lineCount }} 行</span>
        <span class="info-sep">·</span>
        <span class="info-text">{{ formatBytes(current.sizeBytes) }}</span>
        <el-tag v-if="current.encoding.toLowerCase() === 'gbk'" type="warning" size="small">
          GBK 编码
        </el-tag>
        <el-tag v-else type="info" size="small">UTF-8</el-tag>
      </template>

      <div class="info-actions">
        <el-button
          v-if="!editing"
          size="small"
          :disabled="!current"
          :loading="parsing"
          @click="toggleRecognize"
        >
          {{ parseVisible ? '收起' : '识别程序' }}
        </el-button>
        <el-button
          v-if="!editing"
          size="small"
          type="primary"
          plain
          :disabled="!current"
          @click="startEdit"
        >
          编辑
        </el-button>
        <el-button v-else size="small" :disabled="busy" @click="cancelEdit">取消编辑</el-button>
      </div>
    </div>

    <p class="info-hint">保存时会按原编码写回，不会改变文件编码</p>

    <!-- 识别结果：只在只读模式下出现 -->
    <NcParsePanel v-if="parseVisible && !editing && current" :parse="parsed" :program-id="programId" />

    <!-- 程序内容区 -->
    <div v-loading="contentLoading" class="viewer-wrap">
      <p v-if="!current && !contentLoading" class="empty">该程序还没有上传过 NC 文件。</p>

      <template v-else-if="current">
        <!-- 只读：行号与正文同一个滚动容器，字号行高一致，逐行对齐 -->
        <div v-if="!editing" class="viewer">
          <div class="viewer-inner">
            <pre class="gutter">{{ lineNoText }}</pre>
            <pre class="code">{{ current.content }}</pre>
          </div>
        </div>

        <!-- 编辑 -->
        <el-input
          v-else
          v-model="draft"
          class="editor"
          type="textarea"
          :rows="24"
          resize="vertical"
          spellcheck="false"
        />
      </template>
    </div>

    <template #footer>
      <div class="footer">
        <div v-if="editing" class="footer-left">
          <el-input
            v-model="changeNote"
            class="note-input"
            size="small"
            placeholder="本次改了什么？例如：修改 F 值"
            :disabled="busy"
            clearable
          />
        </div>
        <div class="footer-right">
          <el-button @click="visible = false">关闭</el-button>
          <template v-if="editing">
            <el-button
              :disabled="!isDirty || busy"
              :loading="savingNew"
              @click="saveAsNew"
            >
              另存为新版本
            </el-button>
            <el-button
              type="primary"
              :disabled="!isDirty || busy"
              :loading="savingOverwrite"
              @click="overwriteCurrent"
            >
              覆盖当前版本
            </el-button>
          </template>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.dialog-error {
  margin-bottom: 12px;
}

.info-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.ver-select {
  width: 260px;
}

.info-text {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.info-sep {
  color: var(--el-border-color);
}

.info-actions {
  margin-left: auto;
}

.info-hint {
  margin: 8px 0 10px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.empty {
  margin: 12px 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.viewer-wrap {
  min-height: 120px;
}

.viewer {
  max-height: 60vh;
  overflow: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  background: var(--el-fill-color-blank);
}

.viewer-inner {
  display: flex;
  align-items: flex-start;
  min-width: max-content;
}

/* 行号列与正文用同一套字体与行高，保证逐行对齐 */
.gutter,
.code {
  margin: 0;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 20px;
}

/* sticky 让行号列在横向滚动时留在左边，纵向仍跟着内容走 */
.gutter {
  position: sticky;
  left: 0;
  z-index: 1;
  flex: 0 0 auto;
  padding: 8px 8px 8px 10px;
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-light);
  color: var(--el-text-color-placeholder);
  text-align: right;
  user-select: none;
}

.code {
  flex: 0 0 auto;
  padding: 8px 12px;
  color: var(--el-text-color-primary);
  white-space: pre;
}

.editor :deep(.el-textarea__inner) {
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 20px;
  white-space: pre;
  resize: vertical;
}

.footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.footer-left {
  flex: 1 1 320px;
  min-width: 220px;
}

.footer-right {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}
</style>
