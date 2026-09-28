<script setup lang="ts">
/**
 * 左侧图纸列表：平铺列表（不是树），每项只显示「图纸号 + 名称」。
 *
 * 只负责展示与交互，不碰接口：搜索关键字向上抛，由 MainView 做 300ms 防抖后请求。
 */
import type { Drawing } from '../api/types'

defineProps<{
  /** 图纸列表 */
  drawings: Drawing[]
  /** 当前选中的图纸 ID */
  selectedId: number | null
  /** 列表加载中 */
  loading: boolean
  /** 搜索关键字（受控） */
  keyword: string
  /** 后端版本号，来自 GET /api/meta；取不到时为空串 */
  version: string
  /** 仓库上有比当前更新的版本 */
  hasUpdate: boolean
  /** 仓库上的最新版本号，hasUpdate 为真时有意义 */
  latestVersion: string
}>()

const emit = defineEmits<{
  'update:keyword': [value: string]
  select: [id: number]
  create: []
  update: []
}>()

function onInput(value: string): void {
  emit('update:keyword', value)
}
</script>

<template>
  <aside class="drawing-list">
    <div class="list-head">
      <div class="list-title">图纸</div>
      <el-input
        :model-value="keyword"
        placeholder="搜索图纸号 / 名称"
        clearable
        @update:model-value="onInput"
      />
      <el-button class="new-btn" @click="emit('create')">+ 新建图纸</el-button>
    </div>

    <div v-loading="loading" class="list-body">
      <p v-if="!loading && drawings.length === 0" class="list-empty">
        {{ keyword.trim() ? '没有匹配的图纸' : '还没有图纸，先新建图纸' }}
      </p>

      <button
        v-for="item in drawings"
        :key="item.id"
        type="button"
        class="list-item"
        :class="{ active: item.id === selectedId }"
        @click="emit('select', item.id)"
      >
        <span class="item-no">{{ item.drawingNo }}</span>
        <span class="item-name">{{ item.name || '未命名' }}</span>
      </button>
    </div>

    <!-- 版本号放在左下角：现场排查问题时第一件事就是确认装的是哪一版 -->
    <div v-if="version" class="list-foot">
      <div class="foot-version">CNC 加工程序管理 {{ version }}</div>

      <!-- 有新版本时在版本号旁边点一下就能去装 -->
      <button v-if="hasUpdate" type="button" class="update-badge" @click="emit('update')">
        有新版本 {{ latestVersion }} · 去安装
      </button>
    </div>
  </aside>
</template>

<style scoped>
.drawing-list {
  display: flex;
  flex: 0 0 260px;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}

.list-head {
  padding: 14px 12px 10px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.list-title {
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 600;
}

.new-btn {
  width: 100%;
  margin-top: 8px;
}

.list-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px;
}

.list-empty {
  margin: 24px 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.7;
  text-align: center;
}

.list-item {
  display: flex;
  width: 100%;
  align-items: baseline;
  gap: 8px;
  padding: 8px 10px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--el-text-color-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.list-item:hover {
  background: var(--el-fill-color-light);
}

.list-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.item-no {
  font-weight: 600;
  white-space: nowrap;
}

.item-name {
  flex: 1;
  overflow: hidden;
  font-size: 13px;
  color: var(--el-text-color-regular);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-item.active .item-name {
  color: var(--el-color-primary);
}

.list-foot {
  padding: 8px 12px;
  border-top: 1px solid var(--el-border-color-lighter);
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  text-align: center;
}

.foot-version {
  white-space: nowrap;
}

/* 更新提示用主色：这是正常的升级，不是出错，别用警告色吓人 */
.update-badge {
  display: block;
  width: 100%;
  margin-top: 6px;
  padding: 4px 8px;
  border: 1px solid var(--el-color-primary-light-5);
  border-radius: 10px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}

.update-badge:hover {
  background: var(--el-color-primary-light-8);
}
</style>
