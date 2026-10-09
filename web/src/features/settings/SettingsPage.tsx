import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ArrowDownToLine, ArrowRight, Box, LayoutGrid, Save, Server } from 'lucide-react'
import { api, APIError } from '../../api/client'
import type { AppearanceSettings, Settings, SettingsPatch } from '../../api/types'
import { Button } from '../../components/Button'
import { ErrorNotice } from '../../components/Feedback'
import { useSettings } from './useSettings'
import { defaultAppearance, presets } from './defaults'
import { AppearanceSurface } from '../../app/AppearanceSurface'
import { ServiceCard } from '../dashboard/ServiceCard'
import { AddonPreviewWidgets } from '../addons/AddonWidgets'
import dashboardStyles from '../dashboard/DashboardPage.module.css'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const query = useSettings()
  if (query.isPending) return <p role="status" className="helper">설정을 불러오는 중…</p>
  if (query.isError) return <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
  return <div className={styles.page}>
    <header className={styles.heading}><p className="breadcrumb">web-dash / 설정</p><h1>설정</h1></header>
    <nav className={styles.shortcuts} aria-label="설정 바로가기"><Link to="/settings/services"><LayoutGrid size={20} /><span><strong>링크 관리</strong><small>카드 · 그룹 · 표시 순서</small></span><ArrowRight size={18} /></Link><Link to="/settings/imports"><ArrowDownToLine size={20} /><span><strong>설정 가져오기</strong><small>Docker · Caddy · Nginx</small></span><ArrowRight size={18} /></Link></nav>
    <SettingsForm initial={query.data} />
  </div>
}

function SettingsForm({ initial }: { initial: Settings }) {
  const [draft, setDraft] = useState(initial)
  const [saved, setSaved] = useState(false)
  const [tab, setTab] = useState<'screen' | 'display' | 'widgets' | 'advanced'>('screen')
  const qc = useQueryClient()
  const save = useMutation({ mutationFn: () => {
    const { revision, ...body } = draft
    return api.updateSettings(body, revision)
  }, onSuccess: result => { qc.setQueryData(['settings'], result); setDraft(result); setSaved(true) } })
  const reload = useMutation({ mutationFn: api.settings, onSuccess: result => { qc.setQueryData(['settings'], result); setDraft(result); setSaved(false); save.reset() } })
  const dirty = JSON.stringify(draft) !== JSON.stringify(initial)
  function update<K extends keyof SettingsPatch>(key: K, value: SettingsPatch[K]) { setSaved(false); setDraft(current => ({ ...current, [key]: value })) }
  function appearance<K extends keyof AppearanceSettings>(key: K, value: AppearanceSettings[K]) { update('appearance', { ...draft.appearance, [key]: value }) }
  const upload = useMutation({ mutationFn: api.uploadBackground, onSuccess: result => { appearance('backgroundImage', result.id); setDraft(current => ({ ...current, appearance: { ...current.appearance, background: 'image' } })) } })
  const busy = save.isPending || reload.isPending || upload.isPending
  function submit(event: FormEvent) { event.preventDefault(); save.mutate() }
  const select = <K extends keyof AppearanceSettings>(label: string, key: K, options: [AppearanceSettings[K], string][]) => <label className="field">{label}<select value={String(draft.appearance[key])} disabled={busy} onChange={event => appearance(key, event.target.value as AppearanceSettings[K])}>{options.map(([value, name]) => <option key={String(value)} value={String(value)}>{name}</option>)}</select></label>
  const toggles: (keyof AppearanceSettings)[] = ['showTitle', 'showClock', 'showSearch', 'showIcons', 'showDescriptions', 'showUrls', 'showWebStatus', 'showGroupCounts', 'showFooter', 'collapsibleGroups']
  const labels = ['홈 제목 표시', '시계 표시', '검색창 표시', '아이콘 표시', '링크 설명 표시', '링크 주소 표시', '웹 상태 표시', '그룹 링크 수 표시', '하단 메뉴 표시', '그룹 접기 허용']
  return <div className={styles.editorLayout}><form className={styles.form} onSubmit={submit}>
    <fieldset disabled={busy} className={styles.presetField}><legend>프리셋</legend><div className={styles.presets}>{presets.map(preset => <Button key={preset.name} className={styles.preset} aria-pressed={Object.entries(preset.settings).every(([key, value]) => JSON.stringify(draft[key as keyof Settings]) === JSON.stringify(value))} onClick={() => { setSaved(false); setDraft(current => ({ ...current, ...preset.settings })) }}><strong>{preset.name}</strong><small>{preset.description}</small></Button>)}</div></fieldset>
    <nav className={styles.tabs} aria-label="설정 항목">{([['screen', '화면'], ['display', '표시 항목'], ['widgets', '위젯'], ['advanced', '고급']] as const).map(([value, label]) => <Button variant="ghost" key={value} aria-pressed={tab === value} onClick={() => setTab(value)}>{label}</Button>)}</nav>
    <div className={styles.tabContent}>
    {tab === 'screen' && <div className="form-stack">
      <h2>화면 스타일</h2><div className="form-row"><label className="field">테마<select value={draft.theme} disabled={busy} onChange={event => update('theme', event.target.value as Settings['theme'])}><option value="light">밝게</option><option value="dark">어둡게</option><option value="system">시스템 설정 따르기</option></select></label><label className="field">강조색<select value={draft.accent} disabled={busy} onChange={event => update('accent', event.target.value as Settings['accent'])}><option value="blue">블루</option><option value="green">그린</option><option value="amber">앰버</option><option value="slate">슬레이트</option><option value="violet">바이올렛</option><option value="rose">로즈</option><option value="custom">직접 선택</option></select></label></div>
      {draft.accent === 'custom' && <label className="field">사용자 강조색<input type="color" value={draft.appearance.accentColor || '#356edb'} disabled={busy} onChange={event => appearance('accentColor', event.target.value)} /><small>읽기 쉬운 밝기로 자동 조정합니다.</small></label>}
      <h2>배치</h2><div className="form-row">{select('그룹 배치', 'layout', [['grid', '나란히'], ['rows', '한 줄씩']])}<label className="field">열 수<select value={draft.appearance.columns} disabled={busy} onChange={event => appearance('columns', Number(event.target.value))}><option value={0}>자동</option>{[1, 2, 3, 4, 5, 6].map(value => <option key={value} value={value}>{value}열</option>)}</select></label>{select('화면 폭', 'width', [['narrow', '좁게'], ['normal', '기본'], ['wide', '넓게'], ['full', '전체 화면']])}{select('카드 모양', 'cardStyle', [['card', '카드'], ['tile', '타일'], ['list', '목록']])}{select('모서리', 'radius', [['square', '직각'], ['soft', '살짝 둥글게'], ['round', '둥글게']])}{select('아이콘 크기', 'iconSize', [['small', '작게'], ['medium', '기본'], ['large', '크게']])}</div>
      <label className="checkbox-field"><input type="checkbox" checked={draft.compact} disabled={busy} onChange={event => update('compact', event.target.checked)} />홈 카드를 촘촘하게 표시</label>
      <h2>글꼴과 배경</h2><div className="form-row">{select('글꼴', 'fontFamily', [['noto', 'Noto Sans KR'], ['system', '시스템 글꼴'], ['mono', '고정폭 글꼴']])}{select('글자 크기', 'fontSize', [['small', '작게'], ['normal', '기본'], ['large', '크게']])}</div>
      <label className="field">배경<select value={draft.appearance.background} disabled={busy} onChange={event => appearance('background', event.target.value as AppearanceSettings['background'])}><option value="plain">단색</option><option value="grid">격자</option><option value="dots">점</option><option value="image" disabled={!draft.appearance.backgroundImage}>이미지</option></select></label>
      <label className="field">배경 이미지 업로드<input type="file" accept="image/png,image/jpeg,image/webp" disabled={busy} onChange={event => { const file = event.target.files?.[0]; if (file) upload.mutate(file); event.target.value = '' }} /><small>PNG · JPG · WebP, 최대 8MiB / 6000px / 2400만 화소</small></label>
      {upload.isPending && <p className="helper" role="status">배경 이미지를 업로드하는 중…</p>}{upload.isError && <ErrorNotice error={upload.error} />}
      {draft.appearance.backgroundImage && <><label className="field">배경 가림 정도 <output>{draft.appearance.backgroundDim}%</output><input type="range" min={0} max={90} step={5} value={draft.appearance.backgroundDim} disabled={busy} onChange={event => appearance('backgroundDim', Number(event.target.value))} /></label><Button disabled={busy} onClick={() => update('appearance', { ...draft.appearance, background: 'plain', backgroundImage: '' })}>배경 이미지 해제</Button></>}
    </div>}
    {tab === 'display' && <div className="form-stack"><h2>표시 항목</h2><label className="field">홈 제목<input required maxLength={80} value={draft.title} disabled={busy} onChange={event => update('title', event.target.value)} /></label><label className="field">홈 설명<input maxLength={200} value={draft.subtitle} disabled={busy} onChange={event => update('subtitle', event.target.value)} /><small>비워두면 표시하지 않습니다.</small></label><div className={styles.toggles}>{toggles.map((key, index) => <label className="checkbox-field" key={key}><input type="checkbox" checked={Boolean(draft.appearance[key])} disabled={busy} onChange={event => appearance(key, event.target.checked)} />{labels[index]}</label>)}</div></div>}
    {tab === 'widgets' && <div className="form-stack"><h2>상태 위젯</h2><label className="checkbox-field"><input type="checkbox" checked={draft.widgets.showHost} disabled={busy} onChange={event => update('widgets', { ...draft.widgets, showHost: event.target.checked })} />호스트 위젯 표시</label><label className="checkbox-field"><input type="checkbox" checked={draft.widgets.showDocker} disabled={busy} onChange={event => update('widgets', { ...draft.widgets, showDocker: event.target.checked })} />Docker 위젯 표시</label><label className="field">위젯 순서<select value={draft.widgets.order[0]} disabled={busy} onChange={event => update('widgets', { ...draft.widgets, order: event.target.value === 'host' ? ['host', 'docker'] : ['docker', 'host'] })}><option value="host">호스트 → Docker</option><option value="docker">Docker → 호스트</option></select></label><label className="field">상태 갱신 간격<select value={draft.widgets.refreshSeconds} disabled={busy} onChange={event => update('widgets', { ...draft.widgets, refreshSeconds: Number(event.target.value) as Settings['widgets']['refreshSeconds'] })}><option value={0}>수동</option><option value={10}>10초</option><option value={30}>30초</option><option value={60}>60초</option></select></label><p className="helper">화면을 보고 있을 때만 갱신합니다.</p><div className={styles.monitorLinks}><Link to="/hosts"><Server size={16} />호스트 상태</Link><Link to="/connections"><Box size={16} />Docker 상태</Link><Link to="/settings/addons">API 애드온 관리</Link></div></div>}
    {tab === 'advanced' && <div className="form-stack"><h2>사용자 CSS</h2><div className="field"><label htmlFor="appearance-css">사용자 CSS</label><textarea id="appearance-css" className={styles.code} rows={12} spellCheck={false} value={draft.customCss} disabled={busy} onChange={event => update('customCss', event.target.value)} placeholder={'.service-card {\n  border-radius: 16px;\n}\n\n.group-title {\n  color: var(--color-accent);\n}'} aria-describedby="css-help" /></div><p id="css-help" className="helper">홈에만 적용됩니다. 미리보기에는 적용하지 않습니다. .home-dashboard, .service-card, .service-title, .service-description, .service-url, .group-title, .home-widgets, .host-widget, .docker-widget을 사용할 수 있습니다. 최대 32KiB.</p><Button onClick={() => update('customCss', '')} disabled={busy || !draft.customCss}>CSS 지우기</Button><Button disabled={busy} onClick={() => { setSaved(false); setDraft(current => ({ ...current, appearance: defaultAppearance, customCss: '' })) }}>화면 세부 설정 초기화</Button></div>}
    </div>
    {(save.isError || reload.isError) && <ErrorNotice error={reload.error ?? save.error} onRetry={save.error instanceof APIError && save.error.status === 409 ? () => reload.mutate() : undefined} />}
    <div className={styles.actions}><p className="helper" role="status">{dirty ? '저장하지 않은 변경사항이 있습니다.' : saved ? '설정을 저장했습니다.' : ''}</p><div><Button disabled={!dirty || busy} onClick={() => { setDraft(initial); setSaved(false); save.reset() }}>변경 취소</Button><Button type="submit" variant="primary" loading={save.isPending} disabled={!dirty || busy}><Save size={17} />변경 저장</Button></div></div>
  </form><SettingsPreview draft={draft} /></div>
}

function SettingsPreview({ draft }: { draft: Settings }) {
  const list = useQuery({ queryKey: ['dashboards', 'list'], queryFn: api.dashboards })
  const id = list.data?.[0]?.id
  const query = useQuery({ queryKey: ['dashboards', id], queryFn: () => api.dashboard(id!), enabled: !!id })
  const groups = query.data?.groups.filter(group => group.services.some(service => !service.hidden)).slice(0, 3) ?? []
  return <aside className={styles.previewWrap} aria-label="홈 화면 미리보기"><div className={styles.previewHeading}><h2>미리보기</h2><Link to="/">홈 열기<ArrowRight size={15} /></Link></div><AppearanceSurface settings={draft} className={styles.preview}><div className={`home-dashboard ${dashboardStyles.homeDashboard}`} inert>
    {(draft.appearance.showTitle || draft.appearance.showClock) && <div className={dashboardStyles.homeHeading}><div>{draft.appearance.showTitle && <><h2>{draft.title || 'web-dash'}</h2>{draft.subtitle && <p className={dashboardStyles.subtitle}>{draft.subtitle}</p>}</>}</div>{draft.appearance.showClock && <time className={dashboardStyles.clock}><strong>{new Date().toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false })}</strong></time>}</div>}
    {draft.appearance.showSearch && <div className={styles.previewSearch}>링크 찾기</div>}
    {(draft.widgets.showHost || draft.widgets.showDocker) && <div className={styles.previewWidgets}>{draft.widgets.order.map(kind => (kind === 'host' ? draft.widgets.showHost : draft.widgets.showDocker) && <div key={kind}>{kind === 'host' ? <Server size={16} /> : <Box size={16} />}<span>{kind === 'host' ? '호스트 · CPU / 메모리 / 디스크' : 'Docker · 컨테이너 상태'}<small>—</small></span></div>)}</div>}
    <AddonPreviewWidgets />
    <div className={dashboardStyles.homeGroups}>{groups.length ? groups.map(group => <section className={dashboardStyles.homeGroup} key={group.id}><h2 className="group-title">{group.name}{draft.appearance.showGroupCounts && <span>{group.services.filter(service => !service.hidden).length}</span>}</h2><div className={dashboardStyles.homeCards}>{group.services.filter(service => !service.hidden).slice(0, 4).map(service => <ServiceCard key={service.id} service={service} appearance={draft.appearance} manage={false} onEdit={() => {}} onDelete={() => {}} onMove={() => {}} first last busy={false} />)}</div></section>) : <section className={dashboardStyles.homeGroup}><h2 className="group-title">샘플 링크</h2><div className={dashboardStyles.homeCards}>{['홈서버', '즐겨찾기'].map(name => <div className={`service-card ${dashboardStyles.homeCard}`} key={name}>{draft.appearance.showIcons && <Server size={22} />}<span className={dashboardStyles.homeCardCopy}><strong>{name}</strong>{draft.appearance.showDescriptions && <small>링크 설명</small>}{draft.appearance.showUrls && <small>example.com</small>}</span></div>)}</div></section>}</div>
  </div>{draft.appearance.showFooter && <p className={styles.previewFooter}>web-dash</p>}</AppearanceSurface><p className="helper">저장 전 화면 설정을 확인할 수 있습니다. 상태 값은 홈에서 확인하세요.</p></aside>
}
