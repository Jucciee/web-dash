import { expect, test } from '@playwright/test'
import type { Settings, Service, Addon } from '../src/api/types'

test('mixed sections, drag/resize, undo, persistence, search, safe previews and responsive layout', async ({ page }) => {
  test.setTimeout(150_000)
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (message.type() === 'error' && !message.text().includes('Failed to load resource')) errors.push(message.text()) })
  await page.goto('/')
  await expect(page.getByLabel('관리자 아이디')).toBeVisible()
  if (await page.getByRole('heading', { name: '관리자 설정', exact: true }).isVisible()) await page.getByLabel('초기 설정 토큰').fill('playwright-test-only-setup-token-32-characters')
  await page.getByLabel('관리자 아이디').fill('browser-admin')
  await page.getByLabel('비밀번호', { exact: true }).fill('correct horse battery staple')
  const setup = await page.getByRole('button', { name: '관리자 계정 만들기' }).isVisible()
  await page.getByRole('button', { name: setup ? '관리자 계정 만들기' : '로그인', exact: true }).click()
  await expect(page.locator('.home-dashboard')).toBeVisible()
  const get = async (url: string) => {
    // The Windows test environment can briefly reject a loopback connection.
    // Retry GETs only, never writes or a failed HTTP response.
    for (let attempt = 0; ; attempt++) {
      try { return await page.request.get(url) }
      catch (error) { if (attempt >= 2 || !/ETIMEDOUT|ECONNRESET/.test(String(error))) throw error }
    }
  }
  const original: Settings = (await (await get('/api/v1/settings')).json()).data
  const csrf = (await (await get('/api/v1/auth/csrf')).json()).data.csrfToken
  const mutate = async (method: string, path: string, data?: unknown, revision?: number) => {
    const result = await page.request.fetch(`/api/v1${path}`, { method, headers: { 'X-CSRF-Token': csrf, Origin: 'http://127.0.0.1:18080', ...(revision == null ? {} : { 'If-Match': `"${revision}"` }) }, ...(data == null ? {} : { data }) })
    expect(result.ok(), await result.text()).toBe(true)
    return result.status() === 204 ? null : (await result.json()).data
  }
  const settings = async (): Promise<Settings> => (await (await get('/api/v1/settings')).json()).data
  const reads = async () => (await (await get('http://127.0.0.1:18082/__addon_checks')).json()).checks
  await mutate('PATCH', '/settings', { homeLayout: { version: 1, mode: 'auto', sections: [] }, customCss: '', widgets: { showHost: true, showDocker: true, refreshSeconds: 0, order: ['host', 'docker'] } }, original.revision)
  const boardId = (await (await get('/api/v1/dashboards')).json()).data[0].id
  const board = (await (await get(`/api/v1/dashboards/${boardId}`)).json()).data
  const groupId = board.groups[0].id
  const link: Service = await mutate('POST', '/services', { groupId, name: '배치 테스트 링크', url: 'http://127.0.0.1:18082/ok', iconKey: 'photos', iconMode: 'builtin', description: '섹션에 묶는 사진 링크', hidden: false, webCheckEnabled: false })
  const addon: Addon = await mutate('POST', '/addons', { manifest: { schemaVersion: 1, name: '사진 API', icon: 'photos', display: 'stats', refreshSeconds: 0, inputs: [{ key: 'apiKey', label: 'API 키', type: 'secret', required: true, default: '' }, { key: 'cookie', label: '쿠키', type: 'secret', required: true, default: '' }], request: { method: 'GET', url: 'http://127.0.0.1:18082/addon/status', headers: [{ name: 'Authorization', value: 'Bearer {{apiKey}}' }, { name: 'Cookie', value: '{{cookie}}' }], body: '' }, fields: [{ label: '부하', path: '/metrics/load', format: 'percent', unit: '', scale: 100, decimals: 1, valueMap: {} }] }, values: {}, secrets: { apiKey: 'e2e-addon-token', cookie: 'session=e2e-addon-cookie' }, enabled: true })
  let additional: Service | undefined
  try {
    const before = await reads()
    await page.goto('/settings/layout')
    await page.reload()
    await expect(page.getByRole('heading', { name: '배치 편집', exact: true })).toBeVisible()
    await expect(page.locator(`[data-layout-item="addon:${addon.id}"]`)).toBeVisible()
    expect(await reads()).toBe(before)
    await page.getByRole('button', { name: '섹션 추가', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByLabel('섹션 이름').fill('사진과 서버')
    await dialog.getByLabel('섹션 너비 (칸)', { exact: true }).fill('12')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    let mixed = page.getByRole('region', { name: '사진과 서버 그룹', exact: true })
    await mixed.getByRole('button', { name: '사진과 서버에 항목 추가' }).click()
    for (const name of [/^호스트/, /^배치 테스트 링크/, /^사진 API/, /^Docker/]) await dialog.getByRole('checkbox', { name }).check()
    await dialog.getByRole('button', { name: '4개 항목 옮기기' }).click()
    await expect(mixed.locator('[data-layout-item]')).toHaveCount(4)
    // Numeric controls work for every card type and select its destination.
    await mixed.getByRole('button', { name: '사진 API 배치 설정', exact: true }).click()
    await dialog.getByLabel('항목 너비 (칸)').fill('4')
    await dialog.getByLabel('항목 높이 (칸)').fill('5')
    await dialog.getByLabel('가로 위치 (칸)').fill('1')
    await dialog.getByLabel('세로 위치 (칸)').fill('1')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    const tile = mixed.locator(`[data-layout-item="addon:${addon.id}"]`)
    await expect(tile).toHaveAttribute('data-width', '4')
    // Keyboard movement/resizing has the same grid bounds and collision rules.
    await mixed.getByRole('button', { name: '사진 API 이동', exact: true }).press('ArrowRight')
    await expect(tile).toHaveAttribute('data-x', '1')
    await mixed.getByRole('button', { name: '사진 API 크기 조절', exact: true }).press('ArrowDown')
    await expect(tile).toHaveAttribute('data-height', '6')
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    await expect(tile).toHaveAttribute('data-height', '5')
    await page.getByRole('button', { name: '배치 다시 실행', exact: true }).click()
    await expect(tile).toHaveAttribute('data-height', '6')
    // A real pointer gesture changes both dimensions; cancel restores the draft.
    const resize = mixed.getByRole('button', { name: '사진 API 크기 조절', exact: true })
    await resize.scrollIntoViewIfNeeded()
    let rect = (await resize.boundingBox())!
    await page.mouse.move(rect.x + 22, rect.y + 22); await page.mouse.down()
    await page.mouse.move(rect.x + 160, rect.y + 100, { steps: 8 }); await page.mouse.up()
    expect(Number(await tile.getAttribute('data-width'))).toBeGreaterThan(4)
    expect(Number(await tile.getAttribute('data-height'))).toBeGreaterThan(6)
    const resizedHeight = await tile.getAttribute('data-height')
    await resize.scrollIntoViewIfNeeded(); rect = (await resize.boundingBox())!
    await page.mouse.move(rect.x + 22, rect.y + 22); await page.mouse.down()
    await page.mouse.move(rect.x + 22, rect.y + 142, { steps: 4 })
    await page.keyboard.press('Escape'); await page.mouse.up()
    await expect(tile).toHaveAttribute('data-height', resizedHeight!)
    // Touch input uses the same handles without scrolling the page instead.
    await resize.scrollIntoViewIfNeeded(); rect = (await resize.boundingBox())!
    const touch = await page.context().newCDPSession(page)
    await touch.send('Emulation.setTouchEmulationEnabled', { enabled: true })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: rect.x + 22, y: rect.y + 22 }] })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: rect.x + 22, y: rect.y + 102 }] })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    expect(Number(await tile.getAttribute('data-height'))).toBeGreaterThan(Number(resizedHeight))
    await touch.send('Emulation.setTouchEmulationEnabled', { enabled: false })
    await touch.detach()
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    await expect(tile).toHaveAttribute('data-height', resizedHeight!)
    await mixed.getByRole('button', { name: '사진 API 배치 설정', exact: true }).click()
    await dialog.getByLabel('항목 너비 (칸)').fill('1')
    await dialog.getByLabel('항목 높이 (칸)').fill('1')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    // Drag a link into another section, including scrolling while holding it.
    await page.getByRole('button', { name: '섹션 추가', exact: true }).click()
    await dialog.getByLabel('섹션 이름').fill('바로가기')
    await dialog.getByLabel('섹션 너비 (칸)').fill('12')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    const destination = page.getByRole('region', { name: '바로가기 그룹', exact: true })
    const drag = mixed.getByRole('button', { name: '배치 테스트 링크 이동', exact: true })
    await drag.scrollIntoViewIfNeeded(); rect = (await drag.boundingBox())!
    await page.mouse.move(rect.x + 16, rect.y + 22); await page.mouse.down()
    await destination.scrollIntoViewIfNeeded()
    const drop = (await destination.locator('[data-section-grid]').boundingBox())!
    await page.mouse.move(drop.x + 60, drop.y + 60, { steps: 12 }); await page.mouse.up()
    await expect(destination.locator(`[data-layout-item="link:${link.id}"]`)).toHaveCount(1)
    await expect(mixed.locator('[data-layout-item]')).toHaveCount(3)
    await destination.getByRole('button', { name: '바로가기 섹션 이동' }).press('ArrowUp')
    await page.getByRole('button', { name: '배치 저장', exact: true }).click()
    await expect(page.getByText('배치를 저장했습니다.')).toBeVisible()
    const persisted = await settings()
    expect(persisted.homeLayout.mode).toBe('custom')
    expect(persisted.homeLayout.sections.find(section => section.name === '사진과 서버')!.items.map(item => item.kind).sort()).toEqual(['addon', 'docker', 'host'])
    await page.reload()
    mixed = page.getByRole('region', { name: '사진과 서버 그룹', exact: true })
    await expect(mixed.locator(`[data-layout-item="addon:${addon.id}"]`)).toHaveAttribute('data-height', resizedHeight!)
    // Cancel leaves persisted settings unchanged, and stale revisions are rejected.
    await mixed.getByRole('button', { name: '사진 API 이동', exact: true }).press('ArrowDown')
    await page.getByRole('button', { name: '변경 취소', exact: true }).click()
    expect((await settings()).homeLayout).toEqual(persisted.homeLayout)
    await mixed.getByRole('button', { name: '사진 API 이동', exact: true }).press('ArrowDown')
    await mutate('PATCH', '/settings', { subtitle: '동시 설정 변경' }, persisted.revision)
    await page.getByRole('button', { name: '배치 저장', exact: true }).click()
    await expect(page.getByRole('alert')).toContainText('다른 화면에서 수정된 항목')
    await page.setViewportSize({ width: 320, height: 1000 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.getByRole('button', { name: '최신 배치 불러오기', exact: true }).click()
    await page.setViewportSize({ width: 1440, height: 1000 })
    await expect(page.getByRole('alert')).toHaveCount(0)
    // Home shows live data in the saved shared sections, with no duplicate cards.
    await page.goto('/')
    mixed = page.getByRole('region', { name: '사진과 서버 그룹', exact: true })
    await expect(mixed.getByRole('region', { name: '사진 API 애드온' })).toContainText('25.0%')
    await expect(mixed.locator('.host-widget')).toHaveCount(1)
    await expect(mixed.locator('.docker-widget')).toHaveCount(1)
    await expect(page.locator(`[data-layout-item="link:${link.id}"]`)).toHaveCount(1)
    await page.getByLabel('링크 검색').fill('사진 API')
    await expect(page.locator('[data-layout-item]')).toHaveCount(1)
    await page.getByLabel('링크 검색').fill('')
    // Newly registered links appear in New items even before editing again.
    additional = await mutate('POST', '/services', { groupId, name: '자동 등록 링크', url: 'http://127.0.0.1:18082/new', iconKey: 'server', iconMode: 'builtin', hidden: false, webCheckEnabled: false })
    await page.reload()
    await expect(page.getByRole('region', { name: '새 항목 그룹', exact: true }).getByRole('link', { name: /자동 등록 링크/ })).toBeVisible()
    for (const width of [320, 375, 414, 768, 1440]) {
      await page.setViewportSize({ width, height: 1000 })
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await expect(page.getByRole('link', { name: /배치 테스트 링크/ })).toBeVisible()
      await page.screenshot({ path: `test-results/layout-home-${width}.png`, fullPage: true })
    }
    await page.screenshot({ path: 'test-results/layout-home-desktop.png', fullPage: true })
    const previewReads = await reads()
    await page.goto('/settings')
    await expect(page.getByRole('complementary', { name: '홈 화면 미리보기' }).locator('.addon-metric')).toContainText('—')
    expect(await reads()).toBe(previewReads)
    await page.goto('/settings/layout')
    for (const width of [320, 375, 414, 768, 1440]) {
      await page.setViewportSize({ width, height: 1000 })
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await page.getByRole('button', { name: '사진 API 배치 설정', exact: true }).click()
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await page.getByRole('button', { name: '닫기', exact: true }).click()
      await page.screenshot({ path: `test-results/layout-editor-${width}.png`, fullPage: true })
    }
    await page.screenshot({ path: 'test-results/layout-editor-desktop.png', fullPage: true })
    // Deleting a mixed section moves its cards into the chosen destination.
    await page.getByRole('button', { name: '사진과 서버 섹션 설정' }).click()
    await dialog.getByLabel('섹션 삭제 시 항목 이동').selectOption({ label: '바로가기' })
    await dialog.getByRole('button', { name: '섹션 삭제', exact: true }).click()
    await expect(page.getByRole('region', { name: '사진과 서버 그룹', exact: true })).toHaveCount(0)
    await expect(page.getByRole('region', { name: '바로가기 그룹' }).locator('[data-layout-item]')).toHaveCount(4)
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    await expect(page.getByRole('region', { name: '사진과 서버 그룹' }).locator('[data-layout-item]')).toHaveCount(3)
    await page.getByRole('button', { name: '자동 배치', exact: true }).click()
    await page.getByRole('button', { name: '배치 저장', exact: true }).click()
    await expect(page.getByText('배치를 저장했습니다.')).toBeVisible()
    expect((await settings()).homeLayout.mode).toBe('auto')
    expect(errors).toEqual([])
  } finally {
    await mutate('DELETE', `/services/${link.id}`, undefined, link.revision)
    if (additional) await mutate('DELETE', `/services/${additional.id}`, undefined, additional.revision)
    await mutate('DELETE', `/addons/${addon.id}`, undefined, addon.revision)
    const latest = await settings()
    const { revision: _revision, ...body } = original
    await mutate('PATCH', '/settings', body, latest.revision)
  }
})
