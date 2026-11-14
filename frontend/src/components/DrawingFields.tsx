import type { DrawingInput } from '../api/types'
import { Field, TextArea } from './ui/Field'

interface Props {
  value: DrawingInput
  onChange: (value: DrawingInput) => void
  noError?: string
  priceError?: string
}

/** 新建和编辑共用的图纸信息。 */
export default function DrawingFields({ value, onChange, noError, priceError }: Props) {
  const set = (key: keyof DrawingInput) => (e: { target: { value: string } }) => onChange({ ...value, [key]: e.target.value })
  return <>
    <Field label="图纸号" value={value.drawingNo} onChange={set('drawingNo')} error={noError} autoFocus autoCapitalize="characters" enterKeyHint="next" />
    <Field label="类型" value={value.name} onChange={set('name')} placeholder="如 轴类、壳体类、端盖类" hint="相同类型的产品可在总览中搜索查看" enterKeyHint="next" />
    <Field label="材料" value={value.material} onChange={set('material')} enterKeyHint="next" />
    <Field label="价格（元）" value={value.price} onChange={set('price')} inputMode="decimal" placeholder="未填写" hint="选填，最多两位小数" error={priceError} enterKeyHint="next" />
    <TextArea label="备注" value={value.remark} onChange={set('remark')} />
  </>
}
