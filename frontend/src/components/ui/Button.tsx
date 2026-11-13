import clsx from 'clsx'
import { Loader2 } from 'lucide-react'
import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  loading?: boolean
}

const styles: Record<Variant, string> = {
  primary: 'bg-amber-500 text-slate-900 hover:bg-amber-400 active:bg-amber-600',
  secondary: 'bg-white text-slate-900 border border-slate-300 hover:bg-slate-100 active:bg-slate-200',
  ghost: 'text-slate-700 hover:bg-slate-200 active:bg-slate-300',
  danger: 'bg-red-600 text-white hover:bg-red-500 active:bg-red-700',
}

/** 触控友好：最小高度 48px。 */
export function Button({ variant = 'primary', loading, disabled, className, children, type = 'button', ...rest }: Props) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      className={clsx(
        'inline-flex min-h-12 items-center justify-center gap-2 rounded-xl px-5 text-base font-semibold',
        'transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-600',
        styles[variant],
        className,
      )}
      {...rest}
    >
      {loading && <Loader2 className="size-5 animate-spin" aria-hidden />}
      {children}
    </button>
  )
}
