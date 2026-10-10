import assert from 'node:assert/strict'
import { test } from 'node:test'
import { appendItem, flowItems, itemKey, moveInFlow, moveItem, moveSection, orderedSections, overlaps, positionSections, reconcileLayout, reorderSection, sectionHeight, settleItems, settleSections } from '../src/features/layout/layout.ts'

const card = (refId, x = 0, y = 0, width = 6, height = 3) => ({ kind: 'link', refId, x, y, width, height })
const section = (id, items) => ({ id, name: id, width: 6, minRows: 0, collapsed: false, items })
const layout = sections => ({ version: 1, mode: 'custom', sections })
const disjoint = items => {
  for (const [index, item] of items.entries()) {
    assert.ok(item.x >= 0 && item.x + item.width <= 12 && item.y >= 0 && item.y + item.height <= 4096)
    for (const other of items.slice(index + 1)) assert.equal(overlaps(item, other), false)
  }
}

test('resizing shifts colliding cards while preserving cards in other columns and intentional gaps', () => {
  const items = [card('a', 0, 0, 6, 6), card('b', 0, 3), card('c', 0, 6), card('d', 6, 1), card('e', 6, 15)]
  const snapshot = structuredClone(items)
  const result = settleItems(items, 'link:a')
  assert.equal(result[0].y, 0)
  assert.equal(result[1].y, 6)
  assert.equal(result[2].y, 9)
  assert.deepEqual(result[3], items[3]); assert.deepEqual(result[4], items[4])
  assert.deepEqual(items, snapshot); disjoint(result)
})

test('cross-section move preserves one reference and clamps coordinates after resizing', () => {
  const original = layout([section('first', [card('a')]), section('second', [card('b')])])
  const moved = moveItem(original, 'link:a', 'second', { x: 11, y: -1, width: 8, height: 4 })
  assert.equal(moved.sections[0].items.length, 0)
  assert.equal(moved.sections[1].items.length, 2)
  assert.equal(moved.sections[1].items.find(item => item.refId === 'a').x, 4)
  assert.equal(original.sections[0].items.length, 1)
  disjoint(moved.sections[1].items)
})

test('adding fills available space, mobile reordering follows the requested reading order', () => {
  assert.equal(appendItem([card('a', 6, 0)], card('b'))[1].x, 0)
  const original = layout([section('s', flowItems([card('a'), card('b'), card('c', 0, 0, 12)]))])
  const moved = moveInFlow(original, 'link:c', 's', 'link:a')
  assert.deepEqual([...moved.sections[0].items].sort((a, b) => a.y - b.y || a.x - b.x).map(item => item.refId), ['c', 'a', 'b'])
  disjoint(moved.sections[0].items)
})

test('new cards appear automatically, disabled cards retain positions, deleted cards disappear without mutating saved layout', () => {
  const original = layout([section('s', [card('deleted'), card('disabled', 6)])])
  const snapshot = structuredClone(original)
  const catalog = [{ key: 'link:disabled', kind: 'link', refId: 'disabled', enabled: false }, { key: 'addon:new', kind: 'addon', refId: 'new', enabled: true }, { key: 'link:hidden', kind: 'link', refId: 'hidden', enabled: false }]
  const result = reconcileLayout(original, catalog)
  assert.deepEqual(result.sections[0].items, [card('disabled', 6)])
  assert.deepEqual(result.sections[1].items.map(itemKey), ['addon:new'])
  assert.deepEqual(original, snapshot)
})

test('section reordering and repeated moves/resizes preserve unique, non-overlapping cards', () => {
  let current = layout([section('first', flowItems(Array.from({ length: 10 }, (_, i) => card(String(i))))), section('second', [])])
  current = reorderSection(current, 'second', 0)
  assert.equal(orderedSections(positionSections(current.sections))[0].id, 'second')
  for (let i = 0; i < 100; i++) {
    current = moveItem(current, `link:${i % 10}`, i % 3 ? 'first' : 'second', { x: i % 12, y: i % 8, width: 1 + i % 12, height: 1 + i % 9 })
    assert.equal(new Set(current.sections.flatMap(section => section.items).map(itemKey)).size, 10)
    for (const section of current.sections) disjoint(section.items)
  }
})

const disjointSections = sections => {
  for (const [index, section] of sections.entries()) {
    assert.ok(section.x >= 0 && section.x + section.width <= 12 && section.y >= 0 && section.y + sectionHeight(section) <= 65536)
    for (const other of sections.slice(index + 1)) assert.equal(overlaps({ ...section, height: sectionHeight(section) }, { ...other, height: sectionHeight(other) }), false)
  }
}

test('legacy sections fill free rectangles without replacing explicit positions', () => {
  const original = [section('old', [card('a')]), { ...section('fixed', [card('b')]), x: 6, y: 0 }, section('next', [card('c')])]
  const snapshot = structuredClone(original)
  const placed = settleSections(original)
  assert.deepEqual([placed[0].x, placed[0].y], [0, 0])
  assert.deepEqual([placed[1].x, placed[1].y], [6, 0])
  assert.deepEqual([placed[2].x, placed[2].y], [0, 4])
  assert.deepEqual(original, snapshot); disjointSections(placed)
})

test('legacy reading order survives different widths and new cards leave intentional canvas whitespace intact', () => {
  const old = [{ ...section('wide', [card('a', 0, 0, 12, 12)]), width: 8 }, { ...section('next', [card('b')]), width: 8 }, { ...section('last', [card('c')]), width: 4 }]
  const placed = settleSections(old)
  assert.deepEqual(orderedSections(placed).map(section => section.id), ['wide', 'next', 'last'])
  assert.deepEqual([placed[2].x, placed[2].y], [8, placed[1].y])
  const original = layout([{ ...section('fixed', [card('a')]), x: 3, y: 10 }])
  const catalog = [{ key: 'link:a', kind: 'link', refId: 'a', enabled: true }, { key: 'link:new', kind: 'link', refId: 'new', enabled: true }]
  const result = reconcileLayout(original, catalog)
  assert.deepEqual(result.sections[0], original.sections[0])
  assert.ok(result.sections[1].y >= 14)
  disjointSections(result.sections)
})

test('sections move to blank space, collision resolution preserves other columns and intentional gaps', () => {
  const original = layout([
    { ...section('a', [card('a')]), x: 0, y: 0 },
    { ...section('b', [card('b')]), x: 0, y: 4 },
    { ...section('c', [card('c')]), x: 6, y: 2 },
    { ...section('d', [card('d')]), x: 0, y: 20 },
  ])
  const snapshot = structuredClone(original)
  const moved = moveSection(original, 'a', { x: 0, y: 4 })
  assert.equal(moved.sections[0].y, 4)
  assert.equal(moved.sections[1].y, 8)
  assert.deepEqual(moved.sections[2], original.sections[2])
  assert.deepEqual(moved.sections[3], original.sections[3])
  const free = moveSection(moved, 'a', { x: 6, y: 12 })
  assert.deepEqual([free.sections[0].x, free.sections[0].y], [6, 12])
  assert.deepEqual(original, snapshot); disjointSections(free.sections)
})

test('growing a card also reserves space for its section and resizing a section respects the canvas bounds', () => {
  let current = layout([{ ...section('a', [card('a')]), x: 0, y: 0 }, { ...section('b', [card('b')]), x: 0, y: 4 }, { ...section('c', [card('c')]), x: 6, y: 0 }])
  current = moveItem(current, 'link:a', 'a', { height: 10 })
  assert.equal(current.sections[1].y, 11)
  assert.equal(current.sections[2].y, 0)
  current = moveSection(current, 'a', { x: 11, width: 8, minRows: 20 })
  assert.equal(current.sections[0].x, 4)
  assert.equal(current.sections[0].width, 8)
  assert.equal(current.sections[0].minRows, 20)
  disjointSections(current.sections)
  const before = structuredClone(current)
  // Coordinates near the last row are clamped without changing the section's cards.
  current = moveSection(current, 'a', { width: 12, y: 65535 })
  disjointSections(current.sections)
  assert.deepEqual(before.sections[0].items, current.sections[0].items)
})
