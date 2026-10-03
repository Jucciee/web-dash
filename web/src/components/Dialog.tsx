import { useEffect, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { Button } from './Button'

export function Dialog({ title, children, onClose, busy = false }: { title: string; children: ReactNode; onClose: () => void; busy?: boolean }) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const element = ref.current
    const previousFocus = document.activeElement as HTMLElement | null
    element?.showModal()
    return () => { element?.close(); previousFocus?.focus() }
  }, [])
  return <dialog ref={ref} className="dialog" aria-labelledby="dialog-title" onCancel={event => { event.preventDefault(); if (!busy) onClose() }} onClick={event => { if (event.target === ref.current && !busy) onClose() }}>
    <div className="dialog-header"><h2 id="dialog-title">{title}</h2><Button variant="ghost" className="icon-button" aria-label="닫기" disabled={busy} onClick={onClose}><X size={20} /></Button></div>
    {children}
  </dialog>
}
