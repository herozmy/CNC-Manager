<script setup lang="ts">
/**
 * NC 自动识别结果面板。
 *
 * 两个地方共用：
 *   - 上传成功后，就地贴在上传位置（OperationCard / VersionDialog），带版本号标题、
 *     「加入刀具补偿表」按钮，程序号对不上时还有「把记录改成 xxx」；
 *   - 只读查看程序时的「识别程序」（ProgramEditorDialog），只有识别内容，没有记录可比。
 *
 * 判空纪律：parse 可能是 null（后端读文件失败），tools 可能是空数组，
 * offsetNo 可能是空串——都要显示成明确的「未识别到」而不是空白。
 */
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import type { ParseResult, ProgramToolInput } from '../api/types'
import { getProgramTools, saveProgramTools } from '../api'
import { errorMessage } from '../api/client'
import {
  formatToolLabel,
  isMismatchWarning,
  normalizeToolNo,
  programNoEqual
} from '../utils/nc'

const props = defineProps<{
  /** 识别结果；后端读取程序正文失败时为 null */
  parse: ParseResult | null
  /** 用于「加入刀具补偿表」 */
  programId: number
  /**
   * 记录里当前的程序号。传了才做「与记录是否一致」的判断；
   * 只读识别（没有记录可比）时不传，那一列直接不出现。
   */
  recordProgramNo?: string
  /** 面板标题，例如「第 2 版上传成功」 */
  title?: string
  /** 是否显示「加入刀具补偿表」按钮 */
  allowAddTools?: boolean
  /** 是否显示「收起」 */
  closable?: boolean
}>()

const emit = defineEmits<{
  /** 用户点了「把记录改成 xxx」；父组件负责整体替换保存（隐藏字段要按原值带回） */
  fixProgramNo: [programNo: string]
  /** 刀具已写进刀具补偿表，父组件提示并刷新 */
  toolsAdded: [count: number]
  close: []
}>()

/* ------------------------------------------------------------ 展示数据 */

const tools = computed(() => props.parse?.tools ?? [])

const recognizedNo = computed(() => props.parse?.programNo ?? '')
const rawNo = computed(() => props.parse?.programNoRaw ?? '')
const controllerText = computed(() => props.parse?.controller || '未识别')

/** 刀具文案：T01/D01、T02/D02；offsetNo 为空时只显示刀号 */
const toolText = computed(() => tools.value.map(formatToolLabel).join('、'))

/** 其余警告内联展示即可（「程序号对不上」由下面那块红色区域负责，不重复贴一遍） */
const otherWarnings = computed(() =>
  (props.parse?.warnings ?? []).filter((text) => !isMismatchWarning(text))
)

/**
 * 是否与记录对不上。
 *
 * 判断口径与后端 programNoEqual 完全一致（见 utils/nc.ts），这样改完程序号、
 * 父组件刷新出新记录后，这里会实时从红变绿，不会被上传那一刻的旧警告卡在红色。
 */
const mismatch = computed(() => {
  if (props.recordProgramNo === undefined) return false
  if (!recognizedNo.value) return false
  return !programNoEqual(props.recordProgramNo, recognizedNo.value)
})

/* -------------------------------------------------- 把记录改成识别到的号 */

function requestFix(): void {
  if (!recognizedNo.value) return
  emit('fixProgramNo', recognizedNo.value)
}

/* -------------------------------------------------------- 加入刀具补偿表 */

const adding = ref(false)

async function addTools(): Promise<void> {
  const result = props.parse
  if (!result || result.tools.length === 0 || adding.value) return

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

    for (const tool of result.tools) {
      const key = normalizeToolNo(tool.toolNo)
      if (!key || known.has(key)) continue
      known.add(key)
      maxSeq += 1
      added.push({
        seq: maxSeq,
        toolId: null,
        toolNo: tool.toolNo,
        offsetNo: tool.offsetNo,
        toolName: '',
        toolDia: 0,
        cornerRadius: 0,
        compAmount: 0,
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
      ElMessage.info('刀具补偿表里已经有了')
      return
    }

    await saveProgramTools(props.programId, [...items, ...added])
    emit('toolsAdded', added.length)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    adding.value = false
  }
}
</script>

<template>
  <div class="parse-panel" :class="{ 'parse-panel-bad': mismatch }">
    <div v-if="title || closable" class="panel-head">
      <span v-if="title" class="panel-title">✓ {{ title }}</span>
      <el-button v-if="closable" link size="small" class="panel-close" @click="emit('close')">
        收起
      </el-button>
    </div>

    <p v-if="!parse" class="panel-empty">没能读取程序正文，本次没有识别结果。</p>

    <template v-else>
      <div class="panel-sub">程序识别</div>

      <dl class="rows">
        <div class="row">
          <dt>程序号</dt>
          <dd>
            <template v-if="recognizedNo">
              <span class="no">{{ recognizedNo }}</span>
              <span v-if="mismatch" class="badge badge-bad">
                ✗ 与记录不一致，文件里是 {{ recognizedNo }}
              </span>
              <span v-else-if="recordProgramNo !== undefined" class="badge badge-ok">
                ✓ 与记录一致
              </span>
            </template>
            <span v-else class="badge badge-none">未识别到程序号</span>
            <span v-if="rawNo && rawNo !== recognizedNo" class="raw">原文：{{ rawNo }}</span>
          </dd>
        </div>

        <div class="row">
          <dt>数控系统</dt>
          <dd>{{ controllerText }}</dd>
        </div>

        <div class="row">
          <dt>行数</dt>
          <dd>{{ parse.lineCount }} 行</dd>
        </div>

        <div class="row">
          <dt>刀具</dt>
          <dd>
            <template v-if="tools.length > 0">
              <span class="tool-list">{{ toolText }}</span>
              <el-button
                v-if="allowAddTools"
                size="small"
                type="primary"
                plain
                :loading="adding"
                @click="addTools"
              >
                加入刀具补偿表
              </el-button>
            </template>
            <span v-else class="badge badge-none">未识别到刀具</span>
          </dd>
        </div>
      </dl>

      <div v-if="mismatch" class="fix-row">
        <el-button size="small" type="danger" plain @click="requestFix">
          把记录改成 {{ recognizedNo }}
        </el-button>
        <span class="fix-hint">确认文件没传错，再改正记录里的程序号</span>
      </div>

      <el-alert
        v-for="(text, index) in otherWarnings"
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

/* 程序号对不上：整块转红，现场一眼就能看见 */
.parse-panel-bad {
  border-color: var(--el-color-danger-light-5);
  background: var(--el-color-danger-light-9);
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

.parse-panel-bad .panel-title {
  color: var(--el-color-danger);
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
  margin: 0;
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

.no {
  font-family: Consolas, 'Courier New', monospace;
  font-weight: 600;
}

.badge {
  font-size: 12px;
}

.badge-ok {
  color: var(--el-color-success);
}

.badge-bad {
  color: var(--el-color-danger);
  font-weight: 600;
}

.badge-none {
  color: var(--el-text-color-placeholder);
}

.raw {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.tool-list {
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
}

.fix-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}

.fix-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.panel-alert {
  margin-top: 8px;
}
</style>
