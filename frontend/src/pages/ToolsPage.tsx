import { Calculator, ChevronRight, ClipboardList, Code2 } from 'lucide-react'
import { Link } from 'react-router-dom'

export default function ToolsPage() {
  return <main className="page-width min-w-0 space-y-6 px-4 pt-4 pb-8 md:px-6">
    <header>
      <h1 className="text-xl font-semibold text-ink">加工助手</h1>
      <p className="mt-1 text-sm text-muted">选择工具，查询和计算加工参数</p>
    </header>
    <div className="grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Link to="/tools/tapping" className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Calculator className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold text-ink">公制攻丝</span><span className="mt-2 block text-sm leading-6 text-muted">查询底孔与螺距，输入转速计算进给</span></span>
        <ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
      </Link>
      <Link to="/tools/gcode" className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><Code2 className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold text-ink">G 代码编程</span><span className="mt-2 block text-sm leading-6 text-muted">极坐标打孔程序生成与孔位核对</span></span><ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
      </Link>
      <Link to="/tools/preparation" className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><ClipboardList className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold text-ink">加工准备单</span><span className="mt-2 block text-sm leading-6 text-muted">填写装夹要求和刀具清单，打印车间用单</span></span>
        <ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
      </Link>
    </div>
  </main>
}
