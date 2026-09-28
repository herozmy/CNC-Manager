<script setup lang="ts">
/**
 * 主界面：单页线性表单，没有页签、没有左树。
 *
 *   ┌────────────┬─────────────────────────────────────────┐
 *   │ 图纸列表    │ 图纸卡片 → 工序卡片（多个）→ 程序（多个）  │
 *   │ （平铺）    │            → 每个程序的刀具补偿表          │
 *   └────────────┴─────────────────────────────────────────┘
 *
 * 数据流：
 *   - 选中图纸后只发一个请求 GET /api/drawings/{id}/detail，把整页要的数据一次拿到
 *   - 图纸的保存 / 新建 / 删除在本组件处理（会牵动左侧列表）
 *   - 工序与程序的保存由 OperationCard 自己处理，结构变化时抛 reload 让这里重拉详情
 *   - 搜索框 300ms 防抖后请求 GET /api/drawings?q=
 */
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import DrawingList from '../components/DrawingList.vue'
import DrawingCard from '../components/DrawingCard.vue'
import OperationCard from '../components/OperationCard.vue'
import {
  createDrawing,
  createOperation,
  deleteDrawing,
  getDrawingDetail,
  listDrawings,
  updateDrawing
} from '../api'
import { errorMessage } from '../api/client'
import type { Drawing, DrawingDetail, DrawingInput } from '../api/types'
import UpdateDialog from '../components/UpdateDialog.vue'
import { useUpdate } from '../composables/useUpdate'
import { formatOpNo, parseOpNo } from '../utils/format'

/* ------------------------------------------------------------ 图纸列表 */

const drawings = ref<Drawing[]>([])
const drawingsLoading = ref(false)
const keyword = ref('')
const selectedId = ref<number | null>(null)

let searchTimer: number | undefined

/** 顶部错误条：列表或详情请求失败时显示，可点「重试」 */
const loadError = ref('')

async function loadDrawings(): Promise<void> {
  // 显式调用时取消掉待触发的防抖请求，避免重复请求
  if (searchTimer !== undefined) {
    window.clearTimeout(searchTimer)
    searchTimer = undefined
  }
  drawingsLoading.value = true
  try {
    const page = await listDrawings({ q: keyword.value.trim(), size: 200 })
    drawings.value = page.items
    loadError.value = ''
  } catch (error) {
    loadError.value = errorMessage(error)
  } finally {
    drawingsLoading.value = false
  }
}

watch(keyword, () => {
  if (searchTimer !== undefined) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    searchTimer = undefined
    void loadDrawings()
  }, 300)
})

/* ------------------------------------------------------------ 图纸详情 */

const detail = ref<DrawingDetail | null>(null)
const detailLoading = ref(false)
const savingDrawing = ref(false)

const operations = computed(() =>
  [...(detail.value?.operations ?? [])].sort((a, b) => a.opNo - b.opNo)
)

async function loadDetail(): Promise<void> {
  const id = selectedId.value
  if (id === null) {
    detail.value = null
    return
  }
  detailLoading.value = true
  loadError.value = ''
  try {
    detail.value = await getDrawingDetail(id)
  } catch (error) {
    detail.value = null
    loadError.value = errorMessage(error)
  } finally {
    detailLoading.value = false
  }
}

async function selectDrawing(id: number): Promise<void> {
  selectedId.value = id
  await loadDetail()
}

async function retry(): Promise<void> {
  await loadDrawings()
  if (selectedId.value !== null) {
    await loadDetail()
  } else if (drawings.value.length > 0) {
    await selectDrawing(drawings.value[0].id)
  }
}

onMounted(async () => {
  await loadDrawings()
  if (drawings.value.length > 0) {
    await selectDrawing(drawings.value[0].id)
  }
})

/* -------------------------------------------------- 版本号与更新 */

/**
 * appVersion 显示在左侧面板底部：现场排查问题时第一件事就是确认装的是哪一版。
 *
 * 顺带查仓库上有没有新版本，有就在版本号旁边挂一个可点的角标。
 * 查不到（车间没网）时安静地什么都不显示。
 */
const {
  version: appVersion,
  canInstall,
  status: updateStatus,
  checking: updateChecking,
  install: installPackage
} = useUpdate()

const updateVisible = ref(false)
const installing = ref(false)
const installMessage = ref('')

/**
 * 装上用户选的离线包。
 *
 * 服务会退出、换文件、再重启，整个过程由 installPackage 一路等到新版本起来。
 * 等到了就刷新页面——新版本的前端文件也一起换过了，不刷新用的是旧的。
 */
async function handleInstall(file: File): Promise<void> {
  installing.value = true
  installMessage.value = `正在上传并安装 ${file.name}…`
  try {
    const result = await installPackage(file)
    installMessage.value = `已安装 ${result.version}，服务已重启，正在刷新页面…`
    window.location.reload()
  } catch (error) {
    installMessage.value = errorMessage(error)
  } finally {
    installing.value = false
  }
}

/* -------------------------------------------------------- 图纸写操作 */

async function handleSaveDrawing(input: DrawingInput): Promise<void> {
  const id = selectedId.value
  if (id === null) return
  savingDrawing.value = true
  try {
    const updated = await updateDrawing(id, input)
    if (detail.value) detail.value = { ...detail.value, drawing: updated }
    // 左侧列表按图纸号排序，改了图纸号要重新拉
    await loadDrawings()
    ElMessage.success('图纸已保存')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingDrawing.value = false
  }
}

async function handleDeleteDrawing(): Promise<void> {
  const current = detail.value?.drawing
  if (!current) return
  try {
    await ElMessageBox.confirm(
      `确定删除图纸 ${current.drawingNo}？该图纸下的所有工序、程序和 NC 版本记录会一并级联删除，且不可恢复。`,
      '删除图纸',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteDrawing(current.id)
    ElMessage.success('图纸已删除')
    detail.value = null
    selectedId.value = null
    await loadDrawings()
    if (drawings.value.length > 0) await selectDrawing(drawings.value[0].id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

/* ------------------------------------------------------ 新建图纸对话框 */

const createDrawingVisible = ref(false)
const creatingDrawing = ref(false)
const newDrawing = reactive({ drawingNo: '', name: '', material: '' })

function openCreateDrawing(): void {
  newDrawing.drawingNo = ''
  newDrawing.name = ''
  newDrawing.material = ''
  createDrawingVisible.value = true
}

async function confirmCreateDrawing(): Promise<void> {
  const drawingNo = newDrawing.drawingNo.trim()
  if (!drawingNo) {
    ElMessage.error('图纸号不能为空')
    return
  }
  creatingDrawing.value = true
  try {
    // 新建图纸的隐藏字段（客户 / 图纸版本）没有原值可取，按契约给空值
    const created = await createDrawing({
      drawingNo,
      name: newDrawing.name,
      customer: '',
      material: newDrawing.material,
      drawingVersion: '',
      remark: ''
    })
    createDrawingVisible.value = false
    ElMessage.success('图纸已新建')
    keyword.value = ''
    await loadDrawings()
    await selectDrawing(created.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    creatingDrawing.value = false
  }
}

/* ------------------------------------------------------ 添加工序对话框 */

const createOpVisible = ref(false)
const creatingOp = ref(false)
const newOpNoText = ref('')
const newOpNoError = ref('')

function openCreateOperation(): void {
  const maxOpNo = operations.value.reduce((max, item) => Math.max(max, item.opNo), 0)
  // 新工序号默认取当前最大工序号 + 10，中间留出插入余量
  newOpNoText.value = formatOpNo(maxOpNo + 10)
  newOpNoError.value = ''
  createOpVisible.value = true
}

async function confirmCreateOperation(): Promise<void> {
  const drawingId = selectedId.value
  if (drawingId === null) return

  const opNo = parseOpNo(newOpNoText.value)
  if (opNo === null) {
    newOpNoError.value = '工序号必须是 1~9999 之间的整数，例如 10# 或 10'
    return
  }
  newOpNoError.value = ''
  creatingOp.value = true
  try {
    // 新工序的隐藏字段（工序名称 / 机台）没有原值可取，按契约给空值
    await createOperation(drawingId, {
      opNo,
      opName: '',
      machineId: null,
      fixture: '',
      zHeight: 0,
      remark: ''
    })
    createOpVisible.value = false
    ElMessage.success('工序已添加')
    await loadDetail()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    creatingOp.value = false
  }
}
</script>

<template>
  <div class="main-view">
    <DrawingList
      v-model:keyword="keyword"
      :drawings="drawings"
      :selected-id="selectedId"
      :loading="drawingsLoading"
      :version="appVersion"
      :has-update="updateStatus?.hasUpdate === true"
      :latest-version="updateStatus?.latest ?? ''"
      @select="selectDrawing"
      @create="openCreateDrawing"
      @update="updateVisible = true"
    />

    <main class="detail">
      <div v-if="loadError" class="error-bar">
        <span class="error-text">{{ loadError }}</span>
        <el-button link type="primary" @click="retry">重试</el-button>
      </div>

      <div v-loading="detailLoading" class="detail-body">
        <template v-if="detail">
          <DrawingCard
            :drawing="detail.drawing"
            :saving="savingDrawing"
            @save="handleSaveDrawing"
            @remove="handleDeleteDrawing"
          />

          <div class="section-head">
            <span class="section-title">工序</span>
            <span class="section-count">{{ operations.length }} 道</span>
          </div>

          <OperationCard
            v-for="item in operations"
            :key="item.id"
            :operation="item"
            @reload="loadDetail"
          />

          <p v-if="operations.length === 0" class="empty-block">
            这张图纸还没有工序，点击下方「+ 添加工序号」新增一道。
          </p>

          <el-button class="add-op" @click="openCreateOperation">+ 添加工序号</el-button>
        </template>

        <div v-else-if="!detailLoading" class="empty-block big">
          <p v-if="loadError" class="empty-line">数据没能加载出来，请点上方「重试」。</p>
          <template v-else-if="drawings.length === 0">
            <p class="empty-line">还没有图纸</p>
            <el-button type="primary" @click="openCreateDrawing">+ 新建图纸</el-button>
          </template>
          <p v-else class="empty-line">从左侧选择一张图纸</p>
        </div>
      </div>
    </main>

    <el-dialog v-model="createDrawingVisible" title="新建图纸" width="460px">
      <label class="form-field">
        <span class="form-label">图纸号</span>
        <el-input
          v-model="newDrawing.drawingNo"
          placeholder="如 A-1003"
          maxlength="64"
          @keyup.enter="confirmCreateDrawing"
        />
      </label>
      <label class="form-field">
        <span class="form-label">名称</span>
        <el-input v-model="newDrawing.name" placeholder="如 主轴支架" />
      </label>
      <label class="form-field">
        <span class="form-label">材料</span>
        <el-input v-model="newDrawing.material" placeholder="如 45#" />
      </label>
      <template #footer>
        <el-button @click="createDrawingVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingDrawing" @click="confirmCreateDrawing">
          确定
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="createOpVisible" title="添加工序号" width="420px">
      <label class="form-field">
        <span class="form-label">工序号</span>
        <el-input
          v-model="newOpNoText"
          placeholder="如 10# 或 10"
          @keyup.enter="confirmCreateOperation"
        />
      </label>
      <p v-if="newOpNoError" class="form-error">{{ newOpNoError }}</p>
      <template #footer>
        <el-button @click="createOpVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingOp" @click="confirmCreateOperation">
          确定
        </el-button>
      </template>
    </el-dialog>

    <UpdateDialog
      v-model="updateVisible"
      :status="updateStatus"
      :can-install="canInstall"
      :current-version="appVersion"
      :checking="updateChecking"
      :installing="installing"
      :install-message="installMessage"
      @install="handleInstall"
    />
  </div>
</template>

<style scoped>
.main-view {
  display: flex;
  height: 100%;
  min-height: 0;
  background: var(--el-bg-color-page);
}

.detail {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.error-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--el-color-danger-light-7);
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger);
  font-size: 13px;
}

.error-text {
  flex: 1;
  min-width: 0;
}

.detail-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 20px 32px;
}

.section-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 20px 0 10px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
}

.section-count {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.empty-block {
  margin: 8px 0 12px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.empty-block.big {
  margin-top: 15vh;
  text-align: center;
}

.empty-line {
  margin: 0 0 14px;
  font-size: 14px;
}

.add-op {
  width: 100%;
  margin-top: 12px;
}

.form-field {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.form-label {
  flex: 0 0 56px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  text-align: right;
}

.form-field :deep(.el-input) {
  flex: 1;
}

.form-error {
  margin: 0;
  color: var(--el-color-danger);
  font-size: 12px;
}
</style>
