import { useState, useRef, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowDownToLine, CircleHelp, FolderPlus, Plus, Search, SlidersHorizontal, RefreshCw } from 'lucide-react'
import { api } from '../../api/client'
import type { Group, Service } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import { ServiceEditorDialog } from '../services/ServiceEditorDialog'
import { GroupEditorDialog } from './GroupEditorDialog'
import { ServiceCard } from './ServiceCard'
import { useSettings } from '../settings/useSettings'
import styles from './DashboardPage.module.css'

export function DashboardPage({ manage = false }: { manage?: boolean }) {
  const settings = useSettings()
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
  const statuses = useQuery({ queryKey: ['web-statuses', dashboardId], queryFn: () => api.webStatuses(dashboardId!), enabled: !!dashboard, refetchInterval: query => query.state.data?.some(status => status.checking) ? 1000 : 15_000 })
  useEffect(() => { if (dashboard) void qc.invalidateQueries({ queryKey: ['web-statuses', dashboardId] }) }, [dashboard?.revision, dashboardId, qc])
  const check = useMutation({ mutationFn: () => api.checkWebStatuses(dashboardId!), onSuccess: data => qc.setQueryData(['web-statuses', dashboardId], data) })
  const statusFor = (id: string) => statuses.data?.find(status => status.serviceId === id)
  const statusToolbar = <div className={styles.statusToolbar}><span>웹 상태 · 서버에서 30초마다 확인</span><Button variant="ghost" onClick={() => check.mutate()} loading={check.isPending} disabled={check.isPending || !dashboard}><RefreshCw size={14} aria-hidden="true" />지금 확인</Button>{(statuses.isError || check.isError) && <ErrorNotice error={statuses.error ?? check.error} onRetry={() => void statuses.refetch()} />}</div>
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
  const allServices = dashboard?.groups.flatMap(group => group.services) ?? []
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const visible = (service: Service) => (showHidden || !service.hidden) && [service.name, service.url, service.description].some(value => value.toLocaleLowerCase().includes(normalizedSearch))
  const visibleCount = allServices.filter(visible).length
  if (list.isError || query.isError) return <ErrorNotice error={list.error ?? query.error} onRetry={() => { void list.refetch(); void query.refetch() }} />
  if (list.isPending || (!!dashboardId && query.isPending)) return <p className="helper" role="status">링크를 불러오는 중…</p>
  if (!dashboard) return <ErrorNotice error={new Error('기본 대시보드를 찾을 수 없습니다. 서버 데이터를 확인해주세요.')} />
  if (!manage) return <div className={`home-dashboard ${styles.homeDashboard}`}>
    <div className={styles.homeHeading}><div><p className={styles.homeEyebrow}>내 링크</p><h1>{settings.data?.title ?? dashboard.name}</h1><p className={styles.subtitle}>{settings.data?.subtitle ?? '자주 쓰는 링크, 한곳에서.'}</p></div><time className={styles.clock} dateTime={now.toISOString()}><strong>{now.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false })}</strong><span>{now.toLocaleDateString('ko-KR', { month: 'long', day: 'numeric', weekday: 'short' })}</span></time></div>
    <label className={`${styles.search} ${styles.homeSearch}`}><Search size={18} aria-hidden="true" /><span className="sr-only">링크 검색</span><input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="링크 찾기" /></label>
    {settings.isError && <ErrorNotice error={settings.error} onRetry={() => void settings.refetch()} />}
    {allServices.length > 0 && statusToolbar}
    {allServices.length === 0 && <section className={styles.homeEmpty}><h2>자주 쓰는 링크를 모아보세요</h2><p>설정에서 링크를 추가하거나 기존 설정을 가져올 수 있습니다.</p><Link to="/settings/services" className="button">링크 관리</Link><Link to="/settings/imports" className={styles.importHint}>Docker · Caddy · Nginx 가져오기</Link></section>}
    {allServices.length > 0 && visibleCount === 0 && <section className={styles.homeEmpty}><h2>표시할 링크가 없습니다</h2><p>{search ? '검색어를 바꿔보세요.' : '숨긴 링크는 설정에서 관리할 수 있습니다.'}</p>{search ? <Button onClick={() => setSearch('')}>검색 지우기</Button> : <Link to="/settings/services" className="button">링크 관리</Link>}</section>}
    <div className={`home-groups ${styles.homeGroups}`}>{dashboard.groups.map(group => {
      const cards = group.services.filter(service => !service.hidden && visible(service))
      if (!cards.length) return null
      return <section key={group.id} className={`service-group ${styles.homeGroup}`} aria-label={`${group.name} 그룹`}><h2 className="group-title">{group.name}<span>{cards.length}</span></h2><div className={styles.homeCards}>{cards.map(service => <ServiceCard key={service.id} service={service} status={statusFor(service.id)} manage={false} onEdit={() => {}} onDelete={() => {}} onMove={() => {}} first last busy={false} />)}</div></section>
    })}</div>
  </div>
  return <>
    <div className={styles.pageHeader}><div><p className={styles.breadcrumb}>설정 <span>/</span> 링크</p><h1>링크 관리</h1><p className={styles.subtitle}>카드와 그룹을 편집하고 홈에 표시할 링크를 선택하세요.</p></div><Button variant="primary" onClick={() => dashboard.groups.length ? setEditor({}) : setGroupEditor({})}><Plus size={18} aria-hidden="true" />{dashboard.groups.length ? '링크 추가' : '그룹 추가'}</Button></div>
    <div className={styles.toolbar}><label className={styles.search}><Search size={18} aria-hidden="true" /><span className="sr-only">링크 검색</span><input type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder="이름이나 주소로 검색" /></label><div className={styles.toolbarActions}><label className="checkbox-field"><input type="checkbox" checked={showHidden} onChange={event => setShowHidden(event.target.checked)} />숨긴 링크</label><Button variant="ghost" onClick={() => setGroupEditor({})}><FolderPlus size={17} aria-hidden="true" />그룹 추가</Button></div></div>
    {(remove.isError || reorder.isError || restore.isError) && <ErrorNotice error={remove.error ?? reorder.error ?? restore.error} />}
    {allServices.length > 0 && statusToolbar}
    <p className={styles.results} role="status">{visibleCount}개 링크{allServices.some(service => service.hidden) && ` · ${allServices.filter(service => service.hidden).length}개 숨김`}</p>
    {allServices.length === 0 && <section className={styles.emptyState}><span className={styles.emptyIcon}><Plus size={26} aria-hidden="true" /></span><h2>첫 링크를 추가해보세요</h2><p>이름과 접속 주소만 있으면 됩니다.<br />홈서버 앱과 자주 방문하는 웹사이트를 한곳에 모아보세요.</p><Button variant="primary" onClick={() => dashboard.groups.length ? setEditor({}) : setGroupEditor({})}><Plus size={17} aria-hidden="true" />{dashboard.groups.length ? '링크 추가' : '그룹 추가'}</Button><Link to="/imports/new" className={styles.importHint}><ArrowDownToLine size={16} aria-hidden="true" />설정 가져오기 안내</Link></section>}
    {allServices.length > 0 && visibleCount === 0 && <section className={styles.emptyState}><CircleHelp size={26} aria-hidden="true" /><h2>표시할 링크가 없습니다</h2><p>검색어를 바꾸거나 숨긴 링크를 표시해보세요.</p><Button onClick={() => { setSearch(''); setShowHidden(true) }}>모든 링크 보기</Button></section>}
    {dashboard.groups.map(group => {
      const cards = group.services.filter(visible)
      return <section key={group.id} className={styles.group} aria-label={`${group.name} 그룹`}><div className={styles.groupHeader}><h2>{group.name}<span>{cards.length}</span></h2><div><Button variant="ghost" className="icon-button" aria-label={`${group.name} 그룹 편집`} onClick={() => setGroupEditor({ group })}><SlidersHorizontal size={17} /></Button><Button variant="ghost" className="icon-button" aria-label={`${group.name}에 링크 추가`} onClick={() => setEditor({ groupId: group.id })}><Plus size={18} /></Button></div></div>{cards.length > 0 ? <div className={styles.cards}>{cards.map(service => <ServiceCard key={service.id} service={service} status={statusFor(service.id)} onEdit={() => setEditor({ service })} onDelete={() => remove.mutate(service)} onMove={direction => reorder.mutate({ group, service, direction })} first={group.services[0].id === service.id} last={group.services.at(-1)?.id === service.id} busy={pending} />)}</div> : <p className={styles.emptyGroup}>{group.services.length ? '검색 조건에 맞는 링크가 없습니다.' : '아직 링크가 없는 그룹입니다.'}</p>}</section>
    })}
    {undoService && <div className={styles.undoBar} role="status"><span>“{undoService.name}” 링크를 삭제했습니다.</span><Button ref={undoRef} onClick={() => restore.mutate()} loading={restore.isPending}>되돌리기</Button><Button variant="ghost" aria-label="삭제 알림 닫기" disabled={restore.isPending} onClick={() => setUndoService(null)}>닫기</Button></div>}
    {editor && <ServiceEditorDialog service={editor.service} defaultGroupId={editor.groupId} groups={dashboard.groups} onClose={() => setEditor(null)} />}
    {groupEditor && <GroupEditorDialog dashboardId={dashboard.id} group={groupEditor.group} onClose={() => setGroupEditor(null)} />}
  </>
}
