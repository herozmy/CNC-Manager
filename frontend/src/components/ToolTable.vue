<script setup lang="ts">
/**
 * 刀具补偿表（每个程序一张）。
 *
 * 只显示 5 列：序号 / 刀具号 / 刀补号 / 直径 / 补偿量。
 * 转速与进给按现场反馈去掉，界面上不再出现。
 *
 * 两个关键点：
 *  1) 整表提交（PUT /api/programs/{id}/tools），有「未保存」提示。
 *  2) PUT 是**整体替换**语义，所以界面上不显示的列
 *     （toolName / cornerRadius / spindleSpeed / speedMode / feed / feedMode /
 *      cutDepth / coolant / machiningContent / remark / toolId）
 *     必须从加载到的原始行**原样回传**，不能填死默认值，
 *     否则整表替换会把后端已有数据清空。
 *     只有「新增的行」才用这些隐藏字段的默认值。
 */
import { computed, ref, watch } from 'vue'
import type { ProgramTool, ProgramToolInput } from '../api/types'
import { toNumber } from '../utils/format'

const props = defineProps<{
  /** 从 /detail 加载到的刀具行 */
  tools: ProgramTool[]
  /** 整表保存中 */
  saving: boolean
}>()

const emit = defineEmits<{
  save: [items: ProgramToolInput[]]
}>()

/**
 * 表格里的一行。
 * 上半部分是界面上可见的列；下半部分是**只为原样回传而保留**的隐藏列。
 */
interface ToolRow {
  uid: number
  toolNo: string
  offsetNo: string
  toolDia: number
  compAmount: number

  // ---- 以下字段界面不显示，只用于原样回传，避免整表替换丢数据 ----
  toolId: number | null
  toolName: string
  cornerRadius: number
  spindleSpeed: number
  speedMode: number
  feed: number
  feedMode: number
  cutDepth: number
  coolant: number
  machiningContent: string
  remark: string
}

let uidSeed = 0

const rows = ref<ToolRow[]>([])
const savedSnapshot = ref('')

/** 新建一行：可见列给上常用的 T/D 编号，隐藏列全部给默认值 */
function createEmptyRow(): ToolRow {
  uidSeed += 1
  const index = rows.value.length + 1
  return {
    uid: uidSeed,
    toolNo: `T${String(index).padStart(2, '0')}`,
    offsetNo: `D${String(index).padStart(2, '0')}`,
    toolDia: 0,
    compAmount: 0,
    toolId: null,
    toolName: '',
    cornerRadius: 0,
    spindleSpeed: 0,
    speedMode: 0,
    feed: 0,
    feedMode: 0,
    cutDepth: 0,
    coolant: 0,
    machiningContent: '',
    remark: ''
  }
}

/** 从后端数据重建表格。已有行的隐藏列一律沿用后端返回的原值。 */
function reset(): void {
  rows.value = props.tools.map((tool) => {
    uidSeed += 1
    return {
      uid: uidSeed,
      toolNo: tool.toolNo,
      offsetNo: tool.offsetNo,
      toolDia: toNumber(tool.toolDia),
      compAmount: toNumber(tool.compAmount),
      toolId: tool.toolId,
      toolName: tool.toolName,
      cornerRadius: toNumber(tool.cornerRadius),
      spindleSpeed: Math.round(toNumber(tool.spindleSpeed)),
      speedMode: toNumber(tool.speedMode),
      feed: toNumber(tool.feed),
      feedMode: toNumber(tool.feedMode),
      cutDepth: toNumber(tool.cutDepth),
      coolant: toNumber(tool.coolant),
      machiningContent: tool.machiningContent,
      remark: tool.remark
    }
  })
  savedSnapshot.value = JSON.stringify(buildItems())
}

watch(() => props.tools, reset, { immediate: true })

function buildItems(): ProgramToolInput[] {
  return rows.value.map((row, index) => ({
    // 行号按当前顺序重排，保证后端「行号不重复」的校验必过
    seq: index + 1,
    toolId: row.toolId,
    toolNo: row.toolNo,
    offsetNo: row.offsetNo,
    toolName: row.toolName,
    toolDia: toNumber(row.toolDia),
    cornerRadius: toNumber(row.cornerRadius),
    compAmount: toNumber(row.compAmount),
    spindleSpeed: Math.round(toNumber(row.spindleSpeed)),
    speedMode: row.speedMode,
    feed: toNumber(row.feed),
    feedMode: row.feedMode,
    cutDepth: toNumber(row.cutDepth),
    coolant: toNumber(row.coolant),
    machiningContent: row.machiningContent,
    remark: row.remark
  }))
}

const dirty = computed(() => JSON.stringify(buildItems()) !== savedSnapshot.value)

function addRow(): void {
  rows.value.push(createEmptyRow())
}

function removeRow(index: number): void {
  rows.value.splice(index, 1)
}

function submit(): void {
  if (!dirty.value) return
  emit('save', buildItems())
}
</script>

<template>
  <div class="tool-block">
    <div class="tool-head">
      <span class="tool-title">刀具补偿</span>
      <div class="tool-actions">
        <el-button size="small" @click="addRow">+ 添加刀具</el-button>
        <span v-if="dirty" class="state state-dirty">未保存</span>
        <el-button
          size="small"
          type="primary"
          :disabled="!dirty"
          :loading="saving"
          @click="submit"
        >
          保存刀具补偿
        </el-button>
      </div>
    </div>

    <p v-if="rows.length === 0" class="tool-empty">
      还没有刀具数据，点击「+ 添加刀具」新增一行。
    </p>

    <table v-else class="tool-table">
      <thead>
        <tr>
          <th class="col-seq">序号</th>
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
          <td><el-input v-model="row.toolNo" size="small" placeholder="T01" /></td>
          <td><el-input v-model="row.offsetNo" size="small" placeholder="D01" /></td>
          <td>
            <el-input-number
              v-model="row.toolDia"
              size="small"
              :min="0"
              :precision="3"
              :controls="false"
              :value-on-clear="0"
            />
          </td>
          <td>
            <el-input-number
              v-model="row.compAmount"
              size="small"
              :min="0"
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
  </div>
</template>

<style scoped>
.tool-block {
  margin-top: 10px;
  padding: 10px 12px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  background: var(--el-fill-color-blank);
}

.tool-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 12px;
  margin-bottom: 10px;
}

.tool-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.tool-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.state {
  font-size: 12px;
}

.state-dirty {
  color: var(--el-color-warning);
}

.tool-empty {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.tool-table {
  width: 100%;
  max-width: 780px;
  border-collapse: collapse;
  table-layout: fixed;
}

.tool-table th,
.tool-table td {
  padding: 6px 6px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  text-align: left;
  vertical-align: middle;
}

.tool-table th {
  color: var(--el-text-color-regular);
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

.tool-table tbody tr:last-child td {
  border-bottom: none;
}

.col-seq {
  width: 48px;
  color: var(--el-text-color-secondary);
}

.col-no {
  width: 130px;
}

.col-num {
  width: 150px;
}

.col-op {
  width: 56px;
  text-align: right;
}

.tool-table :deep(.el-input-number) {
  width: 100%;
}
</style>
