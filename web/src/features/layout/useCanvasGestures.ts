import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import type { HomeLayout } from '../../api/types'
import { clamp, GRID_GAP, GRID_ROW, itemKey, moveInFlow, moveItem, moveSection, orderedSections, positionSections, reorderSection, sectionRows } from './layout'
import type { Gesture } from './HomeCanvas'

export function useCanvasGestures(layout: HomeLayout, commit: (layout: HomeLayout) => void) {
  const [live, setLive] = useState<HomeLayout | null>(null)
  const [active, setActive] = useState<string>()
  const cleanup = useRef<(() => void) | null>(null)
  useEffect(() => () => cleanup.current?.(), [])

  function start(event: PointerEvent<HTMLButtonElement>, gesture: Gesture, id: string, sectionId: string) {
    if (event.button !== 0 || !event.isPrimary || cleanup.current) return
    event.preventDefault()
    const canvas = event.currentTarget.closest<HTMLElement>('[data-layout-canvas]')
    const section = layout.sections.find(section => section.id === sectionId)
    const sourceGrid = canvas?.querySelector<HTMLElement>(`[data-section-grid="${sectionId}"]`)
    if (!canvas || !section || !sourceGrid) return
    const item = section.items.find(item => itemKey(item) === id)
    const itemElement = event.currentTarget.closest<HTMLElement>('[data-layout-item]')
    const sourceRect = sourceGrid.getBoundingClientRect()
    const itemRect = itemElement?.getBoundingClientRect()
    const gridGap = parseFloat(getComputedStyle(sourceGrid).columnGap) || GRID_GAP
    const columnStep = (sourceRect.width + gridGap) / 12
    const outerGap = parseFloat(getComputedStyle(canvas).columnGap) || 24
    const outerStep = (canvas.getBoundingClientRect().width + outerGap) / 12
    const inFlow = getComputedStyle(canvas).display === 'flex'
    const startScrollY = window.scrollY
    const startX = event.clientX, startY = event.clientY, pointerId = event.pointerId
    const offsetX = itemRect ? startX - itemRect.left : 0
    const offsetY = itemRect ? startY - itemRect.top : 0
    let candidate = layout, lastX = startX, lastY = startY, frame = 0, moved = false
    setActive(id)

    function update(x: number, y: number) {
      if (Math.abs(x - startX) + Math.abs(y - startY) < 4 && !moved) return
      moved = true
      if (gesture === 'item-resize' && item) {
        candidate = moveItem(layout, id, sectionId, { width: clamp(item.width + (x - startX) / columnStep, 1, 12 - item.x), height: clamp(item.height + (y - startY) / (GRID_ROW + GRID_GAP), 1, 24) })
      } else if (gesture === 'section-resize') {
        const deltaRows = Math.round((y - startY) / (GRID_ROW + GRID_GAP))
        candidate = moveSection(layout, id, { width: clamp(section!.width + (x - startX) / outerStep, 3, 12 - (section!.x ?? 0)), minRows: deltaRows ? clamp(sectionRows(section!) + deltaRows, 0, 48) : section!.minRows })
      } else if (gesture === 'section-move' && !inFlow) {
        // The outer canvas accepts empty space, independently of card hit targets.
        candidate = moveSection(layout, id, { x: (section!.x ?? 0) + (x - startX) / outerStep, y: (section!.y ?? 0) + (y - startY + window.scrollY - startScrollY) / (GRID_ROW + GRID_GAP) })
      } else {
        const target = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-layout-section]')
        const targetId = target?.dataset.layoutSection
        if (!targetId || !canvas!.contains(target!)) return
        if (gesture === 'section-move') {
          if (targetId === id) return
          candidate = reorderSection(layout, id, orderedSections(positionSections(layout.sections)).findIndex(card => card.id === targetId))
        } else if (item) {
          const grid = target!.querySelector<HTMLElement>('[data-section-grid]')!
          if (!grid) return
          if (getComputedStyle(grid).display === 'flex') {
            const before = [...grid.querySelectorAll<HTMLElement>('[data-layout-item]')].find(card => card.dataset.layoutItem !== id && y < card.getBoundingClientRect().top + card.getBoundingClientRect().height / 2)
            candidate = moveInFlow(layout, id, targetId, before?.dataset.layoutItem)
          } else {
            const rect = grid.getBoundingClientRect()
            const step = (rect.width + gridGap) / 12
            candidate = moveItem(layout, id, targetId, { x: clamp((x - rect.left - offsetX) / step, 0, 12 - item.width), y: clamp((y - rect.top - offsetY) / (GRID_ROW + GRID_GAP), 0, 4096 - item.height) })
          }
        }
      }
      setLive(candidate)
    }

    function remove() {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      window.removeEventListener('pointercancel', onCancel)
      window.removeEventListener('blur', onCancel)
      window.removeEventListener('keydown', onKey)
      cancelAnimationFrame(frame)
      cleanup.current = null
    }
    function finish(cancelled: boolean) {
      remove()
      setLive(null)
      setActive(undefined)
      if (!cancelled && moved) commit(candidate)
    }
    function onMove(move: globalThis.PointerEvent) {
      if (move.pointerId !== pointerId) return
      move.preventDefault()
      lastX = move.clientX; lastY = move.clientY
      update(lastX, lastY)
    }
    function onUp(up: globalThis.PointerEvent) { if (up.pointerId === pointerId) finish(false) }
    function onCancel() { finish(true) }
    function onKey(key: globalThis.KeyboardEvent) { if (key.key === 'Escape') { key.preventDefault(); finish(true) } }
    function scroll() {
      // Keep handles reachable when moving between sections on a long page.
      if (moved && (gesture === 'item-move' || gesture === 'section-move')) {
        const edge = 64
        const delta = lastY < edge ? -12 : lastY > innerHeight - edge ? 12 : 0
        if (delta) {
          const before = window.scrollY
          window.scrollBy(0, delta)
          if (window.scrollY !== before) update(lastX, lastY)
        }
      }
      frame = requestAnimationFrame(scroll)
    }
    cleanup.current = remove
    window.addEventListener('pointermove', onMove, { passive: false })
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onCancel)
    window.addEventListener('blur', onCancel)
    window.addEventListener('keydown', onKey)
    frame = requestAnimationFrame(scroll)
  }

  function keyboard(event: KeyboardEvent<HTMLButtonElement>, gesture: Gesture, id: string, sectionId: string) {
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return
    event.preventDefault()
    if (active) return
    const dx = event.key === 'ArrowLeft' ? -1 : event.key === 'ArrowRight' ? 1 : 0
    const dy = event.key === 'ArrowUp' ? -1 : event.key === 'ArrowDown' ? 1 : 0
    const section = layout.sections.find(section => section.id === sectionId)!
    if (gesture === 'section-move' || gesture === 'section-resize') {
      const resize = gesture === 'section-resize' || event.shiftKey
      if (!resize && getComputedStyle(event.currentTarget.closest<HTMLElement>('[data-layout-canvas]')!).display === 'flex') return commit(reorderSection(layout, id, orderedSections(positionSections(layout.sections)).findIndex(card => card.id === id) + dx + dy))
      return commit(moveSection(layout, id, resize ? { width: clamp(section.width + dx, 3, 12 - (section.x ?? 0)), minRows: dy ? clamp(sectionRows(section) + dy, 0, 48) : section.minRows } : { x: (section.x ?? 0) + dx, y: (section.y ?? 0) + dy }))
    }
    const item = section.items.find(item => itemKey(item) === id)!
    const resize = gesture === 'item-resize' || event.shiftKey
    commit(moveItem(layout, id, sectionId, resize ? { width: clamp(item.width + dx, 1, 12 - item.x), height: clamp(item.height + dy, 1, 24) } : { x: item.x + dx, y: item.y + dy }))
  }
  return { live, active, start, keyboard }
}
