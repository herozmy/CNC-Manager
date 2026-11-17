import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import type { AuditFilter } from '../api/types'
import { useAccess } from '../auth/permissions'
import { Button } from '../components/ui/Button'
import { Field } from '../components/ui/Field'
import { ErrorState, Spinner } from '../components/ui/State'

const selectClass = 'h-12 w-full rounded-md border border-line-strong bg-surface px-3'
export default function AuditPage() {
  const { admin } = useAccess()
  const [filter, setFilter] = useState<AuditFilter>({ page: 1, size: 20 })
  const [draft, setDraft] = useState({ userId: '', action: '', entityType: '', result: '', from: '', to: '' })
  const [error, setError] = useState('')
  const users = useQuery({ queryKey: ['admin-users'], queryFn: api.users, enabled: admin })
  const logs = useQuery({ queryKey: ['audit-logs', filter], queryFn: () => api.auditLogs(filter), enabled: admin })
  if (!admin) return <main className="p-6">仅系统管理员可查看操作日志。</main>
  return <main className="mx-auto max-w-4xl space-y-4 px-4 py-5"><Link to="/me/account" className="text-sm text-muted">返回账号管理</Link><h1 className="text-2xl font-bold">操作日志</h1><p className="text-sm text-muted">保留操作发生时的身份和归属。旧日志缺失信息显示为未知；恢复后展示备份中的历史和恢复后的新事件。</p>
    <form className="grid grid-cols-1 gap-3 rounded-lg border border-line bg-surface p-4 sm:grid-cols-2" onSubmit={(e) => { e.preventDefault(); if (draft.from && draft.to && draft.from > draft.to) { setError('起始时间不能晚于结束时间'); return } setError(''); setFilter({ page: 1, size: 20, userId: draft.userId ? Number(draft.userId) : undefined, action: draft.action, entityType: draft.entityType, result: draft.result, from: draft.from ? new Date(draft.from).toISOString() : undefined, to: draft.to ? new Date(draft.to).toISOString() : undefined }) }}>
      <label>操作者<select aria-label="操作者" className={selectClass} value={draft.userId} onChange={(e) => setDraft({ ...draft, userId: e.target.value })}><option value="">全部账号</option>{users.data?.map((u) => <option key={u.id} value={u.id}>{u.displayName} ({u.username})</option>)}</select></label>
      <Field label="动作" placeholder="如 create、update、download" value={draft.action} onChange={(e) => setDraft({ ...draft, action: e.target.value })} /><Field label="对象类型" placeholder="如 drawing、program、app_user" value={draft.entityType} onChange={(e) => setDraft({ ...draft, entityType: e.target.value })} />
      <Field label="起始时间" type="datetime-local" value={draft.from} onChange={(e) => setDraft({ ...draft, from: e.target.value })} /><Field label="结束时间" type="datetime-local" value={draft.to} onChange={(e) => setDraft({ ...draft, to: e.target.value })} />
      <label>结果<select aria-label="结果" className={selectClass} value={draft.result} onChange={(e) => setDraft({ ...draft, result: e.target.value })}><option value="">全部结果</option><option value="success">成功</option><option value="failed">失败</option><option value="started">已开始</option></select></label><Button type="submit">筛选日志</Button>
      {error && <p role="alert" className="text-danger">{error}</p>}
    </form>
    {logs.isPending && <Spinner />}{logs.isError && <ErrorState message="日志加载失败" onRetry={() => logs.refetch()} />}
    {logs.data && <><p className="text-sm text-muted">共 {logs.data.total} 条记录 · 第 {filter.page} 页</p>{logs.data.items.length === 0 && <p className="py-8 text-center">没有匹配的记录</p>}<ol className="space-y-3">{logs.data.items.map((e) => <li key={e.id} className="space-y-2 rounded-lg border border-line bg-surface p-4"><div className="flex flex-wrap justify-between gap-2"><strong className="break-all">{e.action} · {e.entityName || e.entityType}{e.entityId ? ` #${e.entityId}` : ''}</strong><span className={e.result === 'failed' ? 'text-danger' : 'text-muted'}>{e.result === 'success' ? '成功' : e.result === 'failed' ? '失败' : '已开始'}</span></div><p className="break-words text-sm">{e.detail}</p><p className="break-words text-sm text-muted">操作者：{e.actor || '未知'} · 当时身份：{e.actorRoleName || '未知'} · 数据归属：{e.ownerName || (e.roleName ? `旧角色：${e.roleName}` : '未知')}</p><time className="text-xs text-muted">{new Date(e.createdAt).toLocaleString()}</time></li>)}</ol><div className="flex justify-between"><Button variant="secondary" disabled={filter.page <= 1} onClick={() => setFilter({ ...filter, page: filter.page - 1 })}>上一页</Button><Button variant="secondary" disabled={filter.page * filter.size >= logs.data.total} onClick={() => setFilter({ ...filter, page: filter.page + 1 })}>下一页</Button></div></>}
  </main>
}
