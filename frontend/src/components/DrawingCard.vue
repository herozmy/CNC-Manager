<script setup lang="ts">
/**
 * 图纸卡片：整页最上面的一张卡，只编辑「图纸号 / 名称 / 材料 / 备注」四个字段。
 *
 * 重要：后端 PUT /api/drawings/{id} 是**整体替换**语义。
 * 界面上刻意不显示的 customer（客户）与 drawingVersion（图纸版本）必须用
 * 从 /detail 加载到的原值原样提交，绝不能填死空串，否则会清空后端已有数据。
 */
import { computed, reactive, ref, watch } from 'vue'
import type { Drawing, DrawingInput } from '../api/types'

const props = defineProps<{
  drawing: Drawing
  /** 保存中 */
  saving: boolean
}>()

const emit = defineEmits<{
  save: [input: DrawingInput]
  remove: []
}>()

/** 界面上真正可编辑的字段 */
const form = reactive({
  drawingNo: '',
  name: '',
  material: '',
  remark: ''
})

/** 界面上不显示、但提交时必须原样带回的字段 */
const preserved = reactive({
  customer: '',
  drawingVersion: ''
})

/** 已落库的提交体快照，用来判断「有没有未保存的改动」 */
const savedSnapshot = ref('')

function buildInput(): DrawingInput {
  return {
    drawingNo: form.drawingNo.trim(),
    name: form.name,
    customer: preserved.customer,
    material: form.material,
    drawingVersion: preserved.drawingVersion,
    remark: form.remark
  }
}

function reset(): void {
  form.drawingNo = props.drawing.drawingNo
  form.name = props.drawing.name
  form.material = props.drawing.material
  form.remark = props.drawing.remark
  preserved.customer = props.drawing.customer
  preserved.drawingVersion = props.drawing.drawingVersion
  savedSnapshot.value = JSON.stringify(buildInput())
}

watch(() => props.drawing, reset, { immediate: true })

const dirty = computed(() => JSON.stringify(buildInput()) !== savedSnapshot.value)
const canSave = computed(() => form.drawingNo.trim().length > 0)

function submit(): void {
  if (!canSave.value) return
  emit('save', buildInput())
}
</script>

<template>
  <section class="card">
    <div class="card-head">
      <span class="card-title">图纸</span>
      <div class="card-actions">
        <span v-if="dirty" class="state state-dirty">未保存</span>
        <el-button
          type="primary"
          :disabled="!dirty || !canSave"
          :loading="saving"
          @click="submit"
        >
          保存图纸
        </el-button>
        <el-button :disabled="saving" @click="emit('remove')">删除图纸</el-button>
      </div>
    </div>

    <div class="fields">
      <label class="field">
        <span class="field-label">图纸号</span>
        <el-input v-model="form.drawingNo" placeholder="如 A-1001" maxlength="64" />
      </label>
      <label class="field">
        <span class="field-label">名称</span>
        <el-input v-model="form.name" placeholder="如 主轴支架" />
      </label>
      <label class="field">
        <span class="field-label">材料</span>
        <el-input v-model="form.material" placeholder="如 45#" />
      </label>
      <label class="field">
        <span class="field-label">备注</span>
        <el-input v-model="form.remark" placeholder="选填" />
      </label>
    </div>
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

.card-title {
  font-size: 14px;
  font-weight: 600;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.state {
  font-size: 12px;
}

.state-dirty {
  color: var(--el-color-warning);
}

.fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 24px;
}

.field {
  display: flex;
  align-items: center;
  gap: 10px;
}

.field-label {
  flex: 0 0 56px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  text-align: right;
}

.field :deep(.el-input) {
  flex: 1;
}
</style>
