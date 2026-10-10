import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { ArrowDownToLine, Box, LayoutDashboard, LayoutGrid, LogOut, Menu, Puzzle, Server, Settings2, X } from 'lucide-react'
import { api } from '../api/client'
import { Button } from '../components/Button'
import { ErrorNotice } from '../components/Feedback'
import { useAuth } from './App'
import { Appearance } from './Appearance'
import { AppearanceSurface } from './AppearanceSurface'
import { useSettings } from '../features/settings/useSettings'
import styles from './AppShell.module.css'

const navigation = [
  { to: '/', label: '대시보드', icon: LayoutDashboard },
  { to: '/settings/services', label: '링크 관리', icon: LayoutDashboard },
  { to: '/settings/layout', label: '배치 편집', icon: LayoutGrid },
  { to: '/settings/imports', label: '설정 가져오기', icon: ArrowDownToLine },
  { to: '/connections', label: 'Docker 상태', icon: Box },
  { to: '/hosts', label: '호스트 상태', icon: Server },
  { to: '/settings/addons', label: '애드온', icon: Puzzle },
  { to: '/settings', label: '설정', icon: Settings2 },
]

export function AppShell() {
  const home = useLocation().pathname === '/'
  const settings = useSettings()
  const [menuOpen, setMenuOpen] = useState(false)
  const auth = useAuth()
  const qc = useQueryClient()
  const logout = useMutation({ mutationFn: api.logout, onSuccess: () => {
    qc.setQueryData(['auth'], null)
    qc.removeQueries({ predicate: query => !['auth', 'setup'].includes(String(query.queryKey[0])) })
  } })
  if (home) return <AppearanceSurface settings={settings.data} className={styles.homeShell}>
    <Appearance home />
    <a className="skip-link" href="#main">본문으로 이동</a>
    <header className={styles.homeBar}><Link to="/" className="wordmark">web-dash<span className="brand-dot" /></Link><div><Link to="/settings/layout" className="button button-ghost icon-button" aria-label="배치 편집" title="배치 편집"><LayoutGrid size={20} /></Link><Link to="/settings" className="button button-ghost icon-button" aria-label="설정 열기" title="설정"><Settings2 size={20} /></Link><Button variant="ghost" className="icon-button" aria-label="로그아웃" loading={logout.isPending} onClick={() => logout.mutate()}><LogOut size={18} /></Button></div></header>
    <main id="main" className={styles.homeMain}>{logout.isError && <ErrorNotice error={logout.error} />}<Outlet /></main>
    {settings.data?.appearance.showFooter !== false && <footer className={styles.homeFooter}><span>web-dash</span><Link to="/settings/services">링크 관리</Link></footer>}
  </AppearanceSurface>
  return <div className={styles.shell}>
    <Appearance home={false} />
    <a className="skip-link" href="#main">본문으로 이동</a>
    <header className={styles.mobileHeader}><span className="wordmark">web-dash<span className="brand-dot" /></span><Button variant="ghost" className="icon-button" aria-label={menuOpen ? '메뉴 닫기' : '메뉴 열기'} aria-expanded={menuOpen} aria-controls="sidebar" onClick={() => setMenuOpen(!menuOpen)}>{menuOpen ? <X size={22} /> : <Menu size={22} />}</Button></header>
    <aside id="sidebar" className={`${styles.sidebar} ${menuOpen ? styles.open : ''}`}>
      <NavLink to="/" className={styles.brand} onClick={() => setMenuOpen(false)}><span className="wordmark">web-dash<span className="brand-dot" /></span></NavLink>
      <nav aria-label="주 메뉴" className={styles.navigation}>{navigation.map(item => <NavLink key={item.to} to={item.to} end={item.to === '/' || item.to === '/settings'} className={({ isActive }) => `${styles.navLink} ${isActive ? styles.active : ''}`} onClick={() => setMenuOpen(false)}><item.icon size={19} aria-hidden="true" /><span>{item.label}</span></NavLink>)}</nav>
      <div className={styles.sidebarBottom}><div className={styles.account}><span className={styles.avatar}>{auth.data?.user.username.slice(0, 1).toUpperCase()}</span><span>{auth.data?.user.username}<small>관리자</small></span><Button variant="ghost" className="icon-button" aria-label="로그아웃" loading={logout.isPending} onClick={() => logout.mutate()}><LogOut size={18} /></Button></div>{logout.isError && <ErrorNotice error={logout.error} />}</div>
    </aside>
    <div className={styles.content}><main id="main" className={styles.main}><Outlet /></main><footer className={styles.footer}><span>web-dash</span><span className="mono">v0.4.0</span></footer></div>
  </div>
}
