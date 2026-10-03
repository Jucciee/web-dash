import type { ButtonHTMLAttributes, Ref } from 'react'
import { LoaderCircle } from 'lucide-react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'ghost' | 'danger'; loading?: boolean; ref?: Ref<HTMLButtonElement> }

export function Button({ variant = 'secondary', loading = false, className = '', children, disabled, ...props }: Props) {
  return <button type="button" {...props} className={`button button-${variant} ${className}`} disabled={disabled || loading} aria-busy={loading || undefined}>
    {loading && <LoaderCircle className="spinner" size={16} aria-hidden="true" />}{children}
  </button>
}
