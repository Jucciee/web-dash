import { readFile, writeFile } from 'node:fs/promises'

const target = new URL('../docs/openapi.json', import.meta.url)
const spec = JSON.parse(await readFile(target, 'utf8'))
const schemas = spec.components.schemas
const ref = name => ({ $ref: `#/components/schemas/${name}` })
const string = (extra = {}) => ({ type: 'string', ...extra })
const uuid = () => string({ format: 'uuid' })
const integer = () => ({ type: 'integer', minimum: 1 })
const array = items => ({ type: 'array', items })
const object = (properties, required = Object.keys(properties)) => ({ type: 'object', properties, required, additionalProperties: false })
const linkFields = { iconMode: string({ enum: ['builtin', 'site', 'url', 'upload'] }), iconUrl: string({ maxLength: 2048 }), iconAssetId: string({ maxLength: 64 }), webCheckEnabled: { type: 'boolean', default: true } }
Object.assign(schemas.ServiceInput.properties, linkFields)
Object.assign(schemas.ServicePatch.properties, linkFields)
Object.assign(schemas.Service.properties, linkFields)
schemas.Service.required = [...new Set([...schemas.Service.required, ...Object.keys(linkFields)])]
schemas.WebStatus = object({ serviceId: uuid(), state: string({ enum: ['unknown', 'checking', 'disabled', 'online', 'auth', 'error', 'offline', 'timeout'] }), checking: { type: 'boolean' }, httpStatus: { type: 'integer', minimum: 0 }, latencyMs: { type: 'integer', minimum: 0 }, checkedAt: { type: ['string', 'null'], format: 'date-time' }, message: string() })
schemas.WebStatusesEnvelope = object({ data: array(ref('WebStatus')), meta: ref('Meta') })
schemas.IconUpload = object({ id: string({ pattern: '^[a-f0-9]{64}$' }) })
schemas.IconUploadEnvelope = object({ data: ref('IconUpload'), meta: ref('Meta') })
const choice = values => string({ enum: values })
const boolean = { type: 'boolean' }
schemas.Appearance = object({
  layout: choice(['grid', 'rows']), columns: { type: 'integer', minimum: 0, maximum: 6 }, width: choice(['narrow', 'normal', 'wide', 'full']),
  cardStyle: choice(['card', 'tile', 'list']), iconSize: choice(['small', 'medium', 'large']), radius: choice(['square', 'soft', 'round']),
  fontSize: choice(['small', 'normal', 'large']), fontFamily: choice(['noto', 'system', 'mono']), background: choice(['plain', 'grid', 'dots', 'image']),
  backgroundImage: string({ pattern: '^([a-f0-9]{64})?$' }), backgroundDim: { type: 'integer', minimum: 0, maximum: 90 }, accentColor: string({ pattern: '^(#[a-fA-F0-9]{6})?$' }),
  showTitle: boolean, showClock: boolean, showSearch: boolean, showIcons: boolean, showDescriptions: boolean, showUrls: boolean, showWebStatus: boolean, showGroupCounts: boolean, showFooter: boolean, collapsibleGroups: boolean,
})
schemas.Widgets = object({ showHost: boolean, showDocker: boolean, refreshSeconds: { type: 'integer', enum: [0, 10, 30, 60] }, order: { ...array(choice(['host', 'docker'])), minItems: 2, maxItems: 2, uniqueItems: true } })
schemas.HomeItem = object({ kind: choice(['link', 'addon', 'host', 'docker']), refId: string({ description: 'Link/addon UUID; empty for host/docker. Each reference occurs at most once.' }), x: { type: 'integer', minimum: 0, maximum: 11 }, y: { type: 'integer', minimum: 0, maximum: 4095 }, width: { type: 'integer', minimum: 1, maximum: 12 }, height: { type: 'integer', minimum: 1, maximum: 24 } })
schemas.HomeSection = object({ id: uuid(), name: string({ minLength: 1, maxLength: 80 }), x: { type: 'integer', minimum: 0, maximum: 11, description: 'Section column in the home canvas. Supply x and y together; omitted coordinates preserve older layouts.' }, y: { type: 'integer', minimum: 0, maximum: 65535 }, width: { type: 'integer', minimum: 3, maximum: 12 }, minRows: { type: 'integer', minimum: 0, maximum: 48 }, collapsed: boolean, items: { ...array(ref('HomeItem')), maxItems: 500 } }, ['id', 'name', 'width', 'minRows', 'collapsed', 'items'])
schemas.HomeLayout = object({ version: { type: 'integer', const: 1 }, mode: choice(['auto', 'custom']), sections: { ...array(ref('HomeSection')), maxItems: 60, description: 'Auto mode requires an empty array. Custom mode allows at most 500 items total. Sections occupy 12 columns and 65536 rows; section height is its content rows plus one heading row (empty content reserves three rows). Items occupy 12 columns and 4096 rows within each section. Positioned sections and items must not overlap. Missing/deleted references are skipped; new enabled cards appear in a New items section.' } })
schemas.Settings = object({ title: string({ minLength: 1, maxLength: 80 }), subtitle: string({ maxLength: 200 }), theme: choice(['light', 'dark', 'system']), accent: choice(['blue', 'green', 'amber', 'slate', 'violet', 'rose', 'custom']), compact: boolean, customCss: string({ description: 'UTF-8 maximum 32KiB. Applied only on home.' }), appearance: ref('Appearance'), widgets: ref('Widgets'), homeLayout: ref('HomeLayout'), revision: integer() })
const { revision: _revision, ...settingsFields } = schemas.Settings.properties
schemas.SettingsPatch = object(settingsFields, [])
schemas.ImportFile = object({ path: string({ maxLength: 240, description: 'Unique relative virtual path, never a server filesystem path' }), content: string({ description: 'UTF-8 maximum 2MiB; all files combined maximum 10MiB' }) })
schemas.ImportInput = object({ dashboardId: uuid(), kind: string({ enum: ['nginx', 'caddy'] }), sourceName: string({ maxLength: 80 }), entryPath: string({ maxLength: 240 }), files: { ...array(ref('ImportFile')), minItems: 1, maxItems: 50 } }, ['dashboardId', 'kind', 'entryPath', 'files'])
schemas.DockerScanInput = object({ dashboardId: uuid(), folder: string({ maxLength: 2048, description: 'Absolute folder on Dashboard server. Scan skips symbolic links, hidden directories and node_modules, up to four subdirectory levels.' }), publicHost: string({ maxLength: 253, description: 'Host or IP used for wildcard-bound runtime published ports, without a port or scheme' }) })
schemas.ImportCandidate = object({ id: uuid(), name: string(), url: string({ description: 'Concrete HTTP/HTTPS URL. Unresolved entries are excluded and reported in preview warnings.' }), file: string(), line: { type: 'integer', minimum: 0 }, upstreams: array(string()), warnings: array(string()), diff: string({ enum: ['new', 'duplicate'] }), existingServiceId: string() })
schemas.ImportPreview = object({ id: uuid(), dashboardId: uuid(), kind: string({ enum: ['nginx', 'caddy', 'docker'] }), sourceName: string(), dashboardRevision: integer(), expiresAt: string({ format: 'date-time' }), candidates: array(ref('ImportCandidate')), warnings: array(string()) })
schemas.ImportSelection = object({ candidateId: uuid(), ...schemas.ServiceInput.properties }, ['candidateId', ...schemas.ServiceInput.required])
schemas.ImportCommit = object({ dashboardRevision: integer(), items: { ...array(ref('ImportSelection')), minItems: 1, maxItems: 500 } })
schemas.ImportResult = object({ serviceIds: array(uuid()), dashboardRevision: integer() })
const nonnegative = { type: 'integer', minimum: 0 }
const stringMap = { type: 'object', additionalProperties: string() }
schemas.AddonHeader = object({ name: string({ maxLength: 80 }), value: string({ maxLength: 8192 }) })
schemas.AddonInput = object({ key: string({ pattern: '^[A-Za-z][A-Za-z0-9_]{0,39}$' }), label: string({ maxLength: 80 }), type: choice(['text', 'secret']), required: boolean, default: string({ description: 'Only text inputs may define a default. Secret values are never part of a manifest.' }) })
schemas.AddonField = object({ label: string({ maxLength: 80 }), path: string({ description: 'JSON Pointer, e.g. /data/count or /items/0/name; empty means root' }), format: choice(['text', 'number', 'percent', 'bytes', 'duration', 'boolean', 'count']), unit: string({ maxLength: 24 }), scale: { type: 'number' }, decimals: { type: 'integer', minimum: 0, maximum: 4 }, valueMap: stringMap })
schemas.AddonRequest = object({ method: choice(['GET', 'POST']), url: string({ maxLength: 2048 }), headers: array(ref('AddonHeader')), body: string({ description: 'JSON POST body, maximum 16KiB before interpolation. Templates: {{key}}, {{key|url}}, {{key|json}}, {{key|base64}}.' }) })
schemas.AddonManifest = object({ schemaVersion: { type: 'integer', const: 1 }, name: string({ minLength: 1, maxLength: 80 }), icon: choice(['server', 'photos', 'media', 'files', 'code', 'home', 'network', 'database']), display: choice(['stats', 'list']), refreshSeconds: { type: 'integer', enum: [0, 10, 30, 60, 300] }, request: ref('AddonRequest'), inputs: array(ref('AddonInput')), fields: { ...array(ref('AddonField')), minItems: 1, maxItems: 16 } })
schemas.Addon = object({ id: uuid(), manifest: ref('AddonManifest'), values: stringMap, configuredSecrets: array(string()), enabled: boolean, position: nonnegative, revision: integer() })
schemas.AddonWrite = object({ manifest: ref('AddonManifest'), values: stringMap, secrets: { ...stringMap, description: 'Write-only secret input values. Omit a key to preserve it when editing the same API origin; empty string clears it.' }, enabled: boolean })
schemas.AddonPreview = object({ ...schemas.AddonWrite.properties, id: string(), revision: nonnegative })
schemas.AddonValue = object({ label: string(), value: string(), unit: string(), missing: boolean })
schemas.AddonChoice = object({ path: string(), value: string(), type: string() })
schemas.AddonSnapshot = object({ available: boolean, httpStatus: nonnegative, latencyMs: nonnegative, measuredAt: string({ format: 'date-time' }), message: string(), values: array(ref('AddonValue')), choices: array(ref('AddonChoice')) })
schemas.AddonListEnvelope = object({ data: array(ref('Addon')), meta: ref('Meta') })
for (const name of ['Addon', 'AddonSnapshot']) schemas[`${name}Envelope`] = object({ data: ref(name), meta: ref('Meta') })
const nullableNumber = { type: ['number', 'null'], minimum: 0 }
const nullable = name => ({ anyOf: [ref(name), { type: 'null' }] })
schemas.ResourceUsage = object({ used: nonnegative, total: nonnegative, percent: { type: 'number', minimum: 0 } })
schemas.DiskUsage = object({ path: string(), ...schemas.ResourceUsage.properties })
schemas.HostSnapshot = object({ available: boolean, name: string(), os: string(), scope: choice(['system', 'runtime', 'host']), cpuPercent: nullableNumber, cpuCores: nonnegative, memory: nullable('ResourceUsage'), disk: nullable('DiskUsage'), uptimeSeconds: { type: ['integer', 'null'], minimum: 0 }, measuredAt: string({ format: 'date-time' }), warnings: array(string()) })
schemas.ContainerSnapshot = object({ id: string(), name: string(), image: string(), project: string(), state: string(), status: string(), health: string(), cpuPercent: nullableNumber, memory: nullable('ResourceUsage'), networkRx: { type: ['integer', 'null'], minimum: 0 }, networkTx: { type: ['integer', 'null'], minimum: 0 }, statsError: string() })
schemas.DockerSnapshot = object({ available: boolean, name: string(), os: string(), version: string(), cpuCores: nonnegative, memoryTotal: nonnegative, running: nonnegative, stopped: nonnegative, containers: array(ref('ContainerSnapshot')), measuredAt: string({ format: 'date-time' }), message: string(), warnings: array(string()) })
for (const name of ['Settings', 'ImportPreview', 'ImportResult', 'HostSnapshot', 'DockerSnapshot']) schemas[`${name}Envelope`] = object({ data: ref(name), meta: ref('Meta') })
const csrf = structuredClone(spec.paths['/services'].post.parameters[0])
const match = structuredClone(spec.paths['/services/{id}'].patch.parameters[2])
const id = { name: 'id', in: 'path', required: true, schema: uuid() }
const error = { description: 'API error: 400, 401, 403, 404, 409, 413, 422, 500 or 502', content: { 'application/json': { schema: ref('ErrorEnvelope') } } }
const operation = (summary, result, status = 200, input, parameters = []) => ({ summary, parameters, ...(input ? { requestBody: { required: true, content: { 'application/json': { schema: ref(input) } } } } : {}), responses: { [status]: { description: 'Success', content: { 'application/json': { schema: ref(`${result}Envelope`) } } }, default: error } })
spec.paths['/settings'] = { get: operation('Read persisted appearance settings', 'Settings'), patch: operation('Update appearance with optimistic revision check', 'Settings', 200, 'SettingsPatch', [csrf, match]) }
spec.paths['/addons'] = { get: operation('List installed API addons without credentials', 'AddonList'), post: operation('Install a v1 declarative addon; encrypt secret inputs', 'Addon', 201, 'AddonWrite', [csrf]) }
spec.paths['/addons/preview'] = { post: operation('Test a draft request and preview bounded, redacted response choices without saving', 'AddonSnapshot', 200, 'AddonPreview', [csrf]) }
spec.paths['/addons/{id}'] = { get: operation('Read public addon configuration', 'Addon', 200, undefined, [id]), patch: operation('Replace addon configuration with revision check; never return secret values', 'Addon', 200, 'AddonWrite', [id, csrf, match]), delete: { summary: 'Uninstall an addon and its saved credentials', parameters: [id, csrf, match], responses: { 204: { description: 'Deleted' }, default: error } } }
spec.paths['/addons/{id}/read'] = { post: operation('Read an enabled addon; 5s shared cache, no background collection', 'AddonSnapshot', 200, undefined, [id, csrf]) }
schemas.AddonMove = object({ direction: { type: 'integer', enum: [-1, 1] } })
spec.paths['/addons/{id}/move'] = { post: operation('Move a widget in installed-addon order', 'AddonList', 200, 'AddonMove', [id, csrf, match]) }
spec.paths['/settings/custom.css'] = { get: { summary: 'Read authenticated custom CSS for home', responses: { 200: { description: 'CSS stylesheet, no-store', content: { 'text/css': { schema: string() } } }, default: error } } }
spec.paths['/monitoring/host'] = { get: operation('Read CPU, memory, disk and uptime of the explicitly labelled measurement environment; 5s shared cache', 'HostSnapshot') }
spec.paths['/monitoring/docker'] = { get: operation('Read Engine metadata, all container states, health and bounded resource samples; 5s shared cache', 'DockerSnapshot') }
spec.paths['/backgrounds'] = { post: { summary: 'Upload a PNG/JPEG/WebP background, up to 8MiB, 6000px and 24 megapixels', parameters: [csrf], requestBody: { required: true, content: { 'multipart/form-data': { schema: object({ file: string({ format: 'binary' }) }) } } }, responses: { 201: { description: 'Stored image ID', content: { 'application/json': { schema: ref('IconUploadEnvelope') } } }, default: error } } }
spec.paths['/backgrounds/{id}'] = { get: { summary: 'Read an authenticated background image', parameters: [{ name: 'id', in: 'path', required: true, schema: string({ pattern: '^[a-f0-9]{64}$' }) }], responses: { 200: { description: 'Raster background image', content: Object.fromEntries(['image/png', 'image/jpeg', 'image/webp'].map(type => [type, { schema: string({ format: 'binary' }) }])) }, default: error } } }
spec.paths['/imports/preview'] = { post: operation('Preview uploaded Nginx/Caddy files (JSON virtual file bundle)', 'ImportPreview', 201, 'ImportInput', [csrf]) }
spec.paths['/imports/docker/scan'] = { post: operation('Find Compose files and runtime published TCP ports from running containers', 'ImportPreview', 201, 'DockerScanInput', [csrf]) }
spec.paths['/imports/{id}'] = { get: operation('Read owned, unexpired preview', 'ImportPreview', 200, undefined, [id]) }
spec.paths['/imports/{id}/commit'] = { post: operation('Atomically create selected cards; duplicate-safe idempotent replay', 'ImportResult', 200, 'ImportCommit', [id, csrf, { name: 'Idempotency-Key', in: 'header', required: true, schema: uuid() }]) }
spec.paths['/dashboards/{id}/web-statuses'] = { get: { summary: 'Read cached server-side HTTP observations without initiating outbound checks', parameters: [id], responses: { 200: { description: 'Success', content: { 'application/json': { schema: ref('WebStatusesEnvelope') } } }, default: error } } }
spec.paths['/dashboards/{id}/web-statuses/check'] = { post: { summary: 'Queue HTTP checks on page visit or manual request; coalesced with a 5s minimum interval', parameters: [id, csrf], responses: { 200: { description: 'Current observations with queued checks', content: { 'application/json': { schema: ref('WebStatusesEnvelope') } } }, default: error } } }
spec.paths['/icons'] = { post: { summary: 'Upload a validated PNG/JPEG/WebP/ICO icon, up to 1MiB and 2048px', parameters: [csrf], requestBody: { required: true, content: { 'multipart/form-data': { schema: object({ file: string({ format: 'binary' }) }) } } }, responses: { 201: { description: 'Stored image ID', content: { 'application/json': { schema: ref('IconUploadEnvelope') } } }, default: error } } }
for (const path of ['/icons/{id}', '/services/{id}/icon']) spec.paths[path] = { get: { summary: 'Read an authenticated, validated icon image; remote discovery is cached and bounded', parameters: [{ ...id, schema: string() }], responses: { 200: { description: 'Raster icon image', content: { 'image/png': { schema: string({ format: 'binary' }) }, 'image/jpeg': { schema: string({ format: 'binary' }) }, 'image/webp': { schema: string({ format: 'binary' }) }, 'image/x-icon': { schema: string({ format: 'binary' }) } } }, 204: { description: 'No site icon found; use built-in fallback' }, default: error } } }
spec.info.title = 'web-dash API'
spec.info.version = '0.4.0'
spec.info.description = 'web-dash: authentication, link/group management, visual customization and widgets, declarative JSON API addons with encrypted secret inputs, uploaded backgrounds and icons, home CSS, HTTP checks, Nginx/Caddy import, Docker discovery and read-only Docker/host metrics.'
await writeFile(target, JSON.stringify(spec, null, 2) + '\n')
