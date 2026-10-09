import { spawn, spawnSync } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import http from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dataDir = path.join(root, '.tmp', `e2e-${process.pid}-${Date.now()}`)
await mkdir(dataDir, { recursive: true })
const composeFolder = path.join(root, '.tmp', 'e2e-compose')
await mkdir(composeFolder, { recursive: true })
const composeFile = path.join(composeFolder, 'compose.yml')
await writeFile(composeFile, 'services:\n  browser-docker:\n    image: example/test-only\n    ports: ["9999:3000"]\n')
const fakeDocker = http.createServer((request, response) => {
  if (request.url === '/_ping') { response.setHeader('API-Version', '1.56'); response.end('OK'); return }
  if (request.method !== 'GET') { response.writeHead(403); response.end(); return }
  response.setHeader('Content-Type', 'application/json')
  if (/^\/v[0-9.]+\/info/.test(request.url ?? '')) { response.end(JSON.stringify({ Name: 'e2e-engine', OperatingSystem: 'Docker Desktop (test fixture)', OSType: 'linux', ServerVersion: '29.0.0', NCPU: 4, MemTotal: 8 * 1024 ** 3 })); return }
  if (/^\/v[0-9.]+\/containers\/[a-f0-9]+\/stats/.test(request.url ?? '')) { response.end(JSON.stringify({ read: new Date().toISOString(), preread: new Date(Date.now() - 1000).toISOString(), cpu_stats: { cpu_usage: { total_usage: 1250 }, system_cpu_usage: 2000, online_cpus: 4 }, precpu_stats: { cpu_usage: { total_usage: 1000 }, system_cpu_usage: 1000 }, memory_stats: { usage: 300 * 1024 ** 2, limit: 1024 ** 3, stats: { inactive_file: 100 * 1024 ** 2 } }, networks: { eth0: { rx_bytes: 1024 ** 2, tx_bytes: 2 * 1024 ** 2 } } })); return }
  if (!/^\/v[0-9.]+\/containers\/json/.test(request.url ?? '')) { response.writeHead(403); response.end(); return }
  response.end(JSON.stringify([
    { Id: 'a'.repeat(64), Names: ['/browser-docker'], Image: 'example/test-only', State: 'running', Status: 'Up 1 hour (healthy)', Health: { Status: 'healthy' }, Labels: { 'com.docker.compose.project': 'e2e-compose', 'com.docker.compose.service': 'browser-docker', 'com.docker.compose.project.config_files': composeFile }, Ports: [{ IP: '0.0.0.0', PublicPort: 31080, PrivatePort: 3000, Type: 'tcp' }] },
    { Id: 'b'.repeat(64), Names: ['/unhealthy-worker'], Image: 'example/worker', State: 'running', Status: 'Up 1 hour (unhealthy)', Health: { Status: 'unhealthy' }, Labels: { 'com.docker.compose.project': 'workers' }, Ports: [] },
    { Id: 'c'.repeat(64), Names: ['/stopped-worker'], Image: 'example/worker', State: 'exited', Status: 'Exited (0)', Labels: {}, Ports: [] },
  ]))
})
await new Promise((resolve, reject) => { fakeDocker.once('error', reject); fakeDocker.listen(18081, '127.0.0.1', resolve) })
const iconPNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64')
let webChecks = 0
let addonReads = 0
const fakeWeb = http.createServer((request, response) => {
  if (request.url === '/__addon_checks') { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ checks: addonReads })); return }
  if (request.url === '/addon/status') {
    addonReads++
    response.setHeader('Content-Type', 'application/json')
    if (request.headers.authorization !== 'Bearer e2e-addon-token' || request.headers.cookie !== 'session=e2e-addon-cookie') { response.writeHead(401); response.end(JSON.stringify({ error: 'fixture authentication missing' })); return }
    response.end(JSON.stringify({ metrics: { active: 0, load: 0.25 }, items: ['first', 'second'], status: 'healthy', echo: 'e2e-addon-token', cookie: 'session=e2e-addon-cookie' })); return
  }
  if (request.url === '/__checks') { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ checks: webChecks })); return }
  if (request.method === 'HEAD') webChecks++
  if (request.url === '/icon.png' || request.url === '/favicon.ico') { response.setHeader('Content-Type', 'image/png'); response.end(iconPNG); return }
  if (request.url === '/auth') { response.writeHead(401); response.end(); return }
  if (request.url === '/error') { response.writeHead(503); response.end(); return }
  response.setHeader('Content-Type', 'text/html')
  response.end('<html><head><link rel="icon" href="/icon.png"></head><body>HTTP fixture</body></html>')
})
await new Promise((resolve, reject) => { fakeWeb.once('error', reject); fakeWeb.listen(18082, '127.0.0.1', resolve) })
const env = { ...process.env, GOCACHE: path.join(root, '.cache', 'go-build'), GOMODCACHE: path.join(root, '.cache', 'go-mod'), GOPATH: path.join(root, '.cache', 'go'), DASHBOARD_ADDR: '127.0.0.1:18080', DASHBOARD_DATA_DIR: dataDir, DASHBOARD_DOCKER_HOST: 'tcp://127.0.0.1:18081', DASHBOARD_SETUP_TOKEN: 'playwright-test-only-setup-token-32-characters', DASHBOARD_COOKIE_SECURE: 'false', DASHBOARD_STATIC_DIR: path.join(root, 'web', 'dist') }
const binary = path.join(dataDir, process.platform === 'win32' ? 'dashboard.exe' : 'dashboard')
const build = spawnSync('go', ['build', '-o', binary, './cmd/dashboard'], { cwd: root, env, stdio: 'inherit', windowsHide: true })
if (build.status !== 0 || build.error) throw build.error ?? new Error(`Go build failed: ${build.status}`)
const server = spawn(binary, [], {
  cwd: root,
  stdio: 'inherit',
  windowsHide: true,
  env,
})
server.on('exit', code => { fakeDocker.close(); fakeWeb.close(); process.exitCode = code ?? 1 })
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { server.kill(signal) })
