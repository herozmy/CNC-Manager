import { ArrowLeft, ChevronRight, CircleDot } from 'lucide-react'
import { Link } from 'react-router-dom'

export default function GCodePage() {
  return <main className="page-width min-w-0 space-y-6 px-4 pt-4 pb-8 md:px-6">
    <header className="flex items-center gap-3"><Link to="/tools" aria-label="返回加工助手" title="返回加工助手" className="flex size-12 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"><ArrowLeft className="size-5" aria-hidden /></Link><div><h1 className="text-xl font-semibold">G 代码编程</h1><p className="mt-1 text-sm text-muted">选择编程工具，生成并核对 NC 程序</p></div></header>
    <div className="grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Link to="/tools/gcode/drilling" className="panel group flex min-w-0 items-start gap-4 p-4 transition-colors hover:border-accent active:bg-inset focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent sm:min-h-36 md:p-5">
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent"><CircleDot className="size-6" aria-hidden /></span>
        <span className="min-w-0 flex-1"><span className="block text-base font-semibold text-ink">钻孔编程</span><span className="mt-2 block text-sm leading-6 text-muted">选择极坐标、直线等距或矩形阵列钻孔</span></span><ChevronRight className="size-5 shrink-0 self-center text-subtle group-hover:text-accent" aria-hidden />
      </Link>
    </div>
  </main>
}
