<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createMachine, deleteMachine, listMachines, listMachineTools, saveMachineTools, updateMachine } from '../api'
import { errorMessage } from '../api/client'
import type { Machine, MachineInput, MachineToolInput } from '../api/types'

const visible = defineModel<boolean>({ required: true })
const machines = ref<Machine[]>([])
const selectedId = ref<number | null>(null)
const rows = ref<MachineToolInput[]>([])
const loading = ref(false)
const saving = ref(false)
const editingMachine = ref(false)
const editingMachineId = ref<number | null>(null)
const machineForm = reactive<MachineInput>({ code: '', name: '', controller: '', remark: '' })

async function loadMachines(selectId?: number): Promise<void> {
  machines.value = await listMachines()
  selectedId.value = selectId ?? selectedId.value ?? machines.value[0]?.id ?? null
  await loadTools()
}
async function loadTools(): Promise<void> {
  if (selectedId.value === null) { rows.value = []; return }
  loading.value = true
  try {
    rows.value = (await listMachineTools(selectedId.value)).map(({ pocketNo, toolNo, toolName, spec, remark }) => ({ pocketNo, toolNo, toolName, spec, remark }))
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { loading.value = false }
}
function addRow(): void {
  const used = new Set(rows.value.map(row => row.pocketNo))
  let pocketNo = 1
  while (used.has(pocketNo)) pocketNo++
  rows.value.push({ pocketNo, toolNo: '', toolName: '', spec: '', remark: '' })
}
async function saveTools(): Promise<void> {
  if (selectedId.value === null) return
  const pockets = rows.value.map(row => row.pocketNo)
  if (pockets.some(no => no < 1) || new Set(pockets).size !== pockets.length) { ElMessage.warning('刀位必须大于 0，且同一机床不能重复'); return }
  saving.value = true
  try { await saveMachineTools(selectedId.value, rows.value); ElMessage.success('刀库已保存'); await loadTools() }
  catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}
function openMachine(machine?: Machine): void {
	editingMachineId.value = machine?.id ?? null
  Object.assign(machineForm, machine ? { code: machine.code, name: machine.name, controller: machine.controller, remark: machine.remark } : { code: '', name: '', controller: '', remark: '' })
  editingMachine.value = true
}
async function saveMachine(): Promise<void> {
  if (!machineForm.code.trim() || !machineForm.name.trim()) { ElMessage.warning('请填写机床编号和名称'); return }
  try {
    const saved = editingMachineId.value !== null ? await updateMachine(editingMachineId.value, machineForm) : await createMachine(machineForm)
    editingMachine.value = false; await loadMachines(saved.id); ElMessage.success('机床已保存')
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
async function removeMachine(): Promise<void> {
  if (selectedId.value === null) return
  const current = machines.value.find(item => item.id === selectedId.value)
  try { await ElMessageBox.confirm(`删除“${current?.name}”及其刀库？`, '删除机床', { type: 'warning' }); await deleteMachine(selectedId.value); selectedId.value = null; await loadMachines(); ElMessage.success('机床已删除') }
  catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error)) }
}
watch(visible, value => { if (value) void loadMachines().catch(error => ElMessage.error(errorMessage(error))) })
watch(selectedId, () => { if (visible.value) void loadTools() })
</script>

<template>
  <el-dialog v-model="visible" title="机床刀库" width="1050px" destroy-on-close>
    <div class="layout">
      <aside class="machines">
        <div class="aside-head"><strong>机床</strong><el-button link type="primary" @click="openMachine()">+ 新建</el-button></div>
        <button v-for="machine in machines" :key="machine.id" class="machine" :class="{ active: selectedId === machine.id }" @click="selectedId = machine.id">
          <strong>{{ machine.name }}</strong><small>{{ machine.code }} · {{ machine.controller || '未填系统' }}</small>
        </button>
        <p v-if="machines.length === 0" class="empty">先新建机床，例如机床1、机床2</p>
      </aside>
      <section class="tools">
        <div class="toolbar">
          <span>{{ machines.find(item => item.id === selectedId)?.name || '请选择机床' }} 当前刀库</span>
          <span class="grow" />
          <el-button v-if="selectedId" @click="openMachine(machines.find(item => item.id === selectedId))">编辑机床</el-button>
          <el-button v-if="selectedId" type="danger" plain @click="removeMachine">删除机床</el-button>
          <el-button :disabled="!selectedId" @click="addRow">+ 添加刀位</el-button>
          <el-button type="primary" :loading="saving" :disabled="!selectedId" @click="saveTools">保存刀库</el-button>
        </div>
        <el-table v-loading="loading" :data="rows" height="430" empty-text="暂无刀具，点击“添加刀位”录入">
          <el-table-column label="刀位" width="105"><template #default="scope"><el-input-number v-model="scope.row.pocketNo" :min="1" :max="999" controls-position="right" /></template></el-table-column>
          <el-table-column label="刀具号" width="135"><template #default="scope"><el-input v-model="scope.row.toolNo" placeholder="如 T01" /></template></el-table-column>
          <el-table-column label="刀具名称" min-width="170"><template #default="scope"><el-input v-model="scope.row.toolName" placeholder="如 Ø10立铣刀" /></template></el-table-column>
          <el-table-column label="规格" min-width="150"><template #default="scope"><el-input v-model="scope.row.spec" placeholder="直径/长度等" /></template></el-table-column>
          <el-table-column label="备注" min-width="150"><template #default="scope"><el-input v-model="scope.row.remark" /></template></el-table-column>
          <el-table-column width="65"><template #default="scope"><el-button link type="danger" @click="rows.splice(scope.$index, 1)">删除</el-button></template></el-table-column>
        </el-table>
      </section>
    </div>
    <el-dialog v-model="editingMachine" title="机床信息" width="430px" append-to-body>
      <el-form label-width="80px"><el-form-item label="机床编号"><el-input v-model="machineForm.code" placeholder="如 MC01" /></el-form-item><el-form-item label="机床名称"><el-input v-model="machineForm.name" placeholder="如 机床1" /></el-form-item><el-form-item label="数控系统"><el-input v-model="machineForm.controller" placeholder="如 FANUC" /></el-form-item><el-form-item label="备注"><el-input v-model="machineForm.remark" /></el-form-item></el-form>
      <template #footer><el-button @click="editingMachine = false">取消</el-button><el-button type="primary" @click="saveMachine">保存</el-button></template>
    </el-dialog>
  </el-dialog>
</template>

<style scoped>
.layout{display:flex;gap:16px;min-height:430px}.machines{width:190px;border-right:1px solid var(--el-border-color);padding-right:14px}.aside-head,.toolbar{display:flex;align-items:center;gap:8px;margin-bottom:12px}.machine{display:flex;flex-direction:column;width:100%;padding:10px;margin-bottom:7px;text-align:left;border:1px solid var(--el-border-color);border-radius:6px;background:white;cursor:pointer}.machine.active{border-color:var(--el-color-primary);background:var(--el-color-primary-light-9)}.machine small{color:var(--el-text-color-secondary);margin-top:3px}.tools{min-width:0;flex:1}.grow{flex:1}.empty{color:var(--el-text-color-secondary);font-size:13px;line-height:1.7}
</style>
