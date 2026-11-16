import clsx from 'clsx'
import { Loader2 } from 'lucide-react'
import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  loading?: boolean
}

const styles: Record<Variant, string> = {
  primary: 'bg-blue-600 text-white hover:bg-blue-500 active:bg-blue-700',
  secondary: 'bg-surface text-ink border border-line-strong hover:bg-inset active:bg-hover',
  ghost: 'text-body hover:bg-hover active:bg-hover',
  danger: 'bg-red-600 text-white hover:bg-red-500 active:bg-red-700',
}

/** 触控友好：最小高度 48px。 */
export function Button({ variant = 'primary', loading, disabled, className, children, type = 'button', ...rest }: Props) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      className={clsx(
        'inline-flex min-h-12 items-center justify-center gap-2 rounded-md px-4 text-sm font-semibold',
        'transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
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
