<script setup lang="ts">
/**
 * 工序卡片：一道工序一张卡，卡里再纵向排它的多个程序，每个程序下面跟自己的刀具补偿表。
 *
 * 保存策略（「失焦即保存」+ 明确的保存状态提示）：
 *   - 工序的 4 个字段改动后失焦即 PUT /api/operations/{id}
 *   - 程序名改动后失焦即 PUT /api/programs/{id}
 *   - 卡头右上角的状态文字始终显示「已保存 / 未保存 / 保存中… / 保存失败」，
 *     不会出现「改了但不知道存没存上」的情况
 *   - 新增 / 删除工序与程序、上传 NC、切换当前版本这些会改变结构的操作，
 *     统一通过 reload 事件让 MainView 重新拉一次 /detail
 *
 * 重要：后端 PUT 是**整体替换**语义。界面上不显示的 opName / machineId /
 * programName / controller / remark 必须用加载到的原值原样提交，不能填死默认值。
 */
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import ToolTable from './ToolTable.vue'
import VersionDialog from './VersionDialog.vue'
import ProgramEditorDialog from './ProgramEditorDialog.vue'
import NcParsePanel from './NcParsePanel.vue'
import {
  createProgram,
  deleteOperation,
  deleteProgram,
  saveProgramTools,
  updateOperation,
  updateProgram
} from '../api'
import { errorMessage } from '../api/client'
import type {
  DetailOperation,
  DetailProgram,
  OperationInput,
  ProgramToolInput,
  UploadVersionResult
} from '../api/types'
import { formatOpNo, parseOpNo, toNumber } from '../utils/format'
import { applyRecognizedProgramNo, uploadNcVersion } from '../utils/ncUpload'

const props = defineProps<{
  operation: DetailOperation
}>()

const emit = defineEmits<{
  /** 结构发生变化（增删工序/程序、上传 NC、切换当前版本、工序号变动），需要重新拉详情 */
  reload: []
}>()

/* ------------------------------------------------------------ 工序字段 */

const form = reactive({
  opNoText: '',
  fixture: '',
  zHeight: 0,
  remark: ''
})

const savedSnapshot = ref('')
const saving = ref(false)
const failed = ref(false)
let savePending = false

/** 提交体：可编辑字段用表单值，隐藏字段用加载到的原值 */
function buildPayload(): OperationInput {
  return {
    opNo: parseOpNo(form.opNoText) ?? props.operation.opNo,
    opName: props.operation.opName,
    machineId: props.operation.machineId,
    fixture: form.fixture,
    zHeight: toNumber(form.zHeight),
    remark: form.remark
  }
}

function resetOperation(): void {
  form.opNoText = formatOpNo(props.operation.opNo)
  form.fixture = props.operation.fixture
  form.zHeight = toNumber(props.operation.zHeight)
  form.remark = props.operation.remark
  savedSnapshot.value = JSON.stringify(buildPayload())
  failed.value = false
}

watch(() => props.operation, resetOperation, { immediate: true })

const dirty = computed(() => JSON.stringify(buildPayload()) !== savedSnapshot.value)
const displayOpNo = computed(() => formatOpNo(parseOpNo(form.opNoText) ?? props.operation.opNo))

type SaveState = 'saved' | 'dirty' | 'saving' | 'error'

const state = computed<SaveState>(() => {
  if (saving.value) return 'saving'
  if (failed.value) return 'error'
  if (dirty.value) return 'dirty'
  return 'saved'
})

const stateText = computed(
  () =>
    ({ saved: '已保存', dirty: '未保存', saving: '保存中…', error: '保存失败' } as const)[state.value]
)

/** 失焦即保存；同一时刻只允许一个请求在飞，期间的新改动排队补发 */
async function save(): Promise<void> {
  if (saving.value) {
    savePending = true
    return
  }
  const payload = buildPayload()
  if (JSON.stringify(payload) === savedSnapshot.value) return

  saving.value = true
  failed.value = false
  try {
    await updateOperation(props.operation.id, payload)
    savedSnapshot.value = JSON.stringify(payload)
  } catch (error) {
    failed.value = true
    ElMessage.error(errorMessage(error))
  } finally {
    saving.value = false
    if (savePending) {
      savePending = false
      void save()
    }
  }
}

/** 工序号允许填 10# 或 10；解析失败就回滚，解析成功统一显示成 10# */
async function onOpNoChange(): Promise<void> {
  const parsed = parseOpNo(form.opNoText)
  if (parsed === null) {
    ElMessage.error('工序号必须是 1~9999 之间的整数，例如 10# 或 10')
    form.opNoText = formatOpNo(props.operation.opNo)
    return
  }
  form.opNoText = formatOpNo(parsed)
  const orderChanged = parsed !== props.operation.opNo
  await save()
  // 工序号变了会影响排序，重新拉一次让卡片挪到正确位置
  if (orderChanged) emit('reload')
}

async function confirmDeleteOperation(): Promise<void> {
  try {
    await ElMessageBox.confirm(
      `确定删除工序 ${displayOpNo.value}？该工序下的所有程序和 NC 版本记录会一并级联删除，且不可恢复。`,
      '删除工序',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteOperation(props.operation.id)
    ElMessage.success('工序已删除')
    emit('reload')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

/* -------------------------------------------------------------- 程序行 */

interface ProgramForm {
  /** 后端加载到的原始对象（含 tools 与隐藏字段），只读使用 */
  source: DetailProgram
  programNo: string
  /** 已落库的程序名，用于判断是否有未保存改动 */
  baselineNo: string
  saving: boolean
}

const programForms = ref<ProgramForm[]>([])

function resetPrograms(): void {
  programForms.value = props.operation.programs.map((program) => ({
    source: program,
    programNo: program.programNo,
    baselineNo: program.programNo,
    saving: false
  }))
}

watch(() => props.operation.programs, resetPrograms, { immediate: true })

function isProgramDirty(item: ProgramForm): boolean {
  return item.programNo.trim() !== item.baselineNo
}

async function saveProgram(item: ProgramForm): Promise<void> {
  const programNo = item.programNo.trim()
  if (!programNo) {
    ElMessage.error('程序名不能为空，例如 O1234')
    item.programNo = item.baselineNo
    return
  }
  if (programNo === item.baselineNo || item.saving) return

  item.saving = true
  try {
    await updateProgram(item.source.id, {
      programNo,
      // 隐藏字段按原值带回，避免整体替换把后端数据清空
      programName: item.source.programName,
      controller: item.source.controller,
      remark: item.source.remark
    })
    item.programNo = programNo
    item.baselineNo = programNo
  } catch (error) {
    ElMessage.error(errorMessage(error))
    item.programNo = item.baselineNo
  } finally {
    item.saving = false
  }
}

async function confirmDeleteProgram(item: ProgramForm): Promise<void> {
  try {
    await ElMessageBox.confirm(
      `确定删除程序 ${item.baselineNo}？该程序下的所有 NC 版本记录会一并级联删除，且不可恢复。`,
      '删除程序',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteProgram(item.source.id)
    ElMessage.success('程序已删除')
    emit('reload')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

/* ------------------------------------------------------ 新增程序对话框 */

const createVisible = ref(false)
const newProgramNo = ref('')
const creating = ref(false)

function openCreateProgram(): void {
  newProgramNo.value = ''
  createVisible.value = true
}

async function confirmCreateProgram(): Promise<void> {
  const programNo = newProgramNo.value.trim()
  if (!programNo) {
    ElMessage.error('程序名不能为空，例如 O1234')
    return
  }
  creating.value = true
  try {
    // 新建程序的隐藏字段没有原值可取，按契约给空值
    await createProgram(props.operation.id, {
      programNo,
      programName: '',
      controller: '',
      remark: ''
    })
    createVisible.value = false
    ElMessage.success('程序已添加')
    emit('reload')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    creating.value = false
  }
}

/* ---------------------------------------------------------- 刀具补偿表 */

const savingToolsId = ref<number | null>(null)

async function onSaveTools(item: ProgramForm, items: ProgramToolInput[]): Promise<void> {
  savingToolsId.value = item.source.id
  try {
    await saveProgramTools(item.source.id, items)
    ElMessage.success('刀具补偿已保存')
    emit('reload')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingToolsId.value = null
  }
}

/* ------------------------------------------------------------ 版本相关 */

const versionVisible = ref(false)
const versionProgram = ref<DetailProgram | null>(null)

function openVersions(item: ProgramForm): void {
  versionProgram.value = item.source
  versionVisible.value = true
}

/* ------------------------------------------------------------ 查看程序 */

const editorVisible = ref(false)
const editorProgram = ref<DetailProgram | null>(null)

/** 没有版本就没得看：版本数为 0 或没有当前版本时「查看程序」按钮禁用 */
function canViewProgram(program: DetailProgram): boolean {
  return program.versionCount > 0 && program.currentVersionId !== null
}

function openProgramEditor(item: ProgramForm): void {
  if (!canViewProgram(item.source)) return
  editorProgram.value = item.source
  editorVisible.value = true
}

/**
 * 详情重新拉取后，弹窗里持有的对象引用会变成旧的（程序号可能刚被改正过），
 * 这里换成同 id 的新对象，避免弹窗标题与识别结果面板一直显示旧值。
 */
watch(
  () => props.operation.programs,
  (programs) => {
    const versionTarget = versionProgram.value
    if (versionTarget) {
      versionProgram.value = programs.find((item) => item.id === versionTarget.id) ?? null
    }
    const editorTarget = editorProgram.value
    if (editorTarget) {
      editorProgram.value = programs.find((item) => item.id === editorTarget.id) ?? null
    }
  }
)

/* -------------------------------------------------------------- 上传 NC */

const fileInputRef = ref<HTMLInputElement | null>(null)
const uploadTarget = ref<ProgramForm | null>(null)
const uploadingId = ref<number | null>(null)

/** 每个程序最近一次上传的识别结果，就地显示在上传区下面，按程序 id 存 */
const uploadResults = ref<Record<number, UploadVersionResult>>({})

function uploadResultOf(programId: number): UploadVersionResult | null {
  return uploadResults.value[programId] ?? null
}

function dismissUploadResult(programId: number): void {
  const next = { ...uploadResults.value }
  delete next[programId]
  uploadResults.value = next
}

function pickFile(item: ProgramForm): void {
  uploadTarget.value = item
  fileInputRef.value?.click()
}

async function onFileChange(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 立刻清空，否则连续选同一个文件不会再触发 change
  input.value = ''

  const target = uploadTarget.value
  uploadTarget.value = null
  if (!file || !target) return

  uploadingId.value = target.source.id
  try {
    // 变更说明输入、上传、以及「程序号对不上」的叫停弹窗都在这里统一处理
    const result = await uploadNcVersion(target.source.id, file, target.baselineNo)
    // null = 用户在上传前取消了
    if (!result) return
    uploadResults.value = { ...uploadResults.value, [target.source.id]: result }
    emit('reload')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    uploadingId.value = null
  }
}

/** 识别结果里的刀具已确认加入，提示条数并刷新详情 */
function onToolsAdded(count: number): void {
  ElMessage.success(`已加入 ${count} 把刀`)
  emit('reload')
}

/** 把记录里的程序号改成识别到的那个（其余字段按加载到的原值带回） */
async function fixProgramNo(item: ProgramForm, recognizedNo: string): Promise<void> {
  const done = await applyRecognizedProgramNo(item.source.id, recognizedNo, {
    programNo: item.source.programNo,
    programName: item.source.programName,
    controller: item.source.controller,
    remark: item.source.remark
  })
  if (!done) return
  // 本地先同步，免得等详情刷新的这段间隙输入框里还是旧号
  item.programNo = recognizedNo
  item.baselineNo = recognizedNo
  emit('reload')
}
</script>

<template>
  <section class="card">
    <div class="card-head">
      <div class="head-left">
        <span class="card-title">工序 {{ displayOpNo }}</span>
        <span class="state" :class="`state-${state}`">{{ stateText }}</span>
      </div>
      <el-button link type="danger" @click="confirmDeleteOperation">删除</el-button>
    </div>

    <div class="fields">
      <label class="field">
        <span class="field-label">工序号</span>
        <el-input v-model="form.opNoText" placeholder="如 10#" @change="onOpNoChange" />
      </label>
      <label class="field">
        <span class="field-label">装夹方式</span>
        <el-input v-model="form.fixture" placeholder="如 三爪卡盘" @change="save" />
      </label>
      <label class="field">
        <span class="field-label">Z轴垫高</span>
        <el-input-number
          v-model="form.zHeight"
          :min="0"
          :precision="3"
          :controls="false"
          :value-on-clear="0"
          @change="save"
        />
        <span class="unit">mm</span>
      </label>
    </div>

    <label class="field field-full">
      <span class="field-label">备注</span>
      <el-input v-model="form.remark" placeholder="选填" @change="save" />
    </label>

    <div class="programs">
      <div v-for="item in programForms" :key="item.source.id" class="program">
        <div class="program-head">
          <span class="field-label">程序名</span>
          <el-input
            v-model="item.programNo"
            class="program-input"
            placeholder="如 O1234"
            @change="saveProgram(item)"
          />
          <span v-if="item.saving" class="state state-saving">保存中…</span>
          <span v-else-if="isProgramDirty(item)" class="state state-dirty">未保存</span>
          <el-button link type="danger" @click="confirmDeleteProgram(item)">删除</el-button>
        </div>

        <ToolTable
          :tools="item.source.tools"
          :saving="savingToolsId === item.source.id"
          @save="onSaveTools(item, $event)"
        />

        <div class="program-bar">
          <el-button
            link
            type="primary"
            :loading="uploadingId === item.source.id"
            @click="pickFile(item)"
          >
            上传 NC
          </el-button>
          <span class="bar-sep">|</span>
          <span class="bar-text">
            当前第 {{ item.source.currentVersionNo ?? '—' }} 版
          </span>
          <span class="bar-sep">|</span>
          <el-button
            link
            type="primary"
            :disabled="!canViewProgram(item.source)"
            :title="canViewProgram(item.source) ? '查看并编辑程序内容' : '还没有上传程序'"
            @click="openProgramEditor(item)"
          >
            查看程序
          </el-button>
          <span class="bar-sep">|</span>
          <el-button link type="primary" @click="openVersions(item)">版本历史</el-button>
        </div>

        <!-- 上传成功后就在上传区下面给出识别结果，不藏在别的弹窗里 -->
        <NcParsePanel
          v-if="uploadResultOf(item.source.id)"
          :parse="uploadResultOf(item.source.id)?.parse ?? null"
          :program-id="item.source.id"
          :record-program-no="item.source.programNo"
          :title="`第 ${uploadResultOf(item.source.id)?.versionNo ?? ''} 版上传成功`"
          allow-add-tools
          closable
          @fix-program-no="fixProgramNo(item, $event)"
          @tools-added="onToolsAdded"
          @close="dismissUploadResult(item.source.id)"
        />
      </div>

      <p v-if="programForms.length === 0" class="hint">该工序还没有程序，点击下方「+ 添加程序」。</p>

      <el-button class="add-program" @click="openCreateProgram">+ 添加程序</el-button>
    </div>

    <input ref="fileInputRef" type="file" class="hidden-file" @change="onFileChange" />

    <el-dialog v-model="createVisible" title="添加程序" width="420px">
      <label class="field dialog-field">
        <span class="field-label">程序名</span>
        <el-input v-model="newProgramNo" placeholder="如 O1234" @keyup.enter="confirmCreateProgram" />
      </label>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="confirmCreateProgram">确定</el-button>
      </template>
    </el-dialog>

    <VersionDialog
      v-if="versionProgram"
      v-model="versionVisible"
      :program-id="versionProgram.id"
      :program-no="versionProgram.programNo"
      :program-name="versionProgram.programName"
      :controller="versionProgram.controller"
      :remark="versionProgram.remark"
      @changed="emit('reload')"
    />

    <ProgramEditorDialog
      v-if="editorProgram"
      v-model="editorVisible"
      :program-id="editorProgram.id"
      :program-no="editorProgram.programNo"
      :current-version-id="editorProgram.currentVersionId"
      @saved="emit('reload')"
    />
  </section>
</template>

<style scoped>
.card {
  padding: 14px 16px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  background: var(--el-bg-color);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.head-left {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
}

.state {
  font-size: 12px;
}

.state-saved {
  color: var(--el-text-color-placeholder);
}

.state-saving {
  color: var(--el-text-color-secondary);
}

.state-dirty {
  color: var(--el-color-warning);
}

.state-error {
  color: var(--el-color-danger);
}

.fields {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 24px;
}

.field {
  display: flex;
  align-items: center;
  gap: 10px;
}

.field-full {
  margin-top: 12px;
}

.field-label {
  flex: 0 0 56px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  text-align: right;
}

.field :deep(.el-input),
.field :deep(.el-input-number) {
  flex: 1;
}

.unit {
  flex: 0 0 auto;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.programs {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.program {
  padding: 10px 12px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  background: var(--el-fill-color-blank);
}

.program + .program {
  margin-top: 12px;
}

.program-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.program-input {
  width: 200px;
  flex: 0 0 auto;
}

.program-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.bar-sep {
  color: var(--el-border-color);
}

.bar-text {
  color: var(--el-text-color-regular);
}

.hint {
  margin: 4px 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.add-program {
  width: 100%;
}

.dialog-field :deep(.el-input) {
  flex: 1;
}

.hidden-file {
  display: none;
}
</style>
