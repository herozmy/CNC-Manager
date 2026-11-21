import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ApiError, http } from '../api/client'
import { Button } from './ui/Button'
import { ConfirmSheet } from './ui/Sheet'

interface State {
  phase: 'disabled' | 'idle' | 'checking' | 'downloading' | 'installing' | 'done' | 'failed'
  target?: string
  message?: string
}

const key = ['admin-update-status']
const path = '/admin/update'
const running = (s?: State) => !!s && ['checking', 'downloading', 'installing'].includes(s.phase)

export default function InstallUpdate({ target }: {target?: string}) {
  const [confirm, setConfirm] = useState(false)
  const [requested, setRequested] = useState(() => sessionStorage.getItem(path))
  const client = useQueryClient()
  const status = useQuery({
    enabled: !!target || !!requested, queryKey: key, retry: false,
    queryFn: async () => {
      try {
        return await http.get<State>(path)
      } catch (error) {
        // An older standalone Release can serve /meta without the launcher status endpoint.
        if (requested && error instanceof ApiError && error.status === 404) {
          const meta = await http.get<{version: string}>('/meta')
          if ('v' + meta.version === requested) return { phase: 'done', target: requested } satisfies State
        }
        throw error
      }
    },
    refetchInterval: q => (running(q.state.data) || (!!requested && q.state.status === 'error')) ? 2000 : false,
  })
  const install = useMutation({
    mutationFn: (version: string) => http.post<State>(path, {target: version}),
    onMutate: version => {
      sessionStorage.setItem(path, version)
      setRequested(version)
    },
    onSuccess: state => {
      client.setQueryData(key, state)
      setConfirm(false)
    },
    onError: () => { setConfirm(false); void status.refetch() },
  })
  const state = status.data
  const busy = running(state)
  const completed = state?.phase === 'done' && !!requested && requested === state.target
  if (!target && !busy && !completed && state?.phase !== 'failed' && !install.error) return null
  return <section aria-label="安装更新" className="space-y-2">
    {busy && <p role="status">{status.isError ? '正在等待服务重新连接…' : state?.message}</p>}
    {completed && <Button onClick={() => { sessionStorage.removeItem(path); window.location.reload() }}>升级已完成，刷新页面</Button>}
    {state?.phase === 'failed' && <p role="alert" className="text-danger">{state.message}</p>}
    {install.error && <p role="alert" className="text-danger">{install.error.message}</p>}
    {target && !busy && !completed && (
      state?.phase === 'disabled' ? <p className="text-sm text-muted">当前运行方式不支持应用内安装。请使用包含启动器的 Linux amd64 镜像。</p>
      : status.isError ? <Button variant="secondary" onClick={() => void status.refetch()}>重试连接更新服务</Button>
      : <Button disabled={!state || install.isPending} onClick={() => setConfirm(true)}>下载并安装 {target}</Button>
    )}
    <ConfirmSheet open={confirm} title="安装系统更新" message={`将下载 ${target} 的程序文件，备份数据并重启 CNC 服务。请先保存工作，重启期间暂时无法访问。`}
      confirmLabel="确认安装并重启" busy={install.isPending} onClose={() => { if (!install.isPending) setConfirm(false) }}
      onConfirm={() => { if (target && !install.isPending) install.mutate(target) }} />
  </section>
}
