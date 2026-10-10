import { useLayoutEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react'
import { Box, ChevronDown, ChevronRight, Grip, GripVertical, MoveDiagonal2, Plus, Server, Settings2 } from 'lucide-react'
import type { HomeItem, HomeLayout, HomeSection, Settings, WebStatus } from '../../api/types'
import { Button } from '../../components/Button'
import { AddonValues, AddonWidget } from '../addons/AddonWidgets'
import { ServiceCard } from '../dashboard/ServiceCard'
import { ServiceIcon } from '../dashboard/ServiceIcon'
import { DockerPanel, HostPanel } from '../monitoring/MonitoringPanels'
import { flowItems, itemKey, orderedItems, orderedSections, sectionRows, settleSections, type CatalogEntry, type PositionedSection } from './layout'
import styles from './HomeLayout.module.css'

export type Gesture = 'item-move' | 'item-resize' | 'section-move' | 'section-resize'
export type CanvasEditor = {
  busy: boolean
  active?: string
  selectItem: (key: string) => void
  selectSection: (id: string) => void
  addItems: (id: string) => void
  start: (event: PointerEvent<HTMLButtonElement>, gesture: Gesture, id: string, sectionId: string) => void
  keyboard: (event: KeyboardEvent<HTMLButtonElement>, gesture: Gesture, id: string, sectionId: string) => void
}

function GridBox({ item, children, editor, name, sectionId }: { item: HomeItem; children: ReactNode; editor?: CanvasEditor; name: string; sectionId: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const key = itemKey(item)
  useLayoutEffect(() => {
    const node = ref.current
    if (!node) return
    node.style.setProperty('--item-x', String(item.x + 1))
    node.style.setProperty('--item-y', String(item.y + 1))
    node.style.setProperty('--item-width', String(item.width))
    node.style.setProperty('--item-height', String(item.height))
  }, [item.x, item.y, item.width, item.height])
  return <div ref={ref} className={`home-layout-item ${styles.item}`} data-layout-item={key} data-kind={item.kind} data-x={item.x} data-y={item.y} data-width={item.width} data-height={item.height} data-active={editor?.active === key || undefined}>
    {editor && <div className={styles.itemTools}>
      <Button variant="ghost" className={styles.moveHandle} aria-label={`${name} 이동`} title="드래그 또는 방향키로 이동" disabled={editor.busy} onPointerDown={event => editor.start(event, 'item-move', key, sectionId)} onKeyDown={event => editor.keyboard(event, 'item-move', key, sectionId)}><GripVertical size={16} /><span>{name}</span></Button>
      <span className={styles.sizeHint} aria-hidden="true">{item.width} × {item.height}</span>
      <Button variant="ghost" className="icon-button" aria-label={`${name} 배치 설정`} disabled={editor.busy} onClick={() => editor.selectItem(key)}><Settings2 size={15} /></Button>
    </div>}
    <div className={styles.itemContent} inert={!!editor}>{children}</div>
    {editor && <Button variant="ghost" className={styles.resizeHandle} aria-label={`${name} 크기 조절`} title="드래그 또는 방향키로 크기 조절" disabled={editor.busy} onPointerDown={event => editor.start(event, 'item-resize', key, sectionId)} onKeyDown={event => editor.keyboard(event, 'item-resize', key, sectionId)}><MoveDiagonal2 size={17} /></Button>}
  </div>
}

function CardContent({ entry, settings, statuses, preview }: { entry: CatalogEntry; settings: Settings; statuses?: WebStatus[]; preview?: boolean }) {
  if (!entry.enabled) return <div className={styles.placeholder}><strong>{entry.name}</strong><p className="helper">설정에서 숨기거나 비활성화한 항목입니다. 배치 위치는 유지됩니다.</p></div>
  if (entry.service) return <ServiceCard service={entry.service} status={statuses?.find(status => status.serviceId === entry.refId)} appearance={settings.appearance} manage={false} fit first last busy={false} onEdit={() => {}} onDelete={() => {}} onMove={() => {}} />
  if (entry.addon) return preview ? <section className={`addon-widget ${styles.placeholder}`} aria-label={`${entry.name} 애드온`}><h2><ServiceIcon iconKey={entry.addon.manifest.icon} size={18} />{entry.name}</h2><AddonValues manifest={entry.addon.manifest} /></section> : <AddonWidget addon={entry.addon} />
  if (preview) return <section className={`${entry.kind}-widget ${styles.placeholder}`} aria-label={`${entry.name} 상태`}><h2>{entry.kind === 'host' ? <Server size={18} /> : <Box size={18} />}{entry.name}</h2><div className={styles.previewMetrics}>{(entry.kind === 'host' ? ['CPU', '메모리', '디스크'] : ['실행 중', '중지·대기']).map(label => <div key={label}><span>{label}</span><strong>—</strong></div>)}</div><p className="helper">상태 값은 홈에서 확인할 수 있습니다.</p></section>
  return entry.kind === 'host' ? <HostPanel interval={settings.widgets.refreshSeconds} /> : <DockerPanel interval={settings.widgets.refreshSeconds} />
}

function matchingItems(section: HomeSection, entries: Map<string, CatalogEntry>, search: string, editing: boolean) {
  return section.items.filter(item => {
    const entry = entries.get(itemKey(item))
    if (!entry || (!editing && !entry.enabled)) return false
    return !search || [section.name, entry.name, entry.service?.url ?? '', entry.service?.description ?? ''].some(value => value.toLocaleLowerCase().includes(search))
  })
}

function CanvasSection({ section, entries, settings, statuses, preview, search, editor }: { section: PositionedSection; entries: Map<string, CatalogEntry>; settings: Settings; statuses?: WebStatus[]; preview?: boolean; search: string; editor?: CanvasEditor }) {
  const ref = useRef<HTMLElement>(null)
  const gridRef = useRef<HTMLDivElement>(null)
  const [toggled, setToggled] = useState<boolean | null>(null)
  const collapsed = !editor && !search && (toggled ?? section.collapsed)
  const matching = matchingItems(section, entries, search, !!editor)
  const items = search ? flowItems(orderedItems(matching)) : orderedItems(matching)
  const rows = sectionRows(section)
  useLayoutEffect(() => {
    ref.current?.style.setProperty('--section-width', String(section.width))
    ref.current?.style.setProperty('--section-x', String(section.x + 1))
    ref.current?.style.setProperty('--section-y', String(section.y + 1))
    ref.current?.style.setProperty('--section-height', String(collapsed ? 1 : rows + 1))
    gridRef.current?.style.setProperty('--section-rows', String(rows))
  }, [section.width, section.x, section.y, rows, collapsed])
  if (!editor && !items.length) return null
  return <section ref={ref} className={`service-group home-layout-section ${styles.section}`} aria-label={`${section.name} 그룹`} data-layout-section={section.id} data-x={section.x} data-y={section.y} data-width={section.width} data-height={rows + 1} data-collapsed={collapsed} data-active={editor?.active === section.id || undefined}>
    <div className={styles.sectionHead}>
      {editor && <Button variant="ghost" className={`icon-button ${styles.moveHandle}`} aria-label={`${section.name} 섹션 이동`} title="빈 공간으로 드래그하거나 방향키로 이동" disabled={editor.busy} onPointerDown={event => editor.start(event, 'section-move', section.id, section.id)} onKeyDown={event => editor.keyboard(event, 'section-move', section.id, section.id)}><Grip size={17} /></Button>}
      <h2 className="group-title">{!editor && (settings.appearance.collapsibleGroups || section.collapsed) ? <button className={styles.collapse} aria-expanded={!collapsed} aria-controls={`home-section-${section.id}`} onClick={() => setToggled(!collapsed)}>{collapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}<span>{section.name}</span>{settings.appearance.showGroupCounts && <small>{items.length}</small>}</button> : <><span>{section.name}</span>{settings.appearance.showGroupCounts && <small>{items.length}</small>}</>}</h2>
      {editor && <div className={styles.sectionTools}><Button variant="ghost" className="icon-button" aria-label={`${section.name}에 항목 추가`} disabled={editor.busy} onClick={() => editor.addItems(section.id)}><Plus size={17} /></Button><Button variant="ghost" className="icon-button" aria-label={`${section.name} 섹션 설정`} disabled={editor.busy} onClick={() => editor.selectSection(section.id)}><Settings2 size={17} /></Button><Button variant="ghost" className={styles.sectionResize} aria-label={`${section.name} 섹션 크기 조절`} title="드래그 또는 방향키로 너비와 최소 높이 조절" disabled={editor.busy} onPointerDown={event => editor.start(event, 'section-resize', section.id, section.id)} onKeyDown={event => editor.keyboard(event, 'section-resize', section.id, section.id)}><MoveDiagonal2 size={17} /></Button></div>}
    </div>
    {!collapsed && <div ref={gridRef} id={`home-section-${section.id}`} className={styles.grid} data-section-grid={section.id}>{items.map(item => {
      const entry = entries.get(itemKey(item))!
      return <GridBox key={entry.key} item={item} name={entry.name} sectionId={section.id} editor={editor}><CardContent entry={entry} settings={settings} statuses={statuses} preview={preview} /></GridBox>
    })}{editor && !items.length && <button className={styles.emptySection} onClick={() => editor.addItems(section.id)} disabled={editor.busy}><Plus size={20} />항목 추가</button>}</div>}
  </section>
}

export function HomeCanvas({ layout, catalog, settings, statuses, search = '', preview = false, editor }: { layout: HomeLayout; catalog: CatalogEntry[]; settings: Settings; statuses?: WebStatus[]; search?: string; preview?: boolean; editor?: CanvasEditor }) {
  const ref = useRef<HTMLDivElement>(null)
  const [flow, setFlow] = useState(true)
  useLayoutEffect(() => {
    const node = ref.current!
    const observer = new ResizeObserver(([entry]) => setFlow(entry.contentRect.width <= 900))
    observer.observe(node)
    setFlow(node.clientWidth <= 900)
    return () => observer.disconnect()
  }, [])
  const entries = new Map(catalog.map(entry => [entry.key, entry]))
  const query = search.trim().toLocaleLowerCase()
  const sections = orderedSections(settleSections(query ? layout.sections.map(section => ({ ...section, x: undefined, y: undefined, minRows: 0, items: flowItems(orderedItems(matchingItems(section, entries, query, !!editor))) })).filter(section => section.items.length) : layout.sections))
  return <div ref={ref} className={`home-layout ${styles.canvas}`} data-layout-canvas data-flow={flow} data-editing={!!editor} aria-label={editor ? '홈 배치 편집 캔버스' : '홈 섹션'}>{sections.map(section => <CanvasSection key={section.id} section={section} entries={entries} settings={settings} statuses={statuses} search={query} preview={preview} editor={editor} />)}</div>
}
