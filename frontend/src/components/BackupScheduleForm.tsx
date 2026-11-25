import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import type { ScheduleSettings } from '../api/types'
import { Button } from './ui/Button'

const key = ['backup-schedule']
const field = 'block mt-1 rounded-md border border-line-strong bg-surface px-3 py-2'
const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

export function BackupScheduleForm() {
  const [expanded, setExpanded] = useState(false)
  const [draft, setDraft] = useState<ScheduleSettings | null>(null)
  const cache = useQueryClient()
  const schedule = useQuery({ queryKey: key, queryFn: api.backupSchedule, enabled: expanded, refetchInterval: expanded ? 30000 : false })
  const save = useMutation({ mutationFn: api.saveBackupSchedule, onSuccess: saved => {
    cache.setQueryData(key, saved)
    setDraft(null)
  } })
  const values = draft ?? schedule.data
  function change(fields: Partial<ScheduleSettings>) {
    if (values) setDraft({ frequency: values.frequency, time: values.time || schedule.data?.time || '02:00', weekday: values.weekday, retention: values.retention, ...fields })
    save.reset()
  }
  return <details className="border-t border-line pt-3" onToggle={e => setExpanded(e.currentTarget.open)}>
    <summary className="min-h-10 cursor-pointer text-sm font-semibold">整站备份计划</summary>
    {expanded && <div className="space-y-3 text-sm">
      {schedule.isLoading && <p>正在读取计划…</p>}
      {schedule.isError && <button onClick={() => schedule.refetch()}>读取计划失败，点击重试</button>}
      {values && <form className="space-y-3" onSubmit={e => { e.preventDefault(); if (draft) save.mutate(draft) }}>
        <fieldset disabled={save.isPending} className="flex flex-wrap gap-3">
          <label>备份频率<select className={field} value={values.frequency} onChange={e => change({ frequency: e.target.value as ScheduleSettings['frequency'] })}>
            <option value="off">关闭</option><option value="daily">每天</option><option value="weekly">每周</option>
          </select></label>
          {values.frequency === 'weekly' && <label>星期<select className={field} value={values.weekday} onChange={e => change({ weekday: Number(e.target.value) })}>
            {weekdays.map((day, index) => <option key={day} value={index}>{day}</option>)}
          </select></label>}
          {values.frequency !== 'off' && <label>执行时间（北京时间）<input required className={field} type="time" value={values.time} onChange={e => change({ time: e.target.value })} /></label>}
          <label>计划备份保留份数<input className={field} type="number" min={1} max={365} step={1} required value={values.retention || ''} onChange={e => change({ retention: Number(e.target.value) })} /></label>
        </fieldset>
        <p className="text-muted">下次计划备份成功后，自动删除超过保留份数的最早计划备份。手动备份和导入前恢复副本始终保留，直到手动删除。</p>
        {schedule.data?.next && <p className="text-muted">下次备份（北京时间） {new Date(schedule.data.next).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false })}</p>}
        {schedule.data?.lastResult && <p>{schedule.data.lastResult}</p>}
        <Button type="submit" variant="secondary" disabled={!draft || save.isPending}>保存计划</Button>
        {save.isSuccess && <p role="status">备份计划已保存。</p>}
        {save.isError && <p role="alert" className="text-danger">{save.error.message}</p>}
      </form>}
    </div>}
  </details>
}
