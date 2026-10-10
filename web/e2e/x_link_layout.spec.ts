import { expect, test, type Locator } from '@playwright/test'
import type { HomeLayout, Service, Settings } from '../src/api/types'

test('small links keep their name and sections retain freely chosen canvas coordinates', async ({ page }) => {
  test.setTimeout(150_000)
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  await page.setViewportSize({ width: 1440, height: 1200 })
  await page.goto('/')
  await expect(page.getByLabel('관리자 아이디')).toBeVisible()
  const setup = await page.getByRole('heading', { name: '관리자 설정', exact: true }).isVisible()
  if (setup) await page.getByLabel('초기 설정 토큰').fill('playwright-test-only-setup-token-32-characters')
  await page.getByLabel('관리자 아이디').fill('browser-admin')
  await page.getByLabel('비밀번호', { exact: true }).fill('correct horse battery staple')
  await page.getByRole('button', { name: setup ? '관리자 계정 만들기' : '로그인', exact: true }).click()
  await expect(page.locator('.home-dashboard')).toBeVisible()
  const get = async (url: string) => {
    for (let attempt = 0; ; attempt++) {
      try { return await page.request.get(url) }
      catch (error) { if (attempt >= 2 || !/ETIMEDOUT|ECONNRESET/.test(String(error))) throw error }
    }
  }
  const settings = async (): Promise<Settings> => (await (await get('/api/v1/settings')).json()).data
  const original = await settings()
  const csrf = (await (await get('/api/v1/auth/csrf')).json()).data.csrfToken
  const mutate = async (method: string, path: string, data?: unknown, revision?: number) => {
    const response = await page.request.fetch(`/api/v1${path}`, { method, headers: { 'X-CSRF-Token': csrf, Origin: 'http://127.0.0.1:18080', ...(revision == null ? {} : { 'If-Match': `"${revision}"` }) }, ...(data == null ? {} : { data }) })
    expect(response.ok(), await response.text()).toBe(true)
    return response.status() === 204 ? null : (await response.json()).data
  }
  const boardId = (await (await get('/api/v1/dashboards')).json()).data[0].id
  const groupId = (await (await get(`/api/v1/dashboards/${boardId}`)).json()).data.groups[0].id
  const fixtures: Service[] = []
  const checkTitle = async (card: Locator, context = 'link card') => {
    await expect(card.locator('.service-title')).toBeVisible()
    const result = await card.locator('.service-title').evaluate(title => {
      const rect = title.getBoundingClientRect()
      const ancestors: Element[] = []
      for (let node = title.parentElement; node; node = node.parentElement) {
        ancestors.push(node)
        if (node.classList.contains('service-card')) break
      }
      return { height: rect.height, contained: ancestors.every(node => {
        const bounds = node.getBoundingClientRect()
        return rect.top >= bounds.top - 1 && rect.bottom <= bounds.bottom + 1
      }) }
    })
    expect(result.height).toBeGreaterThan(15)
    expect(result.contained, `${context}: the name must stay inside every clipped ancestor`).toBe(true)
  }
  try {
    for (const [name, description] of [['사진 보관함', '가족의 사진과 동영상을 정리하고 백업하는 링크입니다. '.repeat(12)], ['보조 링크', '보조 링크 설명']]) {
      fixtures.push(await mutate('POST', '/services', { groupId, name, description, url: 'http://127.0.0.1:18082/ok', iconKey: 'photos', iconMode: 'builtin', hidden: false, webCheckEnabled: false }))
    }
    const makeLayout = (height: number): HomeLayout => ({ version: 1, mode: 'custom', sections: [
      { id: '40280855-3d1f-47a5-ab80-3a9a2e814883', name: '작은 링크', x: 0, y: 0, width: 6, minRows: 0, collapsed: false, items: [{ kind: 'link', refId: fixtures[0].id, x: 0, y: 0, width: 12, height }] },
      { id: '90ee8c93-d690-4ca4-a738-1e7a1117da97', name: '보조', x: 6, y: 4, width: 6, minRows: 0, collapsed: false, items: [{ kind: 'link', refId: fixtures[1].id, x: 0, y: 0, width: 12, height: 3 }] },
    ] })
    const appearance = { ...original.appearance, width: 'full' as const, showDescriptions: true, showIcons: true, showUrls: true, showWebStatus: true, iconSize: 'large' as const, fontSize: 'large' as const }
    for (const cardStyle of ['card', 'tile', 'list'] as const) {
      for (const height of [1, 2, 3]) {
        const current = await settings()
        await mutate('PATCH', '/settings', { homeLayout: makeLayout(height), compact: false, customCss: '', appearance: { ...appearance, cardStyle }, widgets: { ...original.widgets, showHost: false, showDocker: false } }, current.revision)
        await page.goto('/'); await page.reload()
        await expect(page.locator('[data-layout-canvas]')).toHaveAttribute('data-flow', 'false')
        const card = page.locator(`[data-layout-item="link:${fixtures[0].id}"] .service-card`)
        await checkTitle(card, `${cardStyle}, height ${height}`)
        expect((await card.boundingBox())!.height).toBeCloseTo(height * 60 - 12, 0)
        if (height === 1) await page.screenshot({ path: `test-results/link-small-${cardStyle}.png`, fullPage: true })
      }
    }
    await mutate('PATCH', '/settings', { homeLayout: makeLayout(2), appearance: { ...appearance, cardStyle: 'tile' } }, (await settings()).revision)
    await page.goto('/settings/layout')
    await expect(page.locator('[data-layout-canvas]')).toHaveAttribute('data-flow', 'false')
    await checkTitle(page.locator(`[data-layout-item="link:${fixtures[0].id}"] .service-card`))
    const main = page.getByRole('region', { name: '작은 링크 그룹', exact: true })
    const secondary = page.getByRole('region', { name: '보조 그룹', exact: true })
    const dialog = page.getByRole('dialog')
    // Set coordinates directly, leaving deliberate empty space around the section.
    await main.getByRole('button', { name: '작은 링크 섹션 설정', exact: true }).click()
    await dialog.getByLabel('섹션 너비 (칸)').fill('5')
    await dialog.getByLabel('섹션 가로 위치 (칸)').fill('6')
    await dialog.getByLabel('섹션 세로 위치 (칸)').fill('9')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    await expect(main).toHaveAttribute('data-x', '5')
    await expect(main).toHaveAttribute('data-y', '8')
    const handle = main.getByRole('button', { name: '작은 링크 섹션 이동', exact: true })
    await handle.scrollIntoViewIfNeeded()
    let box = (await handle.boundingBox())!
    const canvasBox = (await page.locator('[data-layout-canvas]').boundingBox())!
    const step = (canvasBox.width + 12) / 12
    await page.mouse.move(box.x + 22, box.y + 22); await page.mouse.down()
    await page.mouse.move(box.x + 22 - step * 2, box.y + 142, { steps: 12 }); await page.mouse.up()
    await expect(main).toHaveAttribute('data-x', '3')
    await expect(main).toHaveAttribute('data-y', '10')
    await expect(secondary).toHaveAttribute('data-x', '6')
    await expect(secondary).toHaveAttribute('data-y', '4')
    // Escape, undo and redo also restore section coordinates.
    box = (await handle.boundingBox())!
    await page.mouse.move(box.x + 22, box.y + 22); await page.mouse.down()
    await page.mouse.move(box.x + 22 + step, box.y + 22); await page.keyboard.press('Escape'); await page.mouse.up()
    await expect(main).toHaveAttribute('data-x', '3')
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    await expect(main).toHaveAttribute('data-y', '8')
    await page.getByRole('button', { name: '배치 다시 실행', exact: true }).click()
    await expect(main).toHaveAttribute('data-y', '10')
    const touch = await page.context().newCDPSession(page)
    await touch.send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 1 })
    box = (await handle.boundingBox())!
    await touch.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: box.x + 22, y: box.y + 22, id: 0 }] })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: box.x + 22 + step, y: box.y + 82, id: 0 }] })
    await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    await expect(main).toHaveAttribute('data-x', '4')
    await expect(main).toHaveAttribute('data-y', '11')
    await touch.send('Emulation.setTouchEmulationEnabled', { enabled: false })
    await touch.detach()
    await page.getByRole('button', { name: '배치 되돌리기', exact: true }).click()
    await expect(main).toHaveAttribute('data-x', '3')
    await expect(main).toHaveAttribute('data-y', '10')
    await handle.press('ArrowRight')
    await expect(main).toHaveAttribute('data-x', '4')
    await handle.press('Shift+ArrowRight')
    await expect(main).toHaveAttribute('data-width', '6')
    // Growing a section into a neighbour moves the neighbour rather than overlapping it.
    await main.getByRole('button', { name: '작은 링크 섹션 설정', exact: true }).click()
    await dialog.getByLabel('섹션 가로 위치 (칸)').fill('7')
    await dialog.getByLabel('섹션 세로 위치 (칸)').fill('5')
    await dialog.getByRole('button', { name: '적용', exact: true }).click()
    await expect(main).toHaveAttribute('data-y', '4')
    await expect(secondary).toHaveAttribute('data-y', '7')
    const mainBox = (await main.boundingBox())!, secondBox = (await secondary.boundingBox())!
    expect(secondBox.y).toBeGreaterThanOrEqual(mainBox.y + mainBox.height)
    await page.getByRole('button', { name: '배치 저장', exact: true }).click()
    await expect(page.getByText('배치를 저장했습니다.')).toBeVisible()
    const persisted = await settings()
    const saved = persisted.homeLayout.sections.find(section => section.name === '작은 링크')!
    expect([saved.x, saved.y, saved.width]).toEqual([6, 4, 6])
    await page.reload()
    await expect(main).toHaveAttribute('data-x', '6')
    await expect(main).toHaveAttribute('data-y', '4')
    await page.goto('/')
    await checkTitle(page.locator(`[data-layout-item="link:${fixtures[0].id}"] .service-card`))
    const homeCanvas = (await page.locator('[data-layout-canvas]').boundingBox())!
    const placed = (await main.boundingBox())!
    expect(placed.x - homeCanvas.x).toBeCloseTo(6 * (homeCanvas.width + 12) / 12, 0)
    expect(placed.y - homeCanvas.y).toBeCloseTo(4 * 60, 0)
    await page.screenshot({ path: 'test-results/free-sections-desktop.png', fullPage: true })
    for (const width of [320, 375, 768, 1440]) {
      await page.setViewportSize({ width, height: 1200 })
      await expect(page.locator('[data-layout-canvas]')).toHaveAttribute('data-flow', width < 1000 ? 'true' : 'false')
      await expect(main).toHaveAttribute('data-x', '6')
      await expect(main).toHaveAttribute('data-y', '4')
      await checkTitle(page.locator(`[data-layout-item="link:${fixtures[0].id}"] .service-card`))
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    }
    expect((await settings()).homeLayout).toEqual(persisted.homeLayout)
    // A layout without section coordinates still opens and is upgraded on save.
    const legacy = makeLayout(3)
    for (const section of legacy.sections) { delete section.x; delete section.y }
    await mutate('PATCH', '/settings', { homeLayout: legacy }, (await settings()).revision)
    await page.goto('/settings/layout')
    await expect(main).toHaveAttribute('data-x', '0')
    await expect(secondary).toHaveAttribute('data-x', '6')
    await main.getByRole('button', { name: '작은 링크 섹션 이동', exact: true }).press('ArrowDown')
    await page.getByRole('button', { name: '배치 저장', exact: true }).click()
    await expect(page.getByText('배치를 저장했습니다.')).toBeVisible()
    expect((await settings()).homeLayout.sections.every(section => section.x != null && section.y != null)).toBe(true)
    expect(errors).toEqual([])
  } finally {
    for (const fixture of fixtures) await mutate('DELETE', `/services/${fixture.id}`, undefined, fixture.revision)
    const { revision: _revision, ...body } = original
    await mutate('PATCH', '/settings', body, (await settings()).revision)
  }
})
