import { useQuery } from '@tanstack/react-query'
import { api } from '../api'

export default function OwnerSelect({ value, onChange, includeUnassigned = false, includeDisabled = false, label = '所属账号', required = false }: { value: number | ''; onChange: (value: number | '') => void; includeUnassigned?: boolean; includeDisabled?: boolean; label?: string; required?: boolean }) {
  const users = useQuery({ queryKey: ['admin-users'], queryFn: api.users, retry: false })
  return <label className="block space-y-1.5 text-sm font-medium text-slate-700">{label}
    <select required={required} aria-label={label} className="h-12 w-full rounded-xl border border-slate-300 bg-white px-3 text-base" value={value} onChange={(e) => onChange(e.target.value ? Number(e.target.value) : '')}>
      <option value="">{required ? '请选择所属账号' : '全部账号'}</option>
      {includeUnassigned && <option value="0">待分配（仅管理员可见）</option>}
      {users.data?.filter((user) => includeDisabled || user.enabled).map((user) => <option key={user.id} value={user.id}>{user.displayName}（{user.username}）{user.enabled ? '' : '（已停用）'}</option>)}
    </select>
    {users.isError && <span role="alert" className="block text-red-700">账号加载失败，请刷新重试。</span>}
  </label>
}
