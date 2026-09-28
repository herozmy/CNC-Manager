/**
 * 「上传 NC → 展示识别结果」这条流程的共用逻辑。
 *
 * 上传入口目前有两处（工序卡片里的「上传 NC」和版本历史弹窗里的「上传新版本」），
 * 变更说明弹窗的文案与语义必须完全一致，所以统一收在这里，避免哪天新增入口漏掉。
 *
 * 注意：上传只负责把文件传上去并把识别结果（刀具）带回来展示，
 * **不做任何叫停弹窗**——程序号由人工在记录里输入，界面不与文件内容对比。
 */
import { ElMessageBox } from 'element-plus'
import { uploadVersion } from '../api'
import type { UploadVersionResult } from '../api/types'

/**
 * 弹变更说明 → 上传 → 直接返回结果。
 *
 * 返回 null 表示用户在上传前取消了；上传失败会抛异常，由调用方
 * 用 ElMessage.error(errorMessage(error)) 提示。
 *
 * programNo 是**记录里**的程序号（不是识别出来的），只用于弹窗标题。
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

  // 后端读文件失败时不会带 parse 字段（omitempty），调用方一律写成 result.parse ?? null
  return uploadVersion(programId, file, changeNote)
}
