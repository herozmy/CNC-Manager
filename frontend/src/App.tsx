import { useQuery } from '@tanstack/react-query'
import { fetchMeta } from './api/meta'

export default function App() {
  const { data, isError, isPending } = useQuery({ queryKey: ['meta'], queryFn: fetchMeta })
  return (
    <main className="mx-auto max-w-xl p-4">
      <h1 className="text-xl font-semibold">CNC-Manager</h1>
      <p className="mt-2 text-sm text-slate-600">
        {isPending ? '加载中…' : isError ? '无法连接服务器' : `版本 ${data.version}`}
      </p>
    </main>
  )
}
