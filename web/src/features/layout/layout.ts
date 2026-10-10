import type { Addon, Dashboard, HomeItem, HomeLayout, HomeSection, Settings } from '../../api/types'

export const GRID_COLUMNS = 12
export const GRID_ROW = 48
export const GRID_GAP = 12
export const MAX_CANVAS_ROWS = 65536
export const NEW_ITEMS_ID = '00000000-0000-4000-8000-000000000001'
export const defaultHomeLayout: HomeLayout = { version: 1, mode: 'auto', sections: [] }

export type CatalogEntry = {
  key: string
  kind: HomeItem['kind']
  refId: string
  name: string
  enabled: boolean
  service?: Dashboard['groups'][number]['services'][number]
  addon?: Addon
}
export const itemKey = (item: Pick<HomeItem, 'kind' | 'refId'>) => `${item.kind}:${item.refId.toLowerCase()}`
export const orderedItems = (items: HomeItem[]) => [...items].sort((a, b) => a.y - b.y || a.x - b.x)
export const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, Math.round(value)))
export const sectionRows = (section: HomeSection) => Math.max(section.minRows, ...section.items.map(item => item.y + item.height), section.items.length ? 1 : 3)
export const sectionHeight = (section: HomeSection) => sectionRows(section) + 1
export type PositionedSection = HomeSection & { x: number; y: number }
export const orderedSections = (sections: PositionedSection[]) => [...sections].sort((a, b) => a.y - b.y || a.x - b.x)

export function catalogFor(dashboard: Dashboard, addons: Addon[], settings: Settings): CatalogEntry[] {
  return [
    ...settings.widgets.order.map(kind => ({ key: `${kind}:`, kind, refId: '', name: kind === 'host' ? '호스트' : 'Docker', enabled: kind === 'host' ? settings.widgets.showHost : settings.widgets.showDocker })),
    ...addons.map(addon => ({ key: `addon:${addon.id}`, kind: 'addon' as const, refId: addon.id, name: addon.manifest.name, enabled: addon.enabled, addon })),
    ...dashboard.groups.flatMap(group => group.services.map(service => ({ key: `link:${service.id}`, kind: 'link' as const, refId: service.id, name: service.name, enabled: !service.hidden, service }))),
  ]
}

export function defaultSize(kind: HomeItem['kind']) {
  return kind === 'link' ? { width: 6, height: 3 } : kind === 'addon' ? { width: 6, height: 6 } : { width: 6, height: kind === 'host' ? 7 : 10 }
}

export function overlaps(a: { x: number; y: number; width: number; height: number }, b: { x: number; y: number; width: number; height: number }) {
  return a.x < b.x + b.width && a.x + a.width > b.x && a.y < b.y + b.height && a.y + a.height > b.y
}

const sectionRect = (section: PositionedSection) => ({ ...section, height: sectionHeight(section) })

// Coordinate-free layouts from earlier versions retain their reading order.
// Explicit positions reserve their rectangles before unpositioned sections fill gaps.
export function positionSections(sections: HomeSection[]): PositionedSection[] {
  const placed = sections.filter((section): section is PositionedSection => section.x != null && section.y != null).map(section => ({ ...section }))
  let cursorX = 0, cursorY = 0
  for (const section of sections.filter(section => section.x == null || section.y == null)) {
    const rows = [...new Set([0, ...placed.map(section => section.y + sectionHeight(section))])].sort((a, b) => a - b)
    const columns = [...new Set([0, ...placed.map(section => section.x + section.width)])].sort((a, b) => a - b)
    let position: PositionedSection | undefined
    for (const y of rows) {
      if (y < cursorY) continue
      for (const x of columns) {
        if (y === cursorY && x < cursorX) continue
        const candidate = { ...section, x, y }
        if (x + section.width <= GRID_COLUMNS && !placed.some(other => overlaps(sectionRect(candidate), sectionRect(other)))) { position = candidate; break }
      }
      if (position) break
    }
    placed.push(position!)
    cursorX = position!.x + position!.width; cursorY = position!.y
  }
  return sections.map(section => placed.find(other => other.id === section.id)!)
}

export function settleSections(sections: HomeSection[], priority?: string): PositionedSection[] {
  const positioned = positionSections(sections)
  const active = positioned.find(section => section.id === priority)
  const order = active ? [active, ...orderedSections(positioned.filter(section => section.id !== priority))] : orderedSections(positioned)
  const placed: PositionedSection[] = []
  for (const source of order) {
    const section = { ...source }
    let collisions = placed.filter(other => overlaps(sectionRect(section), sectionRect(other)))
    while (collisions.length) {
      section.y = Math.max(...collisions.map(other => other.y + sectionHeight(other)))
      collisions = placed.filter(other => overlaps(sectionRect(section), sectionRect(other)))
    }
    placed.push(section)
  }
  return positioned.map(section => placed.find(other => other.id === section.id)!)
}

export function withSections(layout: HomeLayout, sections: HomeSection[], priority?: string): HomeLayout {
  const placed = settleSections(sections, priority)
  if (placed.some(section => section.y + sectionHeight(section) > MAX_CANVAS_ROWS)) return layout
  return { ...layout, mode: 'custom', sections: placed }
}

export function moveSection(layout: HomeLayout, id: string, rectangle: Partial<Pick<HomeSection, 'x' | 'y' | 'width' | 'minRows'>>): HomeLayout {
  const sections = positionSections(layout.sections).map(source => {
    if (source.id !== id) return source
    const section = { ...source, ...rectangle }
    section.width = clamp(section.width, 3, GRID_COLUMNS)
    section.minRows = clamp(section.minRows, 0, 48)
    section.x = clamp(section.x, 0, GRID_COLUMNS - section.width)
    section.y = clamp(section.y, 0, MAX_CANVAS_ROWS - sectionHeight(section))
    return section
  })
  return withSections(layout, sections, id)
}

// The actively edited item has priority. Only colliding cards move down; cards
// in unrelated columns and intentional whitespace retain their coordinates.
export function settleItems(items: HomeItem[], priority?: string): HomeItem[] {
  const active = items.find(item => itemKey(item) === priority)
  const order = active ? [active, ...orderedItems(items.filter(item => itemKey(item) !== priority))] : orderedItems(items)
  const placed: HomeItem[] = []
  for (const source of order) {
    const item = { ...source }
    let collisions = placed.filter(other => overlaps(item, other))
    while (collisions.length) {
      item.y = Math.max(...collisions.map(other => other.y + other.height))
      collisions = placed.filter(other => overlaps(item, other))
    }
    placed.push(item)
  }
  return items.map(item => placed.find(other => itemKey(other) === itemKey(item))!)
}

export function appendItem(items: HomeItem[], source: HomeItem): HomeItem[] {
  // Find the first free rectangle, including gaps above existing cards.
  for (let y = 0; y + source.height <= 4096; y++) {
    for (let x = 0; x + source.width <= GRID_COLUMNS; x++) {
      const item = { ...source, x, y }
      if (!items.some(other => overlaps(item, other))) return [...items, item]
    }
  }
  return items
}

export function flowItems(items: HomeItem[]): HomeItem[] {
  let x = 0, y = 0, rowHeight = 0
  return items.map(source => {
    if (x + source.width > GRID_COLUMNS) { y += rowHeight; x = 0; rowHeight = 0 }
    const item = { ...source, x, y }
    x += item.width
    rowHeight = Math.max(rowHeight, item.height)
    return item
  })
}

export function moveItem(layout: HomeLayout, key: string, sectionId: string, rectangle?: Partial<Pick<HomeItem, 'x' | 'y' | 'width' | 'height'>>): HomeLayout {
  const sourceSection = layout.sections.find(section => section.items.some(item => itemKey(item) === key))
  const source = sourceSection?.items.find(item => itemKey(item) === key)
  if (!source || !layout.sections.some(section => section.id === sectionId)) return layout
  const item = { ...source, ...rectangle }
  item.width = clamp(item.width, 1, GRID_COLUMNS)
  item.height = clamp(item.height, 1, 24)
  item.x = clamp(item.x, 0, GRID_COLUMNS - item.width)
  item.y = clamp(item.y, 0, 4096 - item.height)
  const sections = layout.sections.map(section => {
    let items = section.items.filter(card => itemKey(card) !== key)
    if (section.id === sectionId) {
      items = rectangle ? settleItems([...items, item], key) : appendItem(items, item)
    }
    return section.id === sectionId || section.id === sourceSection!.id ? { ...section, items } : section
  })
  if (sections.some(section => section.items.some(card => card.y + card.height > 4096))) return layout
  return withSections(layout, sections, sectionId)
}

export function moveInFlow(layout: HomeLayout, key: string, sectionId: string, beforeKey?: string): HomeLayout {
  const source = layout.sections.flatMap(section => section.items).find(item => itemKey(item) === key)
  if (!source || !layout.sections.some(section => section.id === sectionId)) return layout
  const sections = layout.sections.map(section => {
    const items = orderedItems(section.items.filter(item => itemKey(item) !== key))
    if (section.id === sectionId) {
      const index = beforeKey ? items.findIndex(item => itemKey(item) === beforeKey) : -1
      items.splice(index < 0 ? items.length : index, 0, source)
      return { ...section, items: flowItems(items) }
    }
    return section.items.some(item => itemKey(item) === key) ? { ...section, items: flowItems(items) } : section
  })
  return sections.some(section => sectionRows(section) > 4096) ? layout : withSections(layout, sections, sectionId)
}

export function reorderSection(layout: HomeLayout, id: string, targetIndex: number): HomeLayout {
  const sections = orderedSections(positionSections(layout.sections))
  const index = sections.findIndex(section => section.id === id)
  if (index < 0) return layout
  const target = sections[clamp(targetIndex, 0, sections.length - 1)]
  if (target.id === id) return layout
  return moveSection(layout, id, { x: target.x, y: target.y + (targetIndex > index ? sectionHeight(target) : 0) })
}

export function autoLayout(dashboard: Dashboard, catalog: CatalogEntry[], settings: Settings): HomeLayout {
  const enabled = catalog.filter(entry => entry.enabled)
  const sections: HomeSection[] = []
  const makeSection = (id: string, name: string, width: number, entries: CatalogEntry[], itemWidth?: number) => {
    if (!entries.length) return
    let items: HomeItem[] = []
    for (const entry of entries) items = appendItem(items, { kind: entry.kind, refId: entry.refId, x: 0, y: 0, ...defaultSize(entry.kind), ...(itemWidth ? { width: itemWidth } : {}) })
    sections.push({ id, name, width, minRows: 0, collapsed: false, items })
  }
  makeSection('00000000-0000-4000-8000-000000000002', '시스템', 12, enabled.filter(entry => entry.kind === 'host' || entry.kind === 'docker'))
  makeSection('00000000-0000-4000-8000-000000000003', '애드온', 12, enabled.filter(entry => entry.kind === 'addon'))
  const width = settings.appearance.layout === 'rows' ? 12 : Math.max(3, Math.floor(12 / (settings.appearance.columns || 2)))
  for (const group of dashboard.groups) makeSection(group.id, group.name, width, enabled.filter(entry => entry.service?.groupId === group.id), width >= 8 ? 4 : width >= 6 ? 6 : 12)
  return { version: 1, mode: 'custom', sections: settleSections(sections) }
}

export function reconcileLayout(layout: HomeLayout, catalog: CatalogEntry[]): HomeLayout {
  const known = new Set(catalog.map(entry => entry.key))
  const seen = new Set<string>()
  const sections = layout.sections.map(section => ({ ...section, items: section.items.filter(item => {
    const key = itemKey(item)
    if (!known.has(key) || seen.has(key)) return false
    seen.add(key)
    return true
  }) }))
  const additions = catalog.filter(entry => entry.enabled && !seen.has(entry.key))
  if (additions.length) {
    let inbox = sections.find(section => section.id === NEW_ITEMS_ID)
    if (!inbox && sections.length >= 60) inbox = sections.at(-1)
    if (!inbox) { inbox = { id: NEW_ITEMS_ID, name: '새 항목', x: 0, y: Math.max(0, ...settleSections(sections).map(section => section.y + sectionHeight(section))), width: 12, minRows: 0, collapsed: false, items: [] }; sections.push(inbox) }
    for (const entry of additions) inbox.items = appendItem(inbox.items, { kind: entry.kind, refId: entry.refId, x: 0, y: 0, ...defaultSize(entry.kind) })
  }
  return { ...layout, sections: settleSections(sections) }
}

export function layoutFor(settings: Settings, dashboard: Dashboard, catalog: CatalogEntry[]) {
  return settings.homeLayout.mode === 'custom' ? reconcileLayout(settings.homeLayout, catalog) : autoLayout(dashboard, catalog, settings)
}
