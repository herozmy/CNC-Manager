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
  const [zHeight, setZHeight] = useState(operation && operation.zHeight > 0 ? String(operation.zHeight) : '')
  const [remark, setRemark] = useState(operation?.remark ?? '')
  const [noError, setNoError] = useState('')
  const [zError, setZError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: ['drawing', drawingId] }), qc.invalidateQueries({ queryKey: ['drawings'] })])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setNoError('')
    setZError('')
    setError('')
    const no = Number(opNo)
    if (!Number.isInteger(no) || no < 1 || no > 9999) return setNoError('工序号应为 1~9999 的整数')
    const z = zHeight.trim() === '' ? 0 : Number(zHeight)
    if (!Number.isFinite(z) || z < 0) return setZError('垫高应为不小于 0 的数字')
    setBusy(true)
    try {
      // machineId 必须原样带回：后端按请求整体覆盖，漏传会把已设置的机台清空
      const fixture = fixtureChoice === customFixtureValue ? customFixture.trim() : fixtureChoice
      const input = { opNo: no, opName: opName.trim(), machineId: operation?.machineId ?? null, fixture, zHeight: z, remark }
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
          <Field label="Z 轴垫高（mm）" value={zHeight} onChange={(e) => setZHeight(e.target.value)} error={zError} inputMode="decimal" hint="工件垫块高度，影响 Z 向对刀" />
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
