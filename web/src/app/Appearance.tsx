import { useEffect } from 'react'
import { useSettings } from '../features/settings/useSettings'

export function Appearance({ home }: { home: boolean }) {
  const { data } = useSettings()
  useEffect(() => {
    if (!data) return
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const apply = () => {
      document.documentElement.dataset.theme = data.theme === 'system' ? (media.matches ? 'dark' : 'light') : data.theme
      document.documentElement.dataset.accent = data.accent
      document.documentElement.dataset.compact = String(data.compact)
      document.documentElement.style.setProperty('--home-accent-input', /^#[a-f\d]{6}$/i.test(data.appearance.accentColor) ? data.appearance.accentColor : '#356edb')
      document.title = `${data.title} · web-dash`
    }
    apply()
    media.addEventListener('change', apply)
    return () => media.removeEventListener('change', apply)
  }, [data])
  useEffect(() => {
    if (!home || !data?.customCss.trim()) return
    // Keep CSS off management pages, so a hidden home can always be repaired.
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = `/api/v1/settings/custom.css?revision=${data.revision}`
    link.dataset.customCss = 'true'
    document.head.append(link)
    return () => link.remove()
  }, [home, data?.customCss, data?.revision])
  return null
}
