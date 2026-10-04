/**
 * 所有后端接口函数（与后端真实契约一一对应）。
 *
 * 组件里只调用这里的函数，不直接拼 URL；接口调整时只需改这一个文件。
 * 注意：所有 PUT 都是**整体替换**语义，调用方必须把「没显示的字段」按原值一起提交。
 */
import { apiDelete, apiGet, apiPost, apiPut, apiUpload, buildUrl } from './client'
import type {
  AuthStatus,
  AuthUser,
  DiffResult,
  Drawing,
  DrawingDetail,
  DrawingInput,
  ContentInput,
  InstallResult,
  Machine,
  Meta,
  Operation,
  OperationInput,
  Page,
  ParseResult,
  Program,
  ProgramInput,
  ProgramTool,
  ProgramToolInput,
  UpdateStatus,
  UploadVersionResult,
  Version,
  VersionContent
} from './types'

/* ------------------------------------------------------------------ 认证 */

export function getAuthStatus(): Promise<AuthStatus> {
  return apiGet<AuthStatus>('/auth/status')
}

export function getCurrentUser(): Promise<AuthUser> {
  return apiGet<AuthUser>('/auth/me')
}

export function login(username: string, password: string): Promise<AuthUser> {
  return apiPost<AuthUser>('/auth/login', { username, password })
}

export function setupAdmin(username: string, displayName: string, password: string): Promise<AuthUser> {
  return apiPost<AuthUser>('/auth/setup', { username, displayName, password })
}

export function logout(): Promise<void> {
  return apiPost<void>('/auth/logout')
}

/* ---------------------------------------------------------------- 元信息 */

/** 服务元信息：版本号、服务器时间、数据目录 */
export function getMeta(): Promise<Meta> {
  return apiGet<Meta>('/meta')
}

/* ------------------------------------------------------------------ 图纸 */

/** 图纸列表（左侧平铺列表用 size=200 一次拉全） */
export function listDrawings(
  params: { q?: string; page?: number; size?: number } = {}
): Promise<Page<Drawing>> {
  return apiGet<Page<Drawing>>('/drawings', { q: params.q, page: params.page, size: params.size })
}

export function getDrawing(id: number): Promise<Drawing> {
  return apiGet<Drawing>(`/drawings/${id}`)
}

/** 图纸详情：图纸 → 工序 → 程序 → 刀具补偿表，选中图纸后只发这一个请求 */
export function getDrawingDetail(id: number): Promise<DrawingDetail> {
  return apiGet<DrawingDetail>(`/drawings/${id}/detail`)
}

export function createDrawing(input: DrawingInput): Promise<Drawing> {
  return apiPost<Drawing>('/drawings', input)
}

export function updateDrawing(id: number, input: DrawingInput): Promise<Drawing> {
  return apiPut<Drawing>(`/drawings/${id}`, input)
}

export function deleteDrawing(id: number): Promise<void> {
  return apiDelete(`/drawings/${id}`)
}

/* ------------------------------------------------------------------ 工序 */

export function createOperation(drawingId: number, input: OperationInput): Promise<Operation> {
  return apiPost<Operation>(`/drawings/${drawingId}/operations`, input)
}

export function updateOperation(id: number, input: OperationInput): Promise<Operation> {
  return apiPut<Operation>(`/operations/${id}`, input)
}

export function deleteOperation(id: number): Promise<void> {
  return apiDelete(`/operations/${id}`)
}

/* ------------------------------------------------------------------ 程序 */

export function createProgram(operationId: number, input: ProgramInput): Promise<Program> {
  return apiPost<Program>(`/operations/${operationId}/programs`, input)
}

export function updateProgram(id: number, input: ProgramInput): Promise<Program> {
  return apiPut<Program>(`/programs/${id}`, input)
}

export function deleteProgram(id: number): Promise<void> {
  return apiDelete(`/programs/${id}`)
}

/* -------------------------------------------------------------- 刀具刀补 */

/** 某个程序当前的刀具补偿行（「加入刀具补偿表」先拿它做去重比较） */
export function getProgramTools(programId: number): Promise<ProgramTool[]> {
  return apiGet<ProgramTool[]>(`/programs/${programId}/tools`)
}

/**
 * 整表提交刀具补偿。
 * 未显示的列必须按加载到的原值带回（见 ProgramToolInput 的注释）。
 */
export function saveProgramTools(
  programId: number,
  items: ProgramToolInput[]
): Promise<ProgramTool[]> {
  return apiPut<ProgramTool[]>(`/programs/${programId}/tools`, { items })
}

/* ------------------------------------------------------------------ 版本 */

export function listVersions(programId: number): Promise<Version[]> {
  return apiGet<Version[]>(`/programs/${programId}/versions`)
}

/** 上传新版本（multipart/form-data：file + changeNote）；返回体里带正文识别结果 parse */
export function uploadVersion(
  programId: number,
  file: File,
  changeNote: string
): Promise<UploadVersionResult> {
  const form = new FormData()
  form.append('file', file)
  form.append('changeNote', changeNote)
  return apiUpload<UploadVersionResult>(`/programs/${programId}/versions`, form)
}

/** 版本文件下载地址（后端返回 attachment，直接跳转即下载） */
export function versionDownloadUrl(versionId: number): string {
  return buildUrl(`/versions/${versionId}/download`)
}

/** 版本逐行对比；against 传旧版，路径里的 id 是新版 */
export function diffVersions(versionId: number, againstVersionId: number): Promise<DiffResult> {
  return apiGet<DiffResult>(`/versions/${versionId}/diff`, { against: againstVersionId })
}

/** 设置程序当前生效版本 */
export function setCurrentVersion(programId: number, versionId: number): Promise<Program> {
  return apiPut<Program>(`/programs/${programId}/current-version`, { versionId })
}

/* ------------------------------------------------------------ 程序文本 */

/**
 * 读取某个版本的程序文本。
 *
 * 返回的 content 已解码成 UTF-8，可直接显示；
 * 同时返回 encoding，保存时必须原样带回去（见 ContentInput 的注释）。
 */
export function getVersionContent(versionId: number): Promise<VersionContent> {
  return apiGet<VersionContent>(`/versions/${versionId}/content`)
}

/**
 * 用编辑好的文本**覆盖这一版**的内容，版本号不变。
 *
 * 整体替换语义：原内容不再保留，调用前必须让用户二次确认。
 */
export function overwriteVersionContent(
  versionId: number,
  input: ContentInput
): Promise<Version> {
  return apiPut<Version>(`/versions/${versionId}/content`, input)
}

/** 把编辑好的文本存成该程序的**新版本**（原版本保留，可回溯） */
export function saveVersionContentAsNew(
  programId: number,
  input: ContentInput
): Promise<Version> {
  return apiPost<Version>(`/programs/${programId}/versions/content`, input)
}

/* ------------------------------------------------------------ 程序自动识别 */

/**
 * 解析一段 NC 文本，返回识别到的程序号 / 数控系统 / 刀具调用 / 警告。
 *
 * 内容为空或超过 4MB 时后端返回 400 + { error }，这里会抛出 ApiError。
 */
export function parseNcText(content: string): Promise<ParseResult> {
  return apiPost<ParseResult>('/nc/parse', { content })
}

/* ------------------------------------------------------------------ 机台 */

/** 机台字典。界面上不展示机台选择，保留该函数仅为契约完整性。 */
export function listMachines(): Promise<Machine[]> {
  return apiGet<Machine[]>('/machines')
}

/* ---------------------------------------------------------------- 版本更新 */

/**
 * 问仓库上有没有新版本。
 *
 * 查不到不会抛错，而是在返回值里带一个 error 字段——车间没网是常态，
 * 界面安静地什么都不显示就好。
 *
 * @param fresh 用户手工点「检查更新」时传 true，绕过后端缓存重新问一次
 */
export function checkUpdate(fresh = false): Promise<UpdateStatus> {
  return apiGet<UpdateStatus>('/update/check', fresh ? { fresh: 1 } : undefined)
}

/**
 * 上传离线包安装。
 *
 * 服务端校验通过后会主动退出，由替换脚本换掉 exe 和 web 再重新拉起，
 * 所以这个请求成功返回只代表「已受理」，此时服务马上就会断开。
 * 要等服务重新起来请用 waitForRestart。
 */
export function installUpdate(file: File): Promise<InstallResult> {
  const form = new FormData()
  form.append('file', file)
  return apiUpload<InstallResult>('/update/install', form)
}
