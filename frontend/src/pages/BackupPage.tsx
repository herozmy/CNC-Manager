import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAccess } from '../auth/permissions'
import BackupSection from '../components/BackupSection'

export default function BackupPage() {
  const access = useAccess()
  return <main className="mx-auto max-w-xl space-y-4 px-4 pt-4 pb-8">
    <Link to="/me" className="inline-flex min-h-12 items-center gap-2 rounded-xl px-2 text-sm text-slate-600 hover:bg-slate-200"><ArrowLeft className="size-5" aria-hidden />返回我的</Link>
    <h1 className="text-2xl font-bold">备份与迁移</h1>
    {access.admin ? <BackupSection /> : <p className="text-sm text-slate-500">仅系统管理员可访问备份与迁移。</p>}
  </main>
}
