import { chromium } from '../web/node_modules/playwright/index.mjs'

const browser = await chromium.launch()
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (message.type() === 'error' && !message.text().includes('401')) errors.push(message.text()) })
  const response = await page.goto('http://127.0.0.1:8080')
  await page.getByRole('heading', { name: /관리자 설정|로그인/ }).waitFor()
  await page.evaluate(() => document.fonts.ready)
  const policy = response.headers()['content-security-policy']
  if (!policy || !policy.includes("script-src 'self'")) throw new Error('Missing production CSP')
  if (errors.length) throw new Error(errors.join('\n'))
  console.log('Go-served production SPA renders; scripts and self-hosted fonts pass CSP.')
} finally { await browser.close() }
