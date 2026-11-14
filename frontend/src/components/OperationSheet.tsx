import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { Operation } from '../api/types'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import DeleteSheet from './DeleteSheet'
import { Sheet } from './ui/Sheet'

interface Props {
  drawingId: number
  /** 传入则为编辑，否则新增 */
  operation?: Operation
  /** 新增时建议的工序号（现有最大号 + 10） */
  suggestedNo?: number
  onClose: () => void
}

const fixtureOptions = ['虎钳', '压板', 'V型工装', '吸盘', '工装板'] as const
const customFixtureValue = '__custom__'
const axes = ['X', 'Y', 'Z'] as const

function initialFixtureChoice(fixture: string) {
  if (!fixture) return ''
  return fixtureOptions.includes(fixture as (typeof fixtureOptions)[number]) ? fixture : customFixtureValue
}

/** 新增 / 编辑工序。表单只在打开时挂载，所以每次打开都是干净的初始状态。 */
export default function OperationSheet({ drawingId, operation, suggestedNo = 10, onClose }: Props) {
  const qc = useQueryClient()
  const [opNo, setOpNo] = useState(String(operation?.opNo ?? suggestedNo))
  const [opName, setOpName] = useState(operation?.opName ?? '')
  const existingFixture = operation?.fixture ?? ''
  const [fixtureChoice, setFixtureChoice] = useState(initialFixtureChoice(existingFixture))
  const [customFixture, setCustomFixture] = useState(initialFixtureChoice(existingFixture) === customFixtureValue ? existingFixture : '')
  const [centeringMethod, setCenteringMethod] = useState(operation?.centeringMethod ?? '')
  const [coordinates, setCoordinates] = useState<Record<typeof axes[number], string>>({
    X: operation ? String(operation.xCoordinate ?? 0) : '',
    Y: operation ? String(operation.yCoordinate ?? 0) : '',
    Z: operation ? String(operation.zHeight) : '',
  })
  const [remark, setRemark] = useState(operation?.remark ?? '')
  const [noError, setNoError] = useState('')
  const [coordinateError, setCoordinateError] = useState<Partial<Record<typeof axes[number], string>>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: ['drawing', drawingId] }), qc.invalidateQueries({ queryKey: ['drawings'] })])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setNoError('')
    if (busy) return
    setCoordinateError({})
    setError('')
    const no = Number(opNo)
    if (!Number.isInteger(no) || no < 1 || no > 9999) return setNoError('工序号应为 1~9999 的整数')
    const values = { X: Number(coordinates.X.trim()), Y: Number(coordinates.Y.trim()), Z: Number(coordinates.Z.trim()) }
    const errors: Partial<Record<typeof axes[number], string>> = {}
    for (const axis of axes) {
      if (!Number.isFinite(values[axis]) || Math.abs(values[axis]) > 100000) errors[axis] = `${axis} 坐标应为 -100000~100000 mm 的数字`
    }
    if (Object.keys(errors).length) return setCoordinateError(errors)
    setBusy(true)
    try {
      // machineId 必须原样带回：后端按请求整体覆盖，漏传会把已设置的机台清空
      const fixture = fixtureChoice === customFixtureValue ? customFixture.trim() : fixtureChoice
      const input = { opNo: no, opName: opName.trim(), machineId: operation?.machineId ?? null, fixture, centeringMethod: centeringMethod.trim(), xCoordinate: values.X, yCoordinate: values.Y, zHeight: values.Z, remark }
      if (operation) await api.updateOperation(operation.id, input)
      else await api.createOperation(drawingId, input)
      await refresh()
      onClose()
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器'
      if (err instanceof ApiError && err.status === 409) setNoError(message.replace(/^.*?:\s*/, ''))
      else setError(message)
    } finally {
      setBusy(false)
    }
  }

  if (confirmDelete && operation) return <DeleteSheet
    title="删除工序"
    message={`将删除「${operation.opNo}# ${operation.opName}」及其下的 ${operation.programs.length} 个程序和全部版本，无法恢复。`}
    remove={() => api.deleteOperation(operation.id)}
    onDeleted={onClose}
    onClose={() => setConfirmDelete(false)}
  />

  return (
    <>
      <Sheet open title={operation ? '编辑工序' : '新增工序'} onClose={onClose}>
        <form onSubmit={submit} className="space-y-4" noValidate>
          <div className="grid grid-cols-3 gap-3">
            <Field label="工序号" value={opNo} onChange={(e) => setOpNo(e.target.value)} error={noError} inputMode="numeric" className="col-span-1" />
            <div className="col-span-2">
              <Field label="工序名称" value={opName} onChange={(e) => setOpName(e.target.value)} placeholder="如 粗车、精铣" />
            </div>
          </div>
          <label className="block space-y-1.5 text-sm font-medium text-slate-700">
            <span>装夹方式</span>
            <select
              aria-label="装夹方式"
              value={fixtureChoice}
              onChange={(e) => setFixtureChoice(e.target.value)}
              className="h-12 w-full rounded-xl border border-slate-300 bg-white px-4 text-base text-slate-900 focus:outline-2 focus:outline-offset-0 focus:outline-amber-500"
            >
              <option value="">未选择</option>
              {fixtureOptions.map((option) => <option key={option} value={option}>{option}</option>)}
              <option value={customFixtureValue}>自定义</option>
            </select>
          </label>
          {fixtureChoice === customFixtureValue && (
            <Field
              label="自定义装夹方式"
              value={customFixture}
              onChange={(e) => setCustomFixture(e.target.value)}
              placeholder="请输入装夹方式"
              maxLength={200}
            />
          )}
          <Field label="分中方式" value={centeringMethod} onChange={(e) => setCenteringMethod(e.target.value)} placeholder="请输入分中方式（选填）" maxLength={200} disabled={busy} />
          <fieldset className="space-y-2" disabled={busy}>
            <legend className="text-sm font-medium text-slate-700">坐标（mm）</legend>
            <div className="grid grid-cols-3 gap-2">
              {axes.map((axis) => <div key={axis} className="min-w-0 rounded-xl border border-slate-200 bg-slate-50 p-2">
                <Field label={axis} aria-label={`${axis} 坐标（mm）`} value={coordinates[axis]} onChange={(e) => setCoordinates((previous) => ({ ...previous, [axis]: e.target.value }))} error={coordinateError[axis]} placeholder="0" inputMode="text" autoComplete="off" spellCheck={false} className="min-w-0 px-2! tabular-nums" />
              </div>)}
            </div>
            <p className="text-xs text-slate-500">支持正负数，留空按 0 保存，精确到 0.001 mm</p>
          </fieldset>
          <TextArea label="备注" value={remark} onChange={(e) => setRemark(e.target.value)} />
          {error && (
            <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </p>
          )}
          <Button type="submit" loading={busy} className="w-full">
            保存
          </Button>
          {operation && (
            <Button variant="ghost" className="w-full text-red-600" disabled={busy} onClick={() => setConfirmDelete(true)}>
              删除此工序
            </Button>
          )}
        </form>
      </Sheet>

    </>
  )
}
