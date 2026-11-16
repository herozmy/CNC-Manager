import clsx from 'clsx'
import { useId } from 'react'
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react'

interface BaseProps {
  label: string
  error?: string
  hint?: string
}

const control =
  'w-full rounded-md border bg-surface px-4 text-base text-ink placeholder:text-subtle ' +
  'focus:outline-2 focus:outline-offset-0 focus:outline-accent'

function Wrapper({ id, label, error, hint, children }: BaseProps & { id: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="block text-sm font-medium text-body">
        {label}
      </label>
      {children}
      {error ? (
        <p id={`${id}-err`} role="alert" className="text-sm text-danger">
          {error}
        </p>
      ) : (
        hint && <p className="text-sm text-muted">{hint}</p>
      )}
    </div>
  )
}

export function Field({ label, error, hint, className, ...rest }: BaseProps & InputHTMLAttributes<HTMLInputElement>) {
  const id = useId()
  return (
    <Wrapper id={id} label={label} error={error} hint={hint}>
      <input
        id={id}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-err` : undefined}
        className={clsx(control, 'h-12', error ? 'border-red-500' : 'border-line-strong', className)}
        {...rest}
      />
    </Wrapper>
  )
}

export function TextArea({ label, error, hint, className, ...rest }: BaseProps & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const id = useId()
  return (
    <Wrapper id={id} label={label} error={error} hint={hint}>
      <textarea
        id={id}
        rows={3}
        aria-invalid={!!error}
        className={clsx(control, 'py-3', error ? 'border-red-500' : 'border-line-strong', className)}
        {...rest}
      />
    </Wrapper>
  )
}
