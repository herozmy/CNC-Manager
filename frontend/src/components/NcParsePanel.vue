<script setup lang="ts">
/**
 * NC 自动识别结果面板（**只展示刀具识别结果**）。
 *
 * 三处共用：上传成功后（OperationCard）、版本弹窗里上传后（VersionDialog）、
 * 以及查看程序时的「识别程序」（ProgramEditorDialog）。
 *
 * 识别结果**可以直接在这里改**再确认加入。原先是"识别到什么就往刀具补偿表里灌什么"，
 * 一键下去没法回头；现场的写法五花八门，识别偶尔看走眼很正常，所以这里给一份
 * 可编辑的清单：改刀具号、改刀补号、删掉多出来的、补上漏掉的，确认没问题再点加入。
 *
 * 程序号由人工输入，这里既不显示也不与记录对比。
 *
 * 判空纪律：parse 可能是 null（后端读文件失败），tools 可能是空数组，
 * offsetNo 可能是空串——都要显示成明确的「未识别到」而不是空白。
 */
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { ParseResult, ProgramToolInput } from '../api/types'
import { getProgramTools, saveProgramTools } from '../api'
import { errorMessage } from '../api/client'
import { normalizeToolNo } from '../utils/nc'
import { toNumber } from '../utils/format'

const props = defineProps<{
  /** 识别结果；后端读取程序正文失败时为 null */
  parse: ParseResult | null
  /** 用于「加入刀具补偿表」 */
  programId: number
  /** 面板标题，例如「第 2 版上传成功」 */
  title?: string
  /** 是否显示「收起」 */
  closable?: boolean
}>()

const emit = defineEmits<{
  /** 刀具已写进刀具补偿表，父组件提示并刷新 */
  toolsAdded: [count: number]
  close: []
}>()

/* ------------------------------------------------------------ 展示数据 */

const controllerText = computed(() => props.parse?.controller ?? '')

/**
 * 既没识别出数控系统、也没识别出刀具时，整块只剩行数可看，
 * 这时候给一句明确的「没有识别到刀具调用」，免得用户对着空面板猜。
 */
const nothingRecognized = computed(() => controllerText.value === '' && rows.value.length === 0)

/** 现场要看的提醒，内联展示，不打扰用户 */
const warnings = computed(() => props.parse?.warnings ?? [])

/* -------------------------------------------------------- 可编辑的刀具行 */

interface ToolRow {
  uid: number
  toolNo: string
  offsetNo: string
  /**
   * 直径与补偿量，mm。
   *
   * 这两个识别不出来——程序正文里没有它们——所以要用户自己填。
   * 放在这里而不是等加入之后再去刀具表补，是因为现场本来就是
   * 「看着程序把刀号、直径、刀补一次填齐」，来回切两处最容易漏。
   */
  toolDia: number
  compAmount: number
}

let uidSeed = 0
const rows = ref<ToolRow[]>([])

function newRow(toolNo = '', offsetNo = ''): ToolRow {
  uidSeed += 1
  return { uid: uidSeed, toolNo, offsetNo, toolDia: 0, compAmount: 0 }
}

/** 后端有识别结果就照抄一份成可编辑的行；没有就从空开始 */
function reset(): void {
  rows.value = (props.parse?.tools ?? []).map((tool) => newRow(tool.toolNo, tool.offsetNo))
}

watch(() => props.parse, reset, { immediate: true })

function addRow(): void {
  // 新增的行给个顺下去的刀号，省得每次都从头敲
  const next = rows.value.length + 1
  rows.value.push(newRow(`T${String(next).padStart(2, '0')}`, `D${String(next).padStart(2, '0')}`))
}

function removeRow(index: number): void {
  rows.value.splice(index, 1)
}

/** 刀具号不能为空——后端整表替换时它是必填的，空着提交必然失败 */
const emptyToolNo = computed(() => rows.value.some((row) => row.toolNo.trim() === ''))

const canAdd = computed(() => rows.value.length > 0 && !emptyToolNo.value)

/* -------------------------------------------------------- 加入刀具补偿表 */

const adding = ref(false)

async function addTools(): Promise<void> {
  if (!canAdd.value || adding.value) return

  adding.value = true
  try {
    const existing = await getProgramTools(props.programId)

    // PUT 是整体替换语义：现有行所有字段按加载到的原值带回，绝不填默认值
    const items: ProgramToolInput[] = existing.map((tool) => ({
      seq: tool.seq,
      toolId: tool.toolId,
      toolNo: tool.toolNo,
      offsetNo: tool.offsetNo,
      toolName: tool.toolName,
      toolDia: tool.toolDia,
      cornerRadius: tool.cornerRadius,
      compAmount: tool.compAmount,
      spindleSpeed: tool.spindleSpeed,
      speedMode: tool.speedMode,
      feed: tool.feed,
      feedMode: tool.feedMode,
      cutDepth: tool.cutDepth,
      coolant: tool.coolant,
      machiningContent: tool.machiningContent,
      remark: tool.remark
    }))

    // 已存在的刀具号不再重复添加（T1 与 T01 视为同一把）
    const known = new Set(existing.map((tool) => normalizeToolNo(tool.toolNo)))
    let maxSeq = existing.reduce((max, tool) => Math.max(max, tool.seq), 0)
    const added: ProgramToolInput[] = []
    let skipped = 0

    for (const row of rows.value) {
      const toolNo = row.toolNo.trim()
      const key = normalizeToolNo(toolNo)
      if (!key || known.has(key)) {
        skipped += 1
        continue
      }
      known.add(key)
      maxSeq += 1
      added.push({
        seq: maxSeq,
        toolId: null,
        toolNo,
        offsetNo: row.offsetNo.trim(),
        toolName: '',
        toolDia: toNumber(row.toolDia),
        cornerRadius: 0,
        compAmount: toNumber(row.compAmount),
        spindleSpeed: 0,
        speedMode: 0,
        feed: 0,
        feedMode: 0,
        cutDepth: 0,
        coolant: 0,
        machiningContent: '',
        remark: ''
      })
    }

    if (added.length === 0) {
      ElMessage.info(skipped > 0 ? '这些刀具在补偿表里已经有了' : '没有要加入的刀具')
      return
    }

    await saveProgramTools(props.programId, [...items, ...added])
    if (skipped > 0) {
      ElMessage.success(`已加入 ${added.length} 把刀，另有 ${skipped} 把表里已有，已跳过`)
    } else {
      ElMessage.success(`已加入 ${added.length} 把刀`)
    }
    emit('toolsAdded', added.length)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    adding.value = false
  }
}
</script>

<template>
  <div class="parse-panel">
    <div v-if="title || closable" class="panel-head">
      <span v-if="title" class="panel-title">✓ {{ title }}</span>
      <el-button v-if="closable" link size="small" class="panel-close" @click="emit('close')">
        收起
      </el-button>
    </div>

    <p v-if="!parse" class="panel-empty">没能读取程序正文，本次没有识别结果。</p>

    <template v-else>
      <div class="panel-sub">识别结果</div>

      <dl class="rows">
        <div v-if="controllerText" class="row">
          <dt>数控系统</dt>
          <dd>{{ controllerText }}</dd>
        </div>

        <div class="row">
          <dt>行数</dt>
          <dd>{{ parse.lineCount }} 行</dd>
        </div>
      </dl>

      <!-- 识别到的刀具：可改、可删、可补，确认没问题再加入 -->
      <div class="tool-head">
        <span class="tool-label">
          刀具
          <span v-if="rows.length" class="tool-count">{{ rows.length }} 把</span>
        </span>
        <el-button size="small" @click="addRow">+ 添加一行</el-button>
      </div>

      <p v-if="rows.length === 0" class="tool-empty">
        {{
          nothingRecognized
            ? '没有识别到刀具调用，可以点「+ 添加一行」自己填。'
            : '识别到的刀具已被删空，可以点「+ 添加一行」自己填。'
        }}
      </p>

      <table v-else class="parse-tool-table">
        <thead>
          <tr>
            <th class="col-seq">#</th>
            <th class="col-no">刀具号</th>
            <th class="col-no">刀补号</th>
            <th class="col-num">直径 (mm)</th>
            <th class="col-num">补偿量 (mm)</th>
            <th class="col-op"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in rows" :key="row.uid">
            <td class="col-seq">{{ index + 1 }}</td>
            <td>
              <el-input
                v-model="row.toolNo"
                size="small"
                placeholder="T01"
                :class="{ 'is-empty': row.toolNo.trim() === '' }"
              />
            </td>
            <td>
              <el-input v-model="row.offsetNo" size="small" placeholder="D01（可留空）" />
            </td>
            <td>
              <el-input-number
                v-model="row.toolDia"
                size="small"
                :min="0"
                :max="10000"
                :precision="3"
                :controls="false"
                :value-on-clear="0"
              />
            </td>
            <td>
              <!-- 补偿量允许负数：磨损修下去、半径补偿取反都会是负的 -->
              <el-input-number
                v-model="row.compAmount"
                size="small"
                :min="-10000"
                :max="10000"
                :precision="3"
                :controls="false"
                :value-on-clear="0"
              />
            </td>
            <td class="col-op">
              <el-button link type="danger" size="small" @click="removeRow(index)">删除</el-button>
            </td>
          </tr>
        </tbody>
      </table>

      <div class="tool-actions">
        <el-button
          size="small"
          type="primary"
          plain
          :disabled="!canAdd"
          :loading="adding"
          @click="addTools"
        >
          加入刀具补偿表
        </el-button>
        <span v-if="emptyToolNo" class="tool-tip">刀具号不能为空</span>
        <span v-else-if="rows.length" class="tool-tip">
          加入前可以先改，改错了在这里删掉就行
        </span>
      </div>

      <el-alert
        v-for="(text, index) in warnings"
        :key="index"
        class="panel-alert"
        type="warning"
        :closable="false"
        show-icon
        :title="text"
      />
    </template>
  </div>
</template>

<style scoped>
.parse-panel {
  margin-top: 10px;
  padding: 10px 12px 12px;
  border: 1px solid var(--el-color-success-light-5);
  border-radius: 4px;
  background: var(--el-color-success-light-9);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.panel-title {
  color: var(--el-color-success);
  font-size: 13px;
  font-weight: 600;
}

.panel-close {
  flex: 0 0 auto;
}

.panel-sub {
  margin: 8px 0 6px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  font-weight: 600;
}

.panel-empty {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.rows {
  margin: 0 0 10px;
}

.row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 2px 0;
}

.row dt {
  flex: 0 0 64px;
  color: var(--el-text-color-regular);
  font-size: 12px;
}

.row dd {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 13px;
  word-break: break-all;
}

.tool-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.tool-label {
  color: var(--el-text-color-regular);
  font-size: 13px;
  font-weight: 600;
}

.tool-count {
  margin-left: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  font-weight: 400;
}

.tool-empty {
  margin: 4px 0 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.parse-tool-table {
  width: 100%;
  max-width: 720px;
  border-collapse: collapse;
  table-layout: fixed;
}

.parse-tool-table th,
.parse-tool-table td {
  padding: 4px 6px;
  text-align: left;
  vertical-align: middle;
}

.parse-tool-table th {
  color: var(--el-text-color-regular);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

.col-seq {
  width: 32px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.col-no {
  width: 120px;
}

.col-num {
  width: 130px;
}

.col-op {
  width: 52px;
  text-align: right;
}

.parse-tool-table :deep(.el-input-number) {
  width: 100%;
}

/* 刀具号空着时给个红边，别等点了加入才报错 */
.parse-tool-table :deep(.is-empty .el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--el-color-danger) inset;
}

.tool-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
}

.tool-tip {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.panel-alert {
  margin-top: 8px;
}
</style>
