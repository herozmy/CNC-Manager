/// <reference types="vite/client" />

/**
 * 环境变量类型声明。
 * 注意：这里刻意不为 `*.vue` 声明通配模块——vue-tsc 会直接对真实的单文件组件做类型检查，
 * 一旦加了通配声明反而会屏蔽组件的 props / emits 类型校验。
 */
interface ImportMetaEnv {
  /** 应用标题（可选，默认「CNC 加工程序管理系统」） */
  readonly VITE_APP_TITLE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
