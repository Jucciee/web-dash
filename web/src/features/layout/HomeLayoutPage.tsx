import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useBeforeUnload } from 'react-router-dom'
import { ArrowLeft, FolderPlus, GripVertical, RotateCcw, Save, Undo2, Redo2, Trash2 } from 'lucide-react'
import { api, APIError } from '../../api/client'
import type { Addon, Dashboard, HomeItem, HomeLayout, HomeSection, Settings } from '../../api/types'
import { AppearanceSurface } from '../../app/AppearanceSurface'
import { Button } from '../../components/Button'
import { Dialog } from '../../components/Dialog'
import { ErrorNotice } from '../../components/Feedback'
import { HomeCanvas } from './HomeCanvas'
import { appendItem, autoLayout, catalogFor, defaultHomeLayout, defaultSize, itemKey, layoutFor, moveItem, positionSections, reconcileLayout, withSections, type CatalogEntry } from './layout'
import { useCanvasGestures } from './useCanvasGestures'
import styles from './HomeLayout.module.css'

export function HomeLayoutPage() {
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings })
  const boards = useQuery({ queryKey: ['dashboards', 'list'], queryFn: api.dashboards })
  const id = boards.data?.[0]?.id
  const dashboard = useQuery({ queryKey: ['dashboards', id], queryFn: () => api.dashboard(id!), enabled: !!id })
  const addons = useQuery({ queryKey: ['addons', 'list'], queryFn: api.addons })
  if (settings.isError || boards.isError || dashboard.isError || addons.isError) return <ErrorNotice error={settings.error ?? boards.error ?? dashboard.error ?? addons.error} onRetry={() => { void settings.refetch(); void boards.refetch(); void dashboard.refetch(); void addons.refetch() }} />
  if (!settings.data || !dashboard.data || !addons.data) return <p className="helper" role="status">홈 배치를 불러오는 중…</p>
  return <LayoutEditor initial={settings.data} dashboard={dashboard.data} addons={addons.data} />
}

function LayoutEditor({ initial, dashboard, addons }: { initial: Settings; dashboard: Dashboard; addons: Addon[] }) {
  const catalog = useMemo(() => catalogFor(dashboard, addons, initial), [dashboard, addons, initial])
  const [base, setBase] = useState(() => ({ revision: initial.revision, layout: layoutFor(initial, dashboard, catalog), mode: initial.homeLayout.mode }))
  const [history, setHistory] = useState(() => ({ past: [] as HomeLayout[], present: base.layout, future: [] as HomeLayout[] }))
  const [dialog, setDialog] = useState<{ kind: 'item' | 'section' | 'new' | 'picker'; id: string } | null>(null)
  const [saved, setSaved] = useState(false)
  const qc = useQueryClient()
  const layout = history.present.mode === 'auto' ? autoLayout(dashboard, catalog, initial) : reconcileLayout(history.present, catalog)
  const dirty = JSON.stringify(history.present) !== JSON.stringify(base.layout)
  const canSave = dirty || base.mode === 'auto'
  useBeforeUnload(event => { if (dirty) { event.preventDefault(); event.returnValue = '' } })
  const save = useMutation({ mutationFn: () => api.updateSettings({ homeLayout: history.present.mode === 'auto' ? defaultHomeLayout : layout }, base.revision), onSuccess: result => {
    qc.setQueryData(['settings'], result)
    const next = layoutFor(result, dashboard, catalog)
    setBase({ revision: result.revision, layout: next, mode: result.homeLayout.mode })
    setHistory({ past: [], present: next, future: [] }); setSaved(true)
  } })
  const reload = useMutation({ mutationFn: api.settings, onSuccess: result => {
    qc.setQueryData(['settings'], result)
    const next = layoutFor(result, dashboard, catalogFor(dashboard, addons, result))
    setBase({ revision: result.revision, layout: next, mode: result.homeLayout.mode })
    setHistory({ past: [], present: next, future: [] }); setSaved(false); save.reset()
  } })
  function commit(next: HomeLayout) {
    setHistory(current => JSON.stringify(current.present) === JSON.stringify(next) ? current : { past: [...current.past, current.present].slice(-50), present: next, future: [] })
    setSaved(false); save.reset()
  }
  const gestures = useCanvasGestures(layout, commit)
  const busy = save.isPending || reload.isPending || !!gestures.active
  function undo() { setHistory(current => current.past.length ? { past: current.past.slice(0, -1), present: current.past.at(-1)!, future: [current.present, ...current.future] } : current); setSaved(false); save.reset() }
  function redo() { setHistory(current => current.future.length ? { past: [...current.past, current.present], present: current.future[0], future: current.future.slice(1) } : current); setSaved(false); save.reset() }
  const selectedSection = dialog && layout.sections.find(section => section.id === dialog.id)
  const selectedItem = dialog && layout.sections.flatMap(section => section.items).find(item => itemKey(item) === dialog.id)
  return <div className={styles.page}>
    <div className={styles.heading}><div><h1>배치 편집</h1><p>링크와 위젯을 섹션에 묶고, 원하는 위치와 크기로 조절하세요.</p></div><Link className="button button-ghost" to="/"><ArrowLeft size={16} />홈으로</Link></div>
    <div className={styles.toolbar}><div><Button disabled={busy || layout.sections.length >= 60} onClick={() => setDialog({ kind: 'new', id: '' })}><FolderPlus size={17} />섹션 추가</Button><Button variant="ghost" className="icon-button" aria-label="배치 되돌리기" disabled={busy || !history.past.length} onClick={undo}><Undo2 size={18} /></Button><Button variant="ghost" className="icon-button" aria-label="배치 다시 실행" disabled={busy || !history.future.length} onClick={redo}><Redo2 size={18} /></Button><Button variant="ghost" disabled={busy} onClick={() => commit(defaultHomeLayout)}><RotateCcw size={16} />자동 배치</Button></div><div><Button disabled={busy || !dirty} onClick={() => { setHistory({ past: [], present: base.layout, future: [] }); setSaved(false); save.reset() }}>변경 취소</Button><Button variant="primary" disabled={busy || !canSave} loading={save.isPending} onClick={() => save.mutate()}><Save size={17} />배치 저장</Button></div></div>
    <div className={styles.help}><GripVertical size={17} /><div><p>섹션과 항목의 손잡이를 끌어 원하는 곳에 놓으세요. 항목의 오른쪽 아래 모서리와 섹션 상단의 크기 버튼으로 크기를 조절합니다. 톱니 버튼에서 위치와 크기를 직접 지정할 수도 있습니다.</p><details><summary>키보드·모바일 조작</summary><p>손잡이에서 방향키는 이동, Shift + 방향키는 크기 조절입니다. 좁은 화면은 한 열로 표시하며 넓은 화면의 좌표와 크기를 유지합니다. 섹션 위치는 모바일에서도 톱니 설정으로 지정할 수 있습니다.</p></details></div></div>
    {(save.isError || reload.isError) && <ErrorNotice error={reload.error ?? save.error} />}
    {save.error instanceof APIError && save.error.status === 409 && <div><p className="helper">불러오면 현재 편집 내용을 최신 배치로 교체합니다.</p><Button disabled={busy} onClick={() => reload.mutate()}>최신 배치 불러오기</Button></div>}
    <p className={styles.status} role="status" data-saved={saved}>{gestures.active ? '드래그 중 · Esc로 취소' : saved ? '배치를 저장했습니다.' : dirty ? '저장하지 않은 배치가 있습니다.' : base.mode === 'auto' ? '자동 배치를 바탕으로 편집 중 · 저장하면 사용자 지정 배치가 됩니다.' : '저장된 배치'}</p>
    <AppearanceSurface settings={initial} className={styles.surface}><HomeCanvas layout={gestures.live ?? layout} catalog={catalog} settings={initial} preview editor={{ busy: save.isPending || reload.isPending, active: gestures.active, start: gestures.start, keyboard: gestures.keyboard, selectItem: id => setDialog({ kind: 'item', id }), selectSection: id => setDialog({ kind: 'section', id }), addItems: id => setDialog({ kind: 'picker', id }) }} /></AppearanceSurface>
    {!layout.sections.length && <div className={styles.blank}><p>섹션을 추가해 홈 화면을 구성하세요.</p></div>}
    {dialog?.kind === 'new' && <SectionDialog otherSections={layout.sections} onClose={() => setDialog(null)} onApply={section => { commit(withSections(layout, [...layout.sections, section], section.id)); setDialog(null) }} />}
    {dialog?.kind === 'section' && selectedSection && <SectionDialog section={selectedSection} onClose={() => setDialog(null)} onApply={section => { commit(withSections(layout, layout.sections.map(current => current.id === section.id ? section : current), section.id)); setDialog(null) }} onDelete={targetId => {
      const remaining = layout.sections.filter(section => section.id !== selectedSection.id)
      if (selectedSection.items.length) {
        let target = remaining.find(section => section.id === targetId)
        if (!target) { target = { id: newId(), name: '새 항목', width: 12, minRows: 0, collapsed: false, items: [] }; remaining.push(target) }
        const nextItems = selectedSection.items.reduce((items, item) => appendItem(items, item), target.items)
        remaining[remaining.indexOf(target)] = { ...target, items: nextItems }
      }
      commit(withSections(layout, remaining, targetId)); setDialog(null)
    }} otherSections={layout.sections.filter(section => section.id !== selectedSection.id)} />}
    {dialog?.kind === 'item' && selectedItem && <ItemDialog item={selectedItem} name={catalog.find(entry => entry.key === dialog.id)?.name ?? '항목'} sections={layout.sections} onClose={() => setDialog(null)} onApply={(sectionId, rectangle) => { commit(moveItem(layout, dialog.id, sectionId, rectangle)); setDialog(null) }} />}
    {dialog?.kind === 'picker' && selectedSection && <ItemPicker section={selectedSection} layout={layout} catalog={catalog} onClose={() => setDialog(null)} onApply={keys => { let next = layout; for (const key of keys) next = moveItem(next, key, selectedSection.id); commit(next); setDialog(null) }} />}
  </div>
}

function newId() {
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 15) | 64; bytes[8] = (bytes[8] & 63) | 128
  const hex = [...bytes].map(byte => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

function SectionDialog({ section, onClose, onApply, onDelete, otherSections = [] }: { section?: HomeSection; onClose: () => void; onApply: (section: HomeSection) => void; onDelete?: (targetId: string) => void; otherSections?: HomeSection[] }) {
  const [draft, setDraft] = useState<HomeSection>(() => section ?? positionSections([...otherSections, { id: newId(), name: '', width: 6, minRows: 0, collapsed: false, items: [] }]).at(-1)!)
  const [target, setTarget] = useState(otherSections[0]?.id ?? '')
  function submit(event: FormEvent) { event.preventDefault(); if (draft.name.trim()) onApply({ ...draft, name: draft.name.trim() }) }
  return <Dialog title={section ? `${section.name} 섹션 설정` : '섹션 추가'} onClose={onClose}><form onSubmit={submit} className={styles.dialogBody}>
    <label className="field">섹션 이름<input required maxLength={80} autoFocus value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} /></label>
    <div className={styles.dialogGrid}><div className="field"><label htmlFor="section-width">섹션 너비 (칸)</label><input id="section-width" type="number" min={3} max={12} required aria-describedby="section-width-help" value={draft.width} onChange={event => { const width = Number(event.target.value); setDraft({ ...draft, width, x: width >= 3 && width <= 12 ? Math.min(draft.x ?? 0, 12 - width) : draft.x }) }} /><small id="section-width-help">12칸이 화면 전체 너비입니다.</small></div><div className="field"><label htmlFor="section-height">섹션 최소 높이 (칸)</label><input id="section-height" type="number" min={0} max={48} required aria-describedby="section-height-help" value={draft.minRows} onChange={event => setDraft({ ...draft, minRows: Number(event.target.value) })} /><small id="section-height-help">0은 내용에 맞춰 자동 조절합니다.</small></div><label className="field">섹션 가로 위치 (칸)<input type="number" min={1} max={13 - draft.width} required value={(draft.x ?? 0) + 1} onChange={event => setDraft({ ...draft, x: Number(event.target.value) - 1 })} /></label><label className="field">섹션 세로 위치 (칸)<input type="number" min={1} max={65536 - Math.max(draft.minRows, ...draft.items.map(item => item.y + item.height), draft.items.length ? 1 : 3)} required value={(draft.y ?? 0) + 1} onChange={event => setDraft({ ...draft, y: Number(event.target.value) - 1 })} /></label></div>
    <p className="helper">빈 공간에도 배치할 수 있습니다. 겹치는 섹션은 아래로 이동합니다.</p>
    <label className="checkbox-field"><input type="checkbox" checked={draft.collapsed} onChange={event => setDraft({ ...draft, collapsed: event.target.checked })} />홈에서 처음에 접어두기</label>
    {section && section.items.length > 0 && <label className="field">섹션 삭제 시 항목 이동<select value={target} onChange={event => setTarget(event.target.value)}>{otherSections.length ? otherSections.map(other => <option key={other.id} value={other.id}>{other.name}</option>) : <option value="">새 항목 섹션 만들기</option>}</select><small>링크와 위젯은 선택한 섹션으로 옮겨집니다.</small></label>}
    <div className={styles.dialogActions}>{onDelete ? <Button variant="danger" onClick={() => onDelete(target)}><Trash2 size={16} />섹션 삭제</Button> : <span />}<div><Button onClick={onClose}>취소</Button><Button type="submit" variant="primary" disabled={!draft.name.trim()}>적용</Button></div></div>
  </form></Dialog>
}

function ItemDialog({ item, name, sections, onClose, onApply }: { item: HomeItem; name: string; sections: HomeSection[]; onClose: () => void; onApply: (sectionId: string, rectangle: HomeItem) => void }) {
  const [draft, setDraft] = useState(item)
  const [sectionId, setSectionId] = useState(sections.find(section => section.items.some(card => itemKey(card) === itemKey(item)))!.id)
  function submit(event: FormEvent) { event.preventDefault(); onApply(sectionId, draft) }
  const size = defaultSize(item.kind)
  return <Dialog title={`${name} 배치 설정`} onClose={onClose}><form className={styles.dialogBody} onSubmit={submit}>
    <label className="field">소속 섹션<select value={sectionId} onChange={event => { setSectionId(event.target.value); setDraft({ ...draft, x: 0, y: 0 }) }}>{sections.map(section => <option key={section.id} value={section.id}>{section.name}</option>)}</select></label>
    <div className={styles.presets}>{([['작게', 4, Math.max(2, size.height - 2)], ['기본', size.width, size.height], ['크게', 12, Math.min(24, size.height + 2)]] as const).map(([label, width, height]) => <Button key={label} onClick={() => setDraft({ ...draft, width, height, x: Math.min(draft.x, 12 - width) })}>{label}</Button>)}</div>
    <div className={styles.dialogGrid}><label className="field">항목 너비 (칸)<input type="number" min={1} max={12} required value={draft.width || ''} onChange={event => { const width = Number(event.target.value); setDraft({ ...draft, width, x: width > 0 && width <= 12 ? Math.min(draft.x, 12 - width) : draft.x }) }} /></label><label className="field">항목 높이 (칸)<input type="number" min={1} max={24} required value={draft.height || ''} onChange={event => setDraft({ ...draft, height: Number(event.target.value) })} /></label><label className="field">가로 위치 (칸)<input type="number" min={1} max={13 - draft.width} required value={draft.x < 0 ? '' : draft.x + 1} onChange={event => setDraft({ ...draft, x: Number(event.target.value) - 1 })} /></label><label className="field">세로 위치 (칸)<input type="number" min={1} max={4097 - draft.height} required value={draft.y < 0 ? '' : draft.y + 1} onChange={event => setDraft({ ...draft, y: Number(event.target.value) - 1 })} /></label></div>
    <p className="helper">12칸이 섹션 전체 너비입니다. 겹치는 항목은 아래로 이동합니다.</p>
    <div className={styles.dialogActions}><span /><div><Button onClick={onClose}>취소</Button><Button variant="primary" type="submit">적용</Button></div></div>
  </form></Dialog>
}

function ItemPicker({ section, layout, catalog, onClose, onApply }: { section: HomeSection; layout: HomeLayout; catalog: CatalogEntry[]; onClose: () => void; onApply: (keys: string[]) => void }) {
  const [search, setSearch] = useState('')
  const [checked, setChecked] = useState<string[]>([])
  const available = catalog.filter(entry => entry.enabled && !section.items.some(item => itemKey(item) === entry.key))
  const kinds = { link: '링크', addon: '애드온', host: '호스트', docker: 'Docker' }
  return <Dialog title={`${section.name}에 항목 추가`} onClose={onClose}><div className={styles.dialogBody}><label className="field">옮길 항목 검색<input type="search" value={search} onChange={event => setSearch(event.target.value)} /></label><div className={styles.picker}>{available.filter(entry => entry.name.toLocaleLowerCase().includes(search.toLocaleLowerCase())).map(entry => <label key={entry.key} className={styles.pickRow}><input type="checkbox" checked={checked.includes(entry.key)} onChange={event => setChecked(event.target.checked ? [...checked, entry.key] : checked.filter(key => key !== entry.key))} /><span>{entry.name}<small>{kinds[entry.kind]} · {layout.sections.find(card => card.items.some(item => itemKey(item) === entry.key))?.name}</small></span></label>)}</div>{!available.length && <p className="helper">옮길 항목이 없습니다. 링크나 애드온을 등록하면 배치할 수 있습니다.</p>}<div className={styles.dialogActions}><span /><div><Button onClick={onClose}>취소</Button><Button variant="primary" disabled={!checked.length} onClick={() => onApply(checked)}>{checked.length}개 항목 옮기기</Button></div></div></div></Dialog>
}
