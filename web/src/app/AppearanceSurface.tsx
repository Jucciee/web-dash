import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Settings } from '../api/types'
import { defaultAppearance } from '../features/settings/defaults'
import styles from './AppearanceSurface.module.css'

// A scoped surface is shared by the real home and the unsaved preview.
// CSSOM properties work with the app's self-only stylesheet CSP.
export function AppearanceSurface({ settings, children, className = '' }: { settings?: Settings; children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const [dark, setDark] = useState(() => window.matchMedia('(prefers-color-scheme: dark)').matches)
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const apply = () => setDark(media.matches)
    media.addEventListener('change', apply)
    return () => media.removeEventListener('change', apply)
  }, [])
  const a = settings?.appearance ?? defaultAppearance
  useEffect(() => {
    const node = ref.current
    if (!node) return
    node.style.setProperty('--home-columns', String(a.columns || 3))
    node.style.setProperty('--home-accent-input', /^#[a-f\d]{6}$/i.test(a.accentColor) ? a.accentColor : '#356edb')
    node.style.setProperty('--home-dim', `${a.backgroundDim}%`)
    node.style.setProperty('--home-wallpaper', /^[a-f\d]{64}$/.test(a.backgroundImage) ? `url("/api/v1/backgrounds/${a.backgroundImage}")` : 'none')
  }, [a.columns, a.accentColor, a.backgroundDim, a.backgroundImage])
  return <div ref={ref} className={`${styles.surface} ${className}`} data-appearance="true" data-theme={settings?.theme === 'system' ? (dark ? 'dark' : 'light') : settings?.theme ?? 'light'} data-accent={settings?.accent ?? 'blue'} data-compact={String(settings?.compact ?? false)} data-layout={a.layout} data-columns={a.columns} data-width={a.width} data-card-style={a.cardStyle} data-icon-size={a.iconSize} data-radius={a.radius} data-font-size={a.fontSize} data-font-family={a.fontFamily} data-background={a.background}>{children}</div>
}
