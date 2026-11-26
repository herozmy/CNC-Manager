import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Printer, Plus } from 'lucide-react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import type { DrawingDetail, Operation, Program, ProgramTool } from '../api/types'
import { useAccess } from '../auth/permissions'
import { ErrorState, Spinner } from '../components/ui/State'
import type { Crumb } from '../components/ui/Breadcrumbs'
import { createPortal } from 'react-dom'
import { Breadcrumbs } from '../components/ui/Breadcrumbs'
import { Button } from '../components/ui/Button'
import { Field, TextArea } from '../components/ui/Field'
import './preparation.css'

const details = ['零件名称', '图号', '程序号', '工序', '机床', '毛坯材料及尺寸']
const instructions = ['装夹与定位', '工件坐标系及对刀基准', '加工注意事项', '补充事项']
const columns = ['刀号', '刀具名称及规格', '刀柄及伸出量', '备注']
type Values = Record<string, string>
type Tool = { id: number; values: Values }
type Draft = { fields: Values; tools: Tool[] }
const blankCrumb = { label: '加工助手', to: '/tools' }
const empty: Draft = { fields: {}, tools: [{ id: 1, values: {} }] }

function fromDrawing(drawing: DrawingDetail, op?: Operation, program?: Program, rows?: ProgramTool[]): Draft {
  const machine = (record?: Program | Operation) => [record?.machineCode, record?.machineName].filter(Boolean).join(' · ')
  const coordinates = [['X', op?.xCoordinate], ['Y', op?.yCoordinate], ['Z', op?.zHeight]]
    .filter(([, value]) => value != null).map(([axis, value]) => `${axis} ${value} mm`).join('，')
  return {
    fields: {
      '图号': drawing.drawingNo, '产品类型': drawing.name, '图纸版本': drawing.drawingVersion,
      '程序号': program?.programNo ?? '', '程序版本': program?.currentVersionNo ? `V${program.currentVersionNo}` : '',
      '工序': op ? `${op.opNo}# ${op.opName}` : '', '机床': machine(program) || machine(op),
      '毛坯材料及尺寸': drawing.material, '装夹与定位': op?.fixture ?? '',
      '工件坐标系及对刀基准': [op?.centeringMethod, coordinates].filter(Boolean).join('\n'),
      '加工注意事项': [drawing.remark, op?.remark, program?.remark].filter(Boolean).join('\n'),
    },
    tools: rows?.length ? rows.map((row, i) => ({ id: i + 1, values: {
      '刀号': row.toolNo, '刀具名称及规格': [row.toolName, row.toolDia ? `Ø${row.toolDia} mm` : ''].filter(Boolean).join(' · '),
      '刀补（mm）': String(row.compAmount ?? ''), '备注': [row.machiningContent, row.remark].filter(Boolean).join(' · '),
    } })) : empty.tools,
  }
}

export function DrawingPreparationPage() {
  const id = Number(useParams().id)
  const access = useAccess()
  const [params, setParams] = useSearchParams()
  const valid = Number.isSafeInteger(id) && id > 0 && access.can('read')
  const drawing = useQuery({ queryKey: ['drawing', id], queryFn: () => api.drawingDetail(id), enabled: valid, retry: false })
  const ops = drawing.data?.operations ?? []
  const op = ops.find(item => String(item.id) === params.get('op')) ?? ops[0]
  const program = op?.programs.find(item => String(item.id) === params.get('program')) ?? op?.programs[0]
  const versionId = program?.currentVersionId
  const table = useQuery({ queryKey: ['version-tools', versionId], queryFn: () => api.versionTools(versionId!), enabled: valid && !!versionId, retry: false })
  if (!valid) return <p className="p-6">无法访问此图纸。</p>
  if (drawing.isError) return <ErrorState message="图纸加载失败" onRetry={() => drawing.refetch()} />
  if (!drawing.data) return <Spinner />
  const selectStyle = 'mt-1 block min-h-12 w-full rounded-md border border-line-strong bg-surface px-3 text-ink'
  const scope = [id, op?.id ?? 0, program?.id ?? 0, versionId ?? 0]
  const source = fromDrawing(drawing.data, op, program, table.data?.items)
  return <PreparationPage key={scope.join('/')} scope={scope} source={source}
    crumb={{ label: drawing.data.drawingNo, to: `/drawings/${id}` }}
    loading={!!versionId && table.isPending} error={table.isError ? <ErrorState message="刀具表加载失败" onRetry={() => table.refetch()} /> : undefined}>
    {op && <section aria-label="准备单来源" className="panel grid gap-4 p-4 sm:grid-cols-2">
      <label className="text-sm font-medium">来源工序<select className={selectStyle} value={op.id} onChange={e => setParams({ op: e.target.value }, { replace: true })}>
        {ops.map(item => <option key={item.id} value={item.id}>{item.opNo}# {item.opName}</option>)}
      </select></label>
      {program && <label className="text-sm font-medium">来源程序<select className={selectStyle} value={program.id} onChange={e => setParams({ op: String(op.id), program: e.target.value }, { replace: true })}>
        {op.programs.map(item => <option key={item.id} value={item.id}>{[item.programNo, item.programName].filter(Boolean).join(' · ')}</option>)}
      </select></label>}
    </section>}
  </PreparationPage>
}

type PreparationProps = { scope?: number[]; source?: Draft; crumb?: Crumb; children?: React.ReactNode; loading?: boolean; error?: React.ReactNode }

export default function PreparationPage({ scope = [], source = empty, crumb = blankCrumb, children, loading, error }: PreparationProps) {
  const key = ['preparation-draft', ...scope]
  const sheetDetails = scope.length ? [...details, '产品类型', '图纸版本', '程序版本'] : details
  const sheetColumns = scope.length ? [...columns.slice(0, 2), '刀补（mm）', ...columns.slice(2)] : columns
  const client = useQueryClient()
  const { data: draft } = useQuery<Partial<Draft>>({ queryKey: key, queryFn: () => ({}), initialData: {}, staleTime: Infinity, gcTime: Infinity })
  const fields = { ...source.fields, ...draft.fields }
  const tools = draft.tools ?? source.tools
  function update(change: Partial<Draft>) {
    client.setQueryData(key, { ...draft, ...change })
  }
  function fieldProps(label: string) {
    return { label, value: fields[label] ?? '', onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => update({ fields: { ...draft.fields, [label]: e.target.value } }) }
  }
  return <main className="page-width min-w-0 space-y-5 px-4 pt-4 pb-8 md:px-6">
    <Breadcrumbs items={[crumb, { label: '加工准备单' }]} />
    <header className="flex flex-wrap items-center justify-between gap-3">
      <div><h1 className="text-xl font-semibold">加工准备单</h1>
        <p className="mt-1 text-sm text-muted">草稿在当前登录页面内保留，刷新前请打印或另存为 PDF。</p></div>
      <Button disabled={loading || !!error} onClick={() => window.print()}><Printer size={18} aria-hidden />打印准备单</Button>
    </header>
    {children}
    {loading ? <Spinner /> : error ?? <>
    <section className="panel space-y-4 p-4 md:p-5"><h2 className="font-semibold">工件与工序</h2>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{sheetDetails.map(label => <Field key={label} {...fieldProps(label)} />)}</div>
    </section>
    <section className="panel space-y-4 p-4 md:p-5"><h2 className="font-semibold">装夹与加工要求</h2>
      {instructions.map(label => <TextArea key={label} rows={2} {...fieldProps(label)} />)}
    </section>
    <section className="panel space-y-4 p-4 md:p-5"><h2 className="font-semibold">刀具清单</h2>
      {tools.map((tool, i) => <fieldset key={tool.id} className="min-w-0 border-t border-line">
        <legend className="pr-2 text-sm text-muted">第 {i + 1} 把刀具</legend>
        <div className="grid items-end gap-3 pt-2 sm:grid-cols-2 xl:grid-flow-col xl:auto-cols-fr">
          {sheetColumns.map(label => <Field key={label} label={label} value={tool.values[label] ?? ''} onChange={e => update({ tools: tools.map(row => row.id === tool.id ? { ...row, values: { ...row.values, [label]: e.target.value } } : row) })} />)}
          <Button variant="ghost" aria-label={`移除第 ${i + 1} 把刀具`} onClick={() => update({ tools: tools.filter(row => row.id !== tool.id) })}>移除</Button>
        </div>
      </fieldset>)}
      <Button variant="secondary" onClick={() => update({ tools: [...tools, { id: (tools.at(-1)?.id ?? 0) + 1, values: {} }] })}><Plus size={18} aria-hidden />添加刀具</Button>
    </section>
    {createPortal(<article className="preparation-print">
      <h1>加工准备单</h1>
      <h2>工件与工序</h2>
      <dl>{sheetDetails.map(label => <div key={label}><dt>{label}</dt><dd>{fields[label]}</dd></div>)}</dl>
      <h2>装夹与加工要求</h2>
      <dl className="instructions">{instructions.map(label => <div key={label}><dt>{label}</dt><dd>{fields[label]}</dd></div>)}</dl>
      <h2>刀具清单</h2>
      <table><thead><tr>{sheetColumns.map(label => <th key={label}>{label}</th>)}</tr></thead>
        <tbody>{tools.map(tool => <tr key={tool.id}>{sheetColumns.map(label => <td key={label}>{tool.values[label]}</td>)}</tr>)}</tbody></table>
    </article>, document.body)}
    </>}
  </main>
}
