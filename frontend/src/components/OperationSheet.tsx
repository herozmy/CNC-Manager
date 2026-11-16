import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { MachineOption, Operation } from '../api/types'
import { Button } from './ui/Button'
import { Field, TextArea } from './ui/Field'
import DeleteSheet from './DeleteSheet'
import { Sheet } from './ui/Sheet'

interface Props {
  drawingId: number
  ownerUserId?: number
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
export default function OperationSheet({ drawingId, ownerUserId, operation, suggestedNo = 10, onClose }: Props) {
  const qc = useQueryClient()
  const machines = useQuery({ queryKey: ['drawing-machines', drawingId], queryFn: () => api.drawingMachines(drawingId), retry: false })
  const options = (machines.data ?? []).filter((m) => m.enabled !== false || m.id === operation?.machineId)
  const [machineId, setMachineId] = useState<number | null>(operation?.machineId ?? null)
  const [addingMachine, setAddingMachine] = useState(false)
  const [machineCode, setMachineCode] = useState('')
  const [machineName, setMachineName] = useState('')
  const [machineError, setMachineError] = useState('')
  const pending = useRef(false)
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

  async function addMachine() {
    if (pending.current) return
    setMachineError('')
    if (!machineCode.trim() || [...machineCode.trim()].length > 64) return setMachineError('机台编号应为 1~64 个字符')
    if ([...machineName.trim()].length > 100) return setMachineError('机台名称最多 100 个字符')
    pending.current = true
    setBusy(true)
    try {
      const machine = await api.createDrawingMachine(drawingId, { code: machineCode.trim(), name: machineName.trim() })
      qc.setQueryData<MachineOption[]>(['drawing-machines', drawingId], (old) => [...(old ?? []), machine])
      void qc.invalidateQueries({ queryKey: ['drawing-machines'] })
      void qc.invalidateQueries({ queryKey: ['machines'] })
      setMachineId(machine.id)
      setAddingMachine(false)
      setMachineCode(''); setMachineName('')
    } catch (err) {
      setMachineError(err instanceof ApiError ? err.message : '机台创建失败，请检查网络后重试')
    } finally { pending.current = false; setBusy(false) }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setNoError('')
    if (pending.current) return
    if (addingMachine) { await addMachine(); return }
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
    pending.current = true
    setBusy(true)
    try {
      const fixture = fixtureChoice === customFixtureValue ? customFixture.trim() : fixtureChoice
      const input = { opNo: no, opName: opName.trim(), machineId, fixture, centeringMethod: centeringMethod.trim(), xCoordinate: values.X, yCoordinate: values.Y, zHeight: values.Z, remark }
      if (operation) await api.updateOperation(operation.id, input)
      else await api.createOperation(drawingId, input)
      await refresh()
      onClose()
    } catch (err) {
      const message = err instanceof ApiError ? err.message : '无法连接服务器'
      if (err instanceof ApiError && err.status === 409) setNoError(message.replace(/^.*?:\s*/, ''))
      else setError(message)
    } finally {
      pending.current = false
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
      <Sheet open title={operation ? '编辑工序' : '新增工序'} onClose={() => { if (!pending.current) onClose() }}>
        <form onSubmit={submit} className="space-y-4" noValidate>
          <div className="grid grid-cols-3 gap-3">
            <Field label="工序号" value={opNo} onChange={(e) => setOpNo(e.target.value)} error={noError} inputMode="numeric" className="col-span-1" />
            <div className="col-span-2">
              <Field label="工序名称" value={opName} onChange={(e) => setOpName(e.target.value)} placeholder="如 粗车、精铣" />
            </div>
          </div>
          <section className="space-y-2 rounded-md border border-line p-3" aria-label="工序机台">
            <label className="block space-y-1.5 text-sm font-medium text-body">
              <span>机台（可选）</span>
              <select aria-label="机台（可选）" value={machineId ?? ''} onChange={(e) => setMachineId(e.target.value ? Number(e.target.value) : null)} disabled={busy || machines.isPending || addingMachine} className="h-12 w-full min-w-0 rounded-md border border-line-strong bg-surface px-3 text-base">
                <option value="">未关联机台</option>
                {machineId !== null && !options.some((m) => m.id === machineId) && <option value={machineId}>{[operation?.machineCode, operation?.machineName].filter(Boolean).join(' · ') || ('机台 #' + machineId)}（当前关联）</option>}
                {options.map((m) => <option key={m.id} value={m.id}>{m.code}{m.name ? ' · ' + m.name : ''}{m.enabled === false ? '（已停用，当前关联）' : ''}</option>)}
              </select>
            </label>
            {machines.isPending && <p role="status" className="text-sm text-muted">正在加载机台…</p>}
            {machines.isError && <p className="text-sm text-danger">机台列表加载失败，当前关联仍保留。<button type="button" className="ml-2 min-h-12 underline" disabled={busy} onClick={() => void machines.refetch()}>重试加载机台</button></p>}
            {machines.isSuccess && options.length === 0 && <p className="text-sm text-muted">暂无可选机台，可先不关联。</p>}
            <p className="text-xs text-muted">仅列出图纸所属账号的启用机台；已停用的当前关联可保留。选择“未关联机台”可解除关联。</p>
            {ownerUserId === 0 ? <p className="text-sm text-muted">请先分配图纸所属账号，再新增机台。</p> : !addingMachine && <Button variant="secondary" disabled={busy} onClick={() => { setMachineError(''); setAddingMachine(true) }}>新增机台</Button>}
            {addingMachine && <fieldset disabled={busy} className="space-y-3">
              <Field label="机台编号" value={machineCode} onChange={(e) => setMachineCode(e.target.value)} maxLength={64} placeholder="如 CNC-01" />
              <Field label="机台名称（可选）" value={machineName} onChange={(e) => setMachineName(e.target.value)} maxLength={100} placeholder="如 立式加工中心" />
              <p className="text-xs text-muted">新增后可供同一账号的其他工序复用；工序关联需点击下方“保存”。</p>
              {machineError && <p role="alert" className="text-sm text-danger">{machineError}</p>}
              <div className="flex flex-wrap gap-2"><Button onClick={() => void addMachine()} loading={busy}>新增并选中</Button><Button variant="ghost" onClick={() => setAddingMachine(false)}>取消新增机台</Button></div>
            </fieldset>}
          </section>
          <label className="block space-y-1.5 text-sm font-medium text-body">
            <span>装夹方式</span>
            <select
              aria-label="装夹方式"
              value={fixtureChoice}
              onChange={(e) => setFixtureChoice(e.target.value)}
              className="h-12 w-full rounded-md border border-line-strong bg-surface px-4 text-base text-ink focus:outline-2 focus:outline-offset-0 focus:outline-accent"
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
            <legend className="text-sm font-medium text-body">坐标（mm）</legend>
            <div className="grid grid-cols-3 gap-2">
              {axes.map((axis) => <div key={axis} className="min-w-0 rounded-md border border-line bg-canvas p-2">
                <Field label={axis} aria-label={`${axis} 坐标（mm）`} value={coordinates[axis]} onChange={(e) => setCoordinates((previous) => ({ ...previous, [axis]: e.target.value }))} error={coordinateError[axis]} placeholder="0" inputMode="text" autoComplete="off" spellCheck={false} className="min-w-0 px-2! tabular-nums" />
              </div>)}
            </div>
            <p className="text-xs text-muted">支持正负数，留空按 0 保存，精确到 0.001 mm</p>
          </fieldset>
          <TextArea label="备注" value={remark} onChange={(e) => setRemark(e.target.value)} />
          {error && (
            <p role="alert" className="rounded-lg bg-danger-soft px-3 py-2 text-sm text-danger">
              {error}
            </p>
          )}
          <Button type="submit" loading={busy} disabled={addingMachine} className="w-full">
            保存
          </Button>
          {operation && (
            <Button variant="ghost" className="w-full text-danger" disabled={busy} onClick={() => setConfirmDelete(true)}>
              删除此工序
            </Button>
          )}
        </form>
      </Sheet>

    </>
  )
}
