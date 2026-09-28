/**
 * 「上传 NC → 展示识别结果 → 改正程序号」这条流程的共用逻辑。
 *
 * 上传入口目前有两处（工序卡片里的「上传 NC」和版本历史弹窗里的「上传新版本」），
 * 危险提醒与保存语义必须完全一致，所以统一收在这里，避免哪天新增入口漏掉弹窗。
 *
 * 两件不能妥协的事：
 *  1) 正文程序号和记录对不上时必须**立刻弹窗叫停**——传错文件轻则白干，重则撞刀。
 *  2) 改程序号走的是 PUT /api/programs/{id}，**整体替换**语义，
 *     programName / controller / remark 必须用加载到的原值带回，不能填死空串。
 */
import { ElMessage, ElMessageBox } from 'element-plus'
import { updateProgram, uploadVersion } from '../api'
import { errorMessage } from '../api/client'
import type { ParseResult, UploadVersionResult } from '../api/types'
import { hasProgramNoMismatch, isMismatchWarning } from './nc'

/**
 * 上传成功后的叫停弹窗。
 *
 * 只对「程序号对不上」这条警告弹窗；其余警告（没识别到程序号、同一把刀多个刀补号等）
 * 由识别结果面板用内联提示展示，不打扰用户。
 */
export async function alertProgramNoMismatch(parse: ParseResult | null): Promise<void> {
  if (!parse || !hasProgramNoMismatch(parse.warnings)) return
  const text = parse.warnings.find(isMismatchWarning) ?? parse.warnings[0] ?? ''
  try {
    await ElMessageBox.alert(text, '⚠ 程序号对不上', {
      confirmButtonText: '我知道了',
      type: 'warning'
    })
  } catch {
    // 用户直接关掉弹窗也算看过了，不能因为这个中断上传流程
  }
}

/**
 * 弹变更说明 → 上传 → 必要时弹窗叫停。
 *
 * 返回 null 表示用户在上传前取消了；上传失败会抛异常，由调用方
 * 用 ElMessage.error(errorMessage(error)) 提示。
 */
export async function uploadNcVersion(
  programId: number,
  file: File,
  programNo: string
): Promise<UploadVersionResult | null> {
  let changeNote = ''
  try {
    const answer = await ElMessageBox.prompt(
      '请填写本次变更说明（可留空）',
      `上传 NC · ${programNo}`,
      { confirmButtonText: '上传', cancelButtonText: '取消', inputPlaceholder: '如：提高主轴转速' }
    )
    changeNote = answer.value ?? ''
  } catch {
    return null
  }

  const result = await uploadVersion(programId, file, changeNote)
  // 后端读文件失败时不会带 parse 字段（omitempty），统一成 null 再往下传
  await alertProgramNoMismatch(result.parse ?? null)
  return result
}

/** 记录里界面上不显示、但整体替换时必须原值带回的字段 */
export interface ProgramNoFixSource {
  programNo: string
  programName: string
  controller: string
  remark: string
}

/**
 * 把记录里的程序号改成识别到的那个，其余字段按原值带回。
 * 返回 true 表示确实改成功了（调用方需要刷新数据）。
 */
export async function applyRecognizedProgramNo(
  programId: number,
  recognizedNo: string,
  source: ProgramNoFixSource
): Promise<boolean> {
  try {
    await ElMessageBox.confirm(
      `把记录里的程序号由 ${source.programNo || '（空）'} 改成 ${recognizedNo}？`,
      '改正程序号',
      { confirmButtonText: '改正', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return false
  }

  try {
    await updateProgram(programId, {
      programNo: recognizedNo,
      // 整体替换语义：没显示的字段按加载到的原值带回，不能填死空串
      programName: source.programName,
      controller: source.controller,
      remark: source.remark
    })
    ElMessage.success(`程序号已改成 ${recognizedNo}`)
    return true
  } catch (error) {
    ElMessage.error(errorMessage(error))
    return false
  }
}
