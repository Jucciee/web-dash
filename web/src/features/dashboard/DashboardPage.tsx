import { useState, useRef, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowDown, ArrowUp, ArrowDownToLine, CircleHelp, FolderPlus, Plus, Search, SlidersHorizontal, RefreshCw } from 'lucide-react'
import { api } from '../../api/client'
import type { Group, Service } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import { ServiceEditorDialog } from '../services/ServiceEditorDialog'
import { GroupEditorDialog } from './GroupEditorDialog'
import { ServiceCard } from './ServiceCard'
import { useSettings } from '../settings/useSettings'
import { defaultAppearance, defaultWidgets } from '../settings/defaults'
import { HomeWidgets } from '../monitoring/MonitoringPanels'
import { AddonWidgets } from '../addons/AddonWidgets'
import styles from './DashboardPage.module.css'

export function DashboardPage({ manage = false }: { manage?: boolean }) {
  const settings = useSettings()
  const appearance = settings.data?.appearance ?? defaultAppearance
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const [now, setNow] = useState(() => new Date())
  useEffect(() => { const timer = window.setInterval(() => setNow(new Date()), 60_000); return () => window.clearInterval(timer) }, [])
  const qc = useQueryClient()
  const [search, setSearch] = useState('')
  const [showHidden, setShowHidden] = useState(false)
  const [editor, setEditor] = useState<{ service?: Service; groupId?: string } | null>(null)
  const [groupEditor, setGroupEditor] = useState<{ group?: Group } | null>(null)
  const [undoService, setUndoService] = useState<Service | null>(null)
  const undoRef = useRef<HTMLButtonElement>(null)
  const list = useQuery({ queryKey: ['dashboards', 'list'], queryFn: api.dashboards })
  const dashboardId = list.data?.[0]?.id
  const query = useQuery({ queryKey: ['dashboards', dashboardId], queryFn: () => api.dashboard(dashboardId!), enabled: !!dashboardId })
  const dashboard = query.data
  const statuses = useQuery({ queryKey: ['web-statuses', dashboardId], queryFn: () => api.webStatuses(dashboardId!), enabled: !!dashboard, refetchOnWindowFocus: false, refetchOnReconnect: false, refetchInterval: query => query.state.data?.some(status => status.checking) ? 1000 : false })
  useEffect(() => { if (dashboard) void qc.invalidateQueries({ queryKey: ['web-statuses', dashboardId] }) }, [dashboard?.revision, dashboardId, qc])
  const check = useMutation({ mutationFn: () => api.checkWebStatuses(dashboardId!), onSuccess: async data => {
    // An earlier cached GET must not overwrite the newly queued batch and
    // stop result polling before that batch finishes.
    await qc.cancelQueries({ queryKey: ['web-statuses', dashboardId] })
    qc.setQueryData(['web-statuses', dashboardId], data)
  } })
  const { mutate: checkStatuses } = check
  const checkedPage = useRef('')
  // A visit, or a newly added/changed URL, starts one batch. Clock ticks,
  // focus/reconnect and completed-result reads never start another check.
  const checkKey = dashboard ? JSON.stringify([dashboard.id, manage, dashboard.groups.flatMap(group => group.services).map(link => [link.id, link.url, link.webCheckEnabled]).sort((a, b) => String(a[0]).localeCompare(String(b[0])))]) : ''
  useEffect(() => {
    if (!checkKey || checkedPage.current === checkKey) return
    checkedPage.current = checkKey
    checkStatuses()
  }, [checkKey, checkStatuses])
  const statusFor = (id: string) => statuses.data?.find(status => status.serviceId === id)
  const statusToolbar = <div className={styles.statusToolbar}><span>웹 상태 · 페이지 접속 시 확인</span><Button variant="ghost" onClick={() => check.mutate()} loading={check.isPending} disabled={check.isPending || !dashboard}><RefreshCw size={14} aria-hidden="true" />지금 확인</Button>{(statuses.isError || check.isError) && <ErrorNotice error={statuses.error ?? check.error} onRetry={() => check.mutate()} />}</div>
  const remove = useMutation({ mutationFn: api.deleteService, onSuccess: async (_, service) => { setUndoService(service); await qc.invalidateQueries({ queryKey: ['dashboards'] }); undoRef.current?.focus() }, onError: () => { void qc.invalidateQueries({ queryKey: ['dashboards'] }) } })
  const restore = useMutation({ mutationFn: async () => {
    if (!undoService) return
    const { groupId, name, url, iconKey, iconMode, iconUrl, iconAssetId, webCheckEnabled, description, hidden } = undoService
    const restored = await api.createService({ groupId, name, url, iconKey: iconKey as 'server', iconMode, iconUrl, iconAssetId, webCheckEnabled, description, hidden })
    setUndoService(null)
    const latest = await api.dashboard(dashboardId!)
    const layout = { groups: latest.groups.map(group => {
      const ids = group.services.map(service => service.id).filter(id => id !== restored.id)
      if (group.id === groupId) ids.splice(Math.min(undoService.position, ids.length), 0, restored.id)
      return { groupId: group.id, serviceIds: ids }
    }) }
    await api.saveLayout(latest, layout)
  }, onSuccess: async () => { setUndoService(null); await qc.invalidateQueries({ queryKey: ['dashboards'] }) }, onError: () => { void qc.invalidateQueries({ queryKey: ['dashboards'] }) } })
  const reorder = useMutation({ mutationFn: ({ group, service, direction }: { group: Group; service: Service; direction: -1 | 1 }) => {
    const ids = group.services.map(item => item.id)
    const index = ids.indexOf(service.id)
    const target = index + direction
    if (target < 0 || target >= ids.length) return Promise.resolve(dashboard!)
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    return api.saveLayout(dashboard!, { groups: dashboard!.groups.map(item => ({ groupId: item.id, serviceIds: item.id === group.id ? ids : item.services.map(card => card.id) })) })
  }, onSuccess: async () => { await qc.invalidateQueries({ queryKey: ['dashboards'] }) }, onError: () => { void qc.invalidateQueries({ queryKey: ['dashboards'] }) } })
  const pending = remove.isPending || restore.isPending || reorder.isPending
  const reorderGroup = useMutation({ mutationFn: ({ id, direction }: { id: string; direction: -1 | 1 }) => {
    const groups = [...dashboard!.groups]
    const index = groups.findIndex(group => group.id === id)
    const target = index + direction
    if (target < 0 || target >= groups.length) return Promise.resolve(dashboard!)
    ;[groups[index], groups[target]] = [groups[target], groups[index]]
    return api.saveLayout(dashboard!, { groups: groups.map(group => ({ groupId: group.id, serviceIds: group.services.map(service => service.id) })) })
  }, onSuccess: async () => { await qc.invalidateQueries({ queryKey: ['dashboards'] }) }, onError: () => { void qc.invalidateQueries({ queryKey: ['dashboards'] }) } })
  const allServices = dashboard?.groups.flatMap(group => group.services) ?? []
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const visible = (service: Service) => (showHidden || !service.hidden) && [service.name, service.url, service.description].some(value => value.toLocaleLowerCase().includes(normalizedSearch))
  const visibleCount = allServices.filter(visible).length
  if (list.isError || query.isError) return <ErrorNotice error={list.error ?? query.error} onRetry={() => { void list.refetch(); void query.refetch() }} />
  if (list.isPending || (!!dashboardId && query.isPending)) return <p className="helper" role="status">링크를 불러오는 중…</p>
  if (!dashboard) return <ErrorNotice error={new Error('기본 대시보드를 찾을 수 없습니다. 서버 데이터를 확인해주세요.')} />
  if (!manage) return <div className={`home-dashboard ${styles.homeDashboard}`}>
    {(appearance.showTitle || appearance.showClock) && <div className={styles.homeHeading}><div>{appearance.showTitle && <><h1>{settings.data?.title ?? dashboard.name}</h1>{settings.data?.subtitle && <p className={styles.subtitle}>{settings.data.subtitle}</p>}</>}</div>{appearance.showClock && <time className={styles.clock} dateTime={now.toISOString()}><strong>{now.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false })}</strong><span>{now.toLocaleDateString('ko-KR', { month: 'long', day: 'numeric', weekday: 'short' })}</span></time>}</div>}
    {appearance.showSearch && <label className={`${styles.search} ${styles.homeSearch}`}><Search size={18} aria-hidden="true" /><span className="sr-only">링크 검색</span><input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="링크 찾기" /></label>}
    {settings.isError && <ErrorNotice error={settings.error} onRetry={() => void settings.refetch()} />}
    {settings.data && <HomeWidgets settings={settings.data.widgets ?? defaultWidgets} />}
    <AddonWidgets />
    {allServices.length > 0 && appearance.showWebStatus && statusToolbar}
    {allServices.length === 0 && <section className={styles.homeEmpty}><h2>링크가 없습니다</h2><Link to="/settings/services" className="button">링크 추가</Link><Link to="/settings/imports" className={styles.importHint}>설정 가져오기</Link></section>}
    {allServices.length > 0 && visibleCount === 0 && <section className={styles.homeEmpty}><h2>표시할 링크가 없습니다</h2><p>{search ? '검색어를 바꿔보세요.' : '숨긴 링크는 설정에서 관리할 수 있습니다.'}</p>{search ? <Button onClick={() => setSearch('')}>검색 지우기</Button> : <Link to="/settings/services" className="button">링크 관리</Link>}</section>}
    <div className={`home-groups ${styles.homeGroups}`}>{dashboard.groups.map(group => {
      const cards = group.services.filter(service => !service.hidden && visible(service))
      if (!cards.length) return null
      const isCollapsed = appearance.collapsibleGroups && collapsed.has(group.id) && !search
      const heading = <>{group.name}{appearance.showGroupCounts && <span>{cards.length}</span>}</>
      return <section key={group.id} className={`service-group ${styles.homeGroup}`} aria-label={`${group.name} 그룹`}><h2 className="group-title">{appearance.collapsibleGroups ? <button className={styles.groupToggle} aria-expanded={!isCollapsed} aria-controls={`group-${group.id}`} onClick={() => setCollapsed(current => { const next = new Set(current); next.has(group.id) ? next.delete(group.id) : next.add(group.id); return next })}>{heading}</button> : heading}</h2><div id={`group-${group.id}`} hidden={isCollapsed} className={styles.homeCards}>{cards.map(service => <ServiceCard key={service.id} service={service} status={statusFor(service.id)} appearance={appearance} manage={false} onEdit={() => {}} onDelete={() => {}} onMove={() => {}} first last busy={false} />)}</div></section>
    })}</div>
  </div>
  return <>
    <div className={styles.pageHeader}><div><p className={styles.breadcrumb}>설정 <span>/</span> 링크</p><h1>링크 관리</h1></div><Button variant="primary" onClick={() => dashboard.groups.length ? setEditor({}) : setGroupEditor({})}><Plus size={18} aria-hidden="true" />{dashboard.groups.length ? '링크 추가' : '그룹 추가'}</Button></div>
    <div className={styles.toolbar}><label className={styles.search}><Search size={18} aria-hidden="true" /><span className="sr-only">링크 검색</span><input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="이름이나 주소로 검색" /></label><div className={styles.toolbarActions}><label className="checkbox-field"><input type="checkbox" checked={showHidden} onChange={event => setShowHidden(event.target.checked)} />숨긴 링크</label><Button variant="ghost" onClick={() => setGroupEditor({})}><FolderPlus size={17} aria-hidden="true" />그룹 추가</Button></div></div>
    {(remove.isError || reorder.isError || restore.isError || reorderGroup.isError) && <ErrorNotice error={remove.error ?? reorder.error ?? restore.error ?? reorderGroup.error} />}
    {allServices.length > 0 && statusToolbar}
    <p className={styles.results} role="status">{visibleCount}개 링크{allServices.some(service => service.hidden) && ` · ${allServices.filter(service => service.hidden).length}개 숨김`}</p>
    {allServices.length === 0 && <section className={styles.emptyState}><span className={styles.emptyIcon}><Plus size={26} aria-hidden="true" /></span><h2>링크가 없습니다</h2><Button variant="primary" onClick={() => dashboard.groups.length ? setEditor({}) : setGroupEditor({})}><Plus size={17} aria-hidden="true" />{dashboard.groups.length ? '링크 추가' : '그룹 추가'}</Button><Link to="/imports/new" className={styles.importHint}><ArrowDownToLine size={16} aria-hidden="true" />설정 가져오기 안내</Link></section>}
    {allServices.length > 0 && visibleCount === 0 && <section className={styles.emptyState}><CircleHelp size={26} aria-hidden="true" /><h2>표시할 링크가 없습니다</h2><p>검색어를 바꾸거나 숨긴 링크를 표시해보세요.</p><Button onClick={() => { setSearch(''); setShowHidden(true) }}>모든 링크 보기</Button></section>}
    {dashboard.groups.map((group, groupIndex) => {
      const cards = group.services.filter(visible)
      return <section key={group.id} className={styles.group} aria-label={`${group.name} 그룹`}><div className={styles.groupHeader}><h2>{group.name}<span>{cards.length}</span></h2><div><Button variant="ghost" className="icon-button" aria-label={`${group.name} 그룹 위로 이동`} disabled={groupIndex === 0 || pending || reorderGroup.isPending} onClick={() => reorderGroup.mutate({ id: group.id, direction: -1 })}><ArrowUp size={16} /></Button><Button variant="ghost" className="icon-button" aria-label={`${group.name} 그룹 아래로 이동`} disabled={groupIndex === dashboard.groups.length - 1 || pending || reorderGroup.isPending} onClick={() => reorderGroup.mutate({ id: group.id, direction: 1 })}><ArrowDown size={16} /></Button><Button variant="ghost" className="icon-button" aria-label={`${group.name} 그룹 편집`} onClick={() => setGroupEditor({ group })}><SlidersHorizontal size={17} /></Button><Button variant="ghost" className="icon-button" aria-label={`${group.name}에 링크 추가`} onClick={() => setEditor({ groupId: group.id })}><Plus size={18} /></Button></div></div>{cards.length > 0 ? <div className={styles.cards}>{cards.map(service => <ServiceCard key={service.id} service={service} status={statusFor(service.id)} onEdit={() => setEditor({ service })} onDelete={() => remove.mutate(service)} onMove={direction => reorder.mutate({ group, service, direction })} first={group.services[0].id === service.id} last={group.services.at(-1)?.id === service.id} busy={pending || reorderGroup.isPending} />)}</div> : <p className={styles.emptyGroup}>{group.services.length ? '검색 조건에 맞는 링크가 없습니다.' : '아직 링크가 없는 그룹입니다.'}</p>}</section>
    })}
    {undoService && <div className={styles.undoBar} role="status"><span>“{undoService.name}” 링크를 삭제했습니다.</span><Button ref={undoRef} onClick={() => restore.mutate()} loading={restore.isPending}>되돌리기</Button><Button variant="ghost" aria-label="삭제 알림 닫기" disabled={restore.isPending} onClick={() => setUndoService(null)}>닫기</Button></div>}
    {editor && <ServiceEditorDialog service={editor.service} defaultGroupId={editor.groupId} groups={dashboard.groups} onClose={() => setEditor(null)} />}
    {groupEditor && <GroupEditorDialog dashboardId={dashboard.id} group={groupEditor.group} onClose={() => setGroupEditor(null)} />}
  </>
}
